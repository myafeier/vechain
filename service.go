package vechain

import (
	"context"
	"fmt"
	"math"
	"runtime/debug"
	"sync"
	"time"

	"github.com/go-xorm/xorm"
	"github.com/myafeier/log"
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
		Daemon.CommandChan = make(chan ICommand, 100)
		Daemon.SuccessChan = make(chan *Block, 10000)
		Daemon.dbEngine = engine
		Daemon.config = config
	}
	initTable(engine.NewSession())
	go Daemon.StartDaemon()
}

//提交产品ID和产品HASH，提交上链
var submitMutex sync.Mutex

// 异步提交hash
func AsyncSubmit(hashes []string) (err error) {
	submitMutex.Lock()
	defer submitMutex.Unlock()
	//过滤已经有命令的产品，
	restHashes, err := Daemon.filter(hashes)
	if err != nil {
		log.Error("%+v", err.Error())
		return
	}

	if restHashes != nil && len(restHashes) > 0 {
		var blocks []*Block

		for _, v := range restHashes {
			b := new(Block)
			b.Hash = v
			b.State = BlockStateToOccupy
			blocks = append(blocks, b)
		}
		err = Daemon.dispatchVid(blocks)
		if err != nil {
			log.Error("%+v", err.Error())
			return
		}
	}
	return
}
func GetExplorUrlByTxid(txid string) string {
	return BlockChainExploreLink(txid, Daemon.config)
}

//获取产品的区块信息
func GetBlockInfoByUuid(uuid string) (b *Block, err error) {
	session := Daemon.dbEngine.NewSession()
	b = new(Block)
	has, err := session.Where("uuid=?", uuid).Get(b)
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

//添加观察者
func AddPostArtifactObserver(observer IObserver) {
	Daemon.Observers = append(Daemon.Observers, observer)
}

// Service 服务
type Service struct {
	RunningCommandIds sync.Map      //运行中的命令
	CommandChan       chan ICommand //命令执行通道
	SuccessChan       chan *Block   //执行成功后的处理通道
	Observers         []IObserver   //观察者
	Token             IToken
	dbEngine          *xorm.Engine
	config            *VechainConfig
}

func (s *Service) StartDaemon() {
	ticket := time.NewTicker(CheckFailDuration)
	for {
		select {
		case cmd := <-s.CommandChan:
			log.Debug("receive command")
			go func() {
				defer func() {
					if r := recover(); r != nil {
						log.Debug("Recover: %+v", r)
						debug.PrintStack()
					}
				}()

				cmd.Execute(s)
			}()
		case block := <-s.SuccessChan:
			log.Debug("receive success block。。。")
			go func(b *Block) {
				log.Debug("run watcher:%d", b.Hash)
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
				s.checkFail()
			}()
		}
	}
}

func (s *Service) checkFail() {
	var failIds []int64
	session := s.dbEngine.NewSession()
	err := session.Table("vechain_command").Where("state!=?", CommandStateOfSuccess).Cols("id").Find(&failIds)
	if err != nil {
		log.Error("%+v", err.Error())
		return
	}
	if failIds != nil && len(failIds) > 0 {
		for _, v := range failIds {

			if _, ok := s.RunningCommandIds.Load(v); ok { //命令还在运行中
				log.Debug("命令：%d已在运行中，跳过!", v)
				continue
			}
			ctx := context.TODO()
			ctx, _ = context.WithDeadline(ctx, time.Now().Add(CommandExpireDuration))
			var cmd ICommand
			cmd, err = GetCommandById(session, ctx, v)
			if err != nil {
				log.Error("%+v", err.Error())
				return
			}
			s.CommandChan <- cmd
			s.RunningCommandIds.Store(v, true)
		}
	}
}

func (s *Service) dispatchVid(blocks []*Block) (err error) {
	persistMutex.Lock()
	defer persistMutex.Unlock()
	session := s.dbEngine.NewSession()
	err = session.Begin()
	if err != nil {
		log.Error("%+v", err.Error())
		return
	}

	defer func() {
		if err != nil {
			err = session.Rollback()
		} else {
			err = session.Commit()
		}
	}()

	//按照100每组进行分组，形成command
	hashLength := len(blocks)
	var datas [][]*Block
	if hashLength > ItemAmountPerRequest {
		n := int(math.Ceil(float64(hashLength) / float64(ItemAmountPerRequest)))
		for i := 0; i < n; i++ {
			lastIndex := (i + 1) * ItemAmountPerRequest
			if hashLength < lastIndex {
				lastIndex = hashLength
			}
			datas = append(datas, blocks[i*ItemAmountPerRequest:lastIndex])
		}
		//datas = append(datas, blocks[(n*ItemAmountPerRequest-1):hashLength])
	} else {
		datas = append(datas, blocks)
	}

	var cmds []ICommand

	for _, v := range datas {
		ctx := context.TODO()
		ctx, _ = context.WithDeadline(ctx, time.Now().Add(CommandExpireDuration))
		var cmd ICommand
		cmd, err = NewOccupyVidCommand(session, ctx, v)
		if err != nil {
			log.Error("%+v", err.Error())
			return
		}
		cmds = append(cmds, cmd)
	}
	if cmds != nil && len(cmds) > 0 {
		for _, v := range cmds {
			s.CommandChan <- v
			s.RunningCommandIds.Store(v.GetId(), true)
		}
	}

	return
}

//过滤已经存在的block
func (s *Service) filter(hashes []string) (restIds []string, err error) {
	var existBlocks []*Block
	session := s.dbEngine.NewSession()
	err = session.In("hash", hashes).Find(&existBlocks)
	if err != nil {
		log.Error("%+v", err.Error())
		return
	}
	var existCommandIds = make([]int64, 0)

	if existBlocks != nil && len(existBlocks) > 0 {
		for _, v := range existBlocks {
			if v.State == BlockStatePosted {
				s.SuccessChan <- v
			} else {
				exist := false
				for _, vv := range existCommandIds {
					if vv == v.CurrentCommandId {
						exist = true
						break
					}
				}
				if !exist {
					existCommandIds = append(existCommandIds, v.CurrentCommandId)
				}
			}
			for kk, vv := range hashes {
				if vv == v.Hash {
					hashes = append(hashes[:kk], hashes[kk+1:]...)
					break
				}
			}
		}

		log.Debug("length of existCommandIds:%d", len(existCommandIds))
		for _, v := range existCommandIds {
			if _, ok := s.RunningCommandIds.Load(v); ok { //命令还在运行中
				log.Debug("命令：%d已在运行中，跳过!", v)
				continue
			}
			ctx := context.TODO()
			ctx, _ = context.WithDeadline(ctx, time.Now().Add(CommandExpireDuration))
			var cmd ICommand
			cmd, err = GetCommandById(session, ctx, v)
			if err != nil {
				log.Error("%+v", err.Error())
				return
			}
			s.CommandChan <- cmd
			s.RunningCommandIds.Store(v, true)
		}
	}

	for _, v := range hashes {
		restIds = append(restIds, v)
	}
	return
}
