package vechain

import (
	"context"
	"fmt"
	"math"
	"runtime/debug"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"github.com/myafeier/log"
	"xorm.io/xorm"
)

var Daemon *Service

const BatchAmount = 2

func init() {
	log.SetPrefix("Vechain")
	log.SetLogLevel(log.DEBUG)
}

// NewService 新建服务
func InitService(ctx context.Context, engine *xorm.Engine, config *VechainConfig) {
	if Daemon == nil {
		Daemon = &Service{dbEngine: engine}
		Daemon.Token = NewDefaultToken(config)
		Daemon.SuccessChan = make(chan *Block, 10000)
		Daemon.dbEngine = engine
		Daemon.config = config
		Daemon.missionChan = make(chan []*Block, 100)
		Daemon.key = time.Now().Unix()
	}
	initTable(engine.NewSession())
	go Daemon.StartDaemon(ctx)
}

// 提交产品ID和产品HASH，提交上链
var submitMutex sync.Mutex

// 异步提交hash
func AsyncSubmit(hashes []string) (err error) {
	submitMutex.Lock()
	defer submitMutex.Unlock()

	if len(hashes) == 0 {
		err = errors.WithStack(fmt.Errorf("length of hashes==0"))
		return
	}
	blocks, err := Daemon.AddBlock(hashes)
	if err != nil {
		return
	}

	loop := int(math.Ceil(float64(len(blocks)) / BatchAmount))
	log.Debug("%d", loop)

	for i := 0; i < loop; i++ {
		end := (i + 1) * BatchAmount
		if end > len(blocks) {
			end = len(blocks)
		}
		pending := blocks[i*BatchAmount : end]
		Daemon.missionChan <- pending
	}
	return

}

func GetExplorUrlByTxid(txid string) string {
	return BlockChainExploreLink(txid, Daemon.config)
}

// 获取产品的区块信息
func GetBlockInfoByUuid(uuid string) (b *Block, err error) {
	session := Daemon.dbEngine.NewSession()
	b = new(Block)
	has, err := session.Where("hash=?", uuid).Get(b)
	if err != nil {
		log.Error("%+v", err.Error())
		return
	}
	log.Debug("blockInfo: %+v", *b)
	if !has {
		err = fmt.Errorf("%s not found", uuid)
	}
	b.GetExplorUrl()
	return
}

// 添加观察者
func AddPostArtifactObserver(observer IObserver) {
	Daemon.Observers = append(Daemon.Observers, observer)
}

// Service 服务
type Service struct {
	SuccessChan chan *Block   //执行成功后的处理通道
	missionChan chan []*Block //正常全流程
	Observers   []IObserver   //观察者
	Token       IToken
	dbEngine    *xorm.Engine
	config      *VechainConfig
	key         int64
}
func (s *Service) getKey() int64{
	return atomic.AddInt64(&s.key,1)
}

func (s *Service) StartDaemon(ctx context.Context) {
	ticket := time.NewTicker(CheckFailDuration)
	running := false
	for {
		select {
		case block := <-s.SuccessChan:
			log.Debug("receive success block")
			go func(b *Block) {
				log.Debug("run watcher:%s", b.Hash)
				defer func() {
					if r := recover(); r != nil {
						log.Debug("Recover: %+v", r)
						debug.PrintStack()
					}
				}()
				for _, v := range s.Observers {
					v.Execute(b.Hash, b.Vid, b.TxId)
				}
			}(block)
		case <-ticket.C:
			go func() {
				defer func() {
					s.checkFail()
				}()
			}()
		case block := <-s.missionChan:
			go func() {
				running = true
				if err := s.Generate(block); err != nil {
					log.Error("%+v", err)
				} else {
					if err = s.Submit(block); err != nil {
						log.Error("%+v", err)
					}
				}
				running = false
			}()
		case <-ctx.Done():
			for {
				if running {
					time.Sleep(1 * time.Second)
				} else {
					return
				}
			}
		}
	}
}

func (s *Service) checkFail() {
	log.Info("check fail")

}

