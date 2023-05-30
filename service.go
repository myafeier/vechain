package vechain

import (
	"context"
	"fmt"
	"math"
	"runtime/debug"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/myafeier/log"
	"xorm.io/xorm"
)

var Daemon *Service

func init() {
	log.SetPrefix("Vechain")
	log.SetLogLevel(log.DEBUG)
}

// NewService 新建服务
func InitService(engine *xorm.Engine, config *VechainConfig) {
	if Daemon == nil {
		Daemon = &Service{dbEngine: engine}
		Daemon.Token = NewDefaultToken(config)
		Daemon.SuccessChan = make(chan *Block, 10000)
		Daemon.dbEngine = engine
		Daemon.config = config
	}
	initTable(engine.NewSession())
	go Daemon.StartDaemon()
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
	blocks, err := Daemon.AddHashs(hashes)
	if err != nil {
		return
	}

	loop := int(math.Floor(float64(len(blocks)) / 2000))

	for i := 0; i < loop; i++ {
		pending := blocks[i*2000 : (i+1)*2000]
		err = Daemon.Generate(pending)
		if err != nil {
			return
		}
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
	SuccessChan chan *Block //执行成功后的处理通道
	Observers   []IObserver //观察者
	Token       IToken
	dbEngine    *xorm.Engine
	config      *VechainConfig
}

func (s *Service) StartDaemon() {
	ticket := time.NewTicker(CheckFailDuration)
	for {
		select {
		case block := <-s.SuccessChan:
			log.Debug("receive success block。。。")
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
					if r := recover(); r != nil {
						log.Debug("Recover: %+v", r)
						debug.PrintStack()
					}
				}()
				//s.checkFail()
			}()
		}
	}
}
func (s *Service) AddHashs(hash []string) (result []*Block, err error) {
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
	}
	return
}

// 批量生成
func (s *Service) Generate(block []*Block) (err error) {
	// 统计hash数量
	amount := len(block)
	req := new(GenerateRequest)
	req.Quantity = amount
	req.RequestNo = string(time.Now().Unix())

	ctx := context.WithValue(context.Background(), "request", req)
	resp, err := Generate(ctx, s.config, s.Token)
	if err != nil {
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

}

// 提交上链请求
func (self *Service) Submit(hash []*Block) (err error) {
	// 统计hash数量
	amount := len(hash)
	if amount == 0 {
		errors.WithStack(fmt.Errorf("长度为0"))
	}
	// 生成响应数量的vid

	// 上链
	return
}

func (self *Service) CreateSubAccount(requestNo, account string) (uid string, err error) {

	log.Debug(requestNo)
	return GenerateSubAccount(requestNo, account, self.config, self.Token)
}