func (s *Service) AddBlock(hash []string) (result []*Block, err error) {
	sess := s.dbEngine.NewSession()
	if err = sess.Begin(); err != nil {
		err = errors.WithStack(err)
		return
	}
	defer func(err error) {
		if err != nil {
			sess.Rollback()
		} else {
			sess.Commit()
		}
		sess.Close()
	}(err)

	for _, v := range hash {
		b := new(Block)
		b.Hash = v
		b.State = BlockStateToPost
		_, err = sess.Insert(b)
		if err != nil {
			err = errors.WithStack(err)
			return
		}
		result = append(result, b)
	}
	return
}

// 批量生成
func (s *Service) Generate(block []*Block) (err error) {
	// 统计hash数量
	amount := len(block)
	req := new(GenerateRequest)
	req.Quantity = amount
	req.RequestNo = strconv.FormatInt(s.getKey(),10)

	ctx := context.WithValue(context.Background(), "request", req)
	resp, err := Generate(ctx, s.config, s.Token)
	if err != nil {
		if err1 := s.storeFailMission(req.RequestNo, MissionTypeOfGenerate, MissionStatusOfFail, err.Error(), block); err1 != nil {
			err = errors.WithMessage(err, err1.Error())
		}
		return
	}

	for k, v := range block {
		v.Vid = resp.VidList[k]
	}
	err = s.update(block)
	if err != nil {
		return
	}
	// 上链
	return
}

func (s *Service) storeFailMission(requestNo string, mtype MissionType, status MissionStatus, msg string, block []*Block) (err error) {
	sess := s.dbEngine.NewSession()
	defer sess.Close()
	var mission Mission
	has, err := sess.Where("request_no=?", requestNo).Get(&mission)
	if err != nil {
		err = errors.WithStack(err)
		return
	}
	mission.Status = status
	mission.Message = msg
	if has {
		_, err = sess.Where("id=?", mission.Id).Cols("status", "message", "updated").Update(&mission)
		if err != nil {
			err = errors.WithStack(err)
			return
		}
	} else {
		mission.MissionType = mtype
		for _, v := range block {
			mission.BlockIds = append(mission.BlockIds, v.Id)
		}
		_, err = sess.Insert(&mission)
		if err != nil {
			err = errors.WithStack(err)
			return
		}
	}
	return
}

// 提交上链请求
func (s *Service) Submit(block []*Block) (err error) {
	// 统计hash数量
	req := new(SubmitRequest)
	req.OperatorUID = s.config.UserIdOfYuanZhiLian
	req.RequestNo = strconv.FormatInt(s.getKey(), 10)
	for _, v := range block {
		req.HashList = append(req.HashList, &Hash{
			Vid:      v.Vid,
			DataHash: v.Hash,
		})
	}
	ctx := context.WithValue(context.Background(), "request", req)
	resp, err := Submit(ctx, s.config, s.Token)
	if err != nil {
		if err1 := s.storeFailMission(req.RequestNo, MissionTypeOfSubmit, MissionStatusOfFail, err.Error(), block); err1 != nil {
			err = errors.WithMessage(err, err1.Error())
		}
		return
	}
	for _, v := range block {
		for _, vv := range resp.TxList {
			if vv.Vid == v.Vid {
				v.ClauseIndex = vv.ClauseIndex
				v.TxId = vv.TxId
				v.SubmitTime = time.Unix(vv.SubmitTime, 0)
				v.State = BlockStatePosted
				break
			}
		}
	}
	err = s.update(block)
	return
}

func (s *Service) update(block []*Block) (err error) {
	sess := s.dbEngine.NewSession()
	if err = sess.Begin(); err != nil {
		err = errors.WithStack(err)
		return
	}
	defer func(err error) {
		if err != nil {
			sess.Rollback()
		} else {
			sess.Commit()
		}
		sess.Close()
	}(err)
	for _, v := range block {
		_, err = sess.Where("id=?", v.Id).Update(v)
		if err != nil {
			err = errors.WithStack(err)
			return
		}
	}
	return
}

func (self *Service) CreateSubAccount(requestNo, account string) (uid string, err error) {

	log.Debug(requestNo)
	return GenerateSubAccount(requestNo, account, self.config, self.Token)
}
