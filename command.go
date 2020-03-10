package vechain

import (
	"context"
	"fmt"
	"github.com/go-xorm/xorm"
	"github.com/myafeier/log"
	"strconv"
	"sync"
)

const (
	Command_Post_Artifact = "post_artifact"
	Command_Occupy_Vid    = "occupy_vid"
)

type CommandState string

const (
	CommandStateOfGenerating CommandState = "GENERATING"
	CommandStateOfSuccess    CommandState = "SUCCESS"
	CommandStateOfFail       CommandState = "FAIL"
)

var persistMutex sync.Mutex

type ICommand interface {
	Execute(service *Service) (err error)
	GetId() int64
	GetState() CommandState
	GetBlocks() []*Block
}

func GetCommandById(session *xorm.Session, ctx context.Context, id int64) (cmd ICommand, err error) {
	cm := &CommandModel{}
	has, err := session.ID(id).Get(cm)
	if err != nil {
		log.Error("%+v", err.Error())
		return
	}
	if !has {
		err = fmt.Errorf("invalid command id:%d", id)
		return
	}
	blocks, err := cm.GetBlock(session)
	if err != nil {
		log.Error("%+v", err.Error())
		return
	}
	switch cm.Cmd {
	case Command_Occupy_Vid:
		cmdT := new(OccupyVidCommand)
		cmdT.id = id
		cmdT.state = cm.State
		cmdT.ctx = ctx
		cmdT.blocks = blocks
		cmd = cmdT

	case Command_Post_Artifact:
		cmdT := new(PostArtifactCommand)
		cmdT.id = id
		cmdT.state = cm.State
		cmdT.ctx = ctx
		cmdT.blocks = blocks
		cmd = cmdT
	}
	return
}

func NewOccupyVidCommand(session *xorm.Session, ctx context.Context, blocks []*Block) (cmd *OccupyVidCommand, err error) {
	cmdM := &CommandModel{}
	cmdM.State = CommandStateOfGenerating
	cmdM.Cmd = Command_Occupy_Vid
	_, err = session.Insert(cmdM)
	if err != nil {
		log.Error("%+v", err.Error())
		return
	}
	for k, v := range blocks {
		v.CurrentCommandId = cmdM.Id
		v.State = BlockStateToOccupy
		v.GenerateVid()
		log.Debug("%d \n", k)
		_, err = session.Insert(v)
		if err != nil {
			log.Error("%+v", err.Error())
			return
		}
	}
	cmd = new(OccupyVidCommand)
	cmd.id = cmdM.Id
	cmd.state = cmdM.State
	cmd.blocks = blocks
	cmd.ctx = ctx
	return
}

func NewPostArtifactCommand(session *xorm.Session, ctx context.Context, blocks []*Block) (cmd *PostArtifactCommand, err error) {
	cmdM := &CommandModel{}
	cmdM.State = CommandStateOfGenerating
	cmdM.Cmd = Command_Post_Artifact
	_, err = session.Insert(cmdM)
	if err != nil {
		log.Error("%+v", err.Error())
		return
	}
	for _, v := range blocks {
		v.CurrentCommandId = cmdM.Id
		v.State = BlockStateToPost
		_, err = session.ID(v.Id).Cols("state", "current_command_id", "").Update(v)
		if err != nil {
			log.Error("%+v", err.Error())
			return
		}
	}
	cmd = new(PostArtifactCommand)
	cmd.id = cmdM.Id
	cmd.state = cmdM.State
	cmd.blocks = blocks
	cmd.ctx = ctx
	return
}

type OccupyVidCommand struct {
	id          int64
	state       CommandState
	blocks      []*Block
	successChan chan *Block
	ctx         context.Context
}

func (self *OccupyVidCommand) Execute(service *Service) (err error) {
	defer func() {
		if err != nil {
			_, err = service.dbEngine.NewSession().ID(self.id).Update(&CommandModel{Error: err.Error(), State: CommandStateOfFail})
		}
		service.RunningCommandIds.Delete(self.id)
		log.Debug("complete OccupyVidCommand!")
	}()

	request := &OccupyVidRequest{}
	request.RequestNo = strconv.FormatInt(self.id, 10)
	for _, v := range self.blocks {
		request.VidList = append(request.VidList, v.Vid)
		//log.Debug("vid:%s \n",v.Vid)
	}

	ctx := context.WithValue(self.ctx, "request", request)

	response, err := OccupyVid(ctx, service.config, service.Token)
	if err != nil {
		log.Error("%+v", err.Error())
		return
	}
	if response.Status == string(CommandStateOfSuccess) {
		//保存当前command的状态
		self.state = CommandStateOfSuccess
		var newCommandIds []int64
		newCommandIds, err = self.next(service.dbEngine.NewSession(), service.CommandChan, response)
		if err != nil {
			log.Error("%+v", err.Error())
			return
		}
		if newCommandIds != nil {
			for _, v := range newCommandIds {
				service.RunningCommandIds.Store(v, true)
			}
		}
	} else {
		err = fmt.Errorf(response.Status)
		log.Error("%+v", err.Error())
		return
	}

	return
}
func (self *OccupyVidCommand) next(session *xorm.Session, commandChan chan ICommand, response *OccupyVidResponse) (newCommandId []int64, err error) {
	persistMutex.Lock()
	defer persistMutex.Unlock()
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

	cmdM := &CommandModel{}
	cmdM.State = CommandStateOfSuccess
	_, err = session.ID(self.id).Update(cmdM)
	if err != nil {
		log.Error(err.Error())
		return
	}

	var successBlocks []*Block
	var failBlocks []*Block
	if response.SuccessList != nil && len(response.SuccessList) > 0 {
		for _, v := range response.SuccessList {
			log.Debug("=====================response: %+v \n", v)
			for _, vv := range self.blocks {
				log.Debug("self: %+v", vv)
				if vv.Vid == v {
					vv.State = BlockStateToPost
					successBlocks = append(successBlocks, vv)
					break
				}
			}
		}
		log.Debug("length of successBlocks:%d", len(successBlocks))
		//生成新的Post
		if successBlocks != nil && len(successBlocks) > 0 {
			var cmd ICommand
			cmd, err = NewPostArtifactCommand(session, self.ctx, successBlocks)
			if err != nil {
				log.Error(err.Error())
				return
			}
			commandChan <- cmd
			newCommandId = append(newCommandId, cmd.GetId())
		}

	}
	if response.FailureList != nil && len(response.FailureList) > 0 {
		for _, v := range response.FailureList {
			for _, vv := range self.blocks {
				if vv.Vid == v {
					vv.GenerateVid()
					failBlocks = append(failBlocks, vv)
					break
				}
			}
		}
		//生成新的Post
		if failBlocks != nil && len(failBlocks) > 0 {
			var cmd ICommand
			cmd, err = NewOccupyVidCommand(session, self.ctx, failBlocks)
			if err != nil {
				log.Error(err.Error())
				return
			}
			commandChan <- cmd
			newCommandId = append(newCommandId, cmd.GetId())
		}
	}
	return
}

func (self *OccupyVidCommand) GetId() int64           { return self.id }
func (self *OccupyVidCommand) GetState() CommandState { return self.state }
func (self *OccupyVidCommand) GetBlocks() []*Block    { return self.blocks }

type PostArtifactCommand struct {
	id          int64
	state       CommandState
	blocks      []*Block
	successChan chan ICommand `xorm:"-"`
	ctx         context.Context
}

func (self *PostArtifactCommand) Execute(service *Service) (err error) {
	defer func() {
		if err != nil {
			_, err = service.dbEngine.NewSession().ID(self.id).Update(&CommandModel{Error: err.Error(), State: CommandStateOfFail})
		}
		service.RunningCommandIds.Delete(self.id)
	}()
	request := &PostArtifactRequest{}
	request.RequestNo = strconv.FormatInt(self.id, 10)
	request.Uid = service.config.UserIdOfYuanZhiLian

	for _, v := range self.blocks {
		requestData := new(PostArtifactRequestData)
		requestData.Vid = v.Vid
		requestData.DataHash = v.Hash
		request.Data = append(request.Data, requestData)
	}

	ctx := context.WithValue(self.ctx, "request", request)

	response, err := PostArtifact(ctx, service.config, service.Token)
	if err != nil {
		log.Error("%+v", err.Error())
		return
	}
	if response.Status == string(CommandStateOfSuccess) {
		self.state = CommandStateOfSuccess
		err = self.next(service.dbEngine.NewSession(), service.SuccessChan, response)
		if err != nil {
			log.Error("%+v", err.Error())
			return
		}
	} else if response.Status == "INSUFFICIENT" {
		err = fmt.Errorf("链上账户余额不足")
		log.Error("%+v", err.Error())
		return
	} else {
		err = fmt.Errorf(response.Status)
		log.Error("%+v", err.Error())
		return
	}
	return
}

func (self *PostArtifactCommand) next(session *xorm.Session, successChan chan *Block, response *PostArtifactResponse) (err error) {
	persistMutex.Lock()
	defer persistMutex.Unlock()
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
			if err == nil {
				log.Debug("add block to channel...")
				for _, v := range self.blocks {
					log.Debug("add block %+v", *v)
					successChan <- v
				}
			}
		}
	}()

	cmdM := &CommandModel{}
	cmdM.State = CommandStateOfSuccess
	_, err = session.ID(self.id).Update(cmdM)
	if err != nil {
		log.Error(err.Error())
		return
	}

	for _, v := range response.TxList {
		log.Debug("Response TxList: %+v", *v)
		for _, vv := range self.blocks {
			//log.Debug("self blocks: %+v",*vv)
			//log.Debug("equal: %+v, vv.vid=%s  v.vid=%s",vv.Vid == v.Vid,vv.Vid,v.Vid)

			if vv.Vid == v.Vid {
				vv.State = BlockStatePosted
				vv.TxId = v.TxId
				vv.ClauseIndex = v.ClauseIndex
				_, err = session.ID(vv.Id).Cols("state,tx_id,clause_index").Update(vv)
				if err != nil {
					log.Error(err.Error())
					return
				}
				break
			}
		}
	}

	return
}

func (self *PostArtifactCommand) GetId() int64           { return self.id }
func (self *PostArtifactCommand) GetState() CommandState { return self.state }
func (self *PostArtifactCommand) GetBlocks() []*Block    { return self.blocks }

type CommandModel struct {
	CommonModel `json:",inline" xorm:"extends"`
	Cmd         string       `json:"cmd" xorm:"varchar(30) default '' index"`
	State       CommandState `json:"state" xorm:"varchar(20) default '' index"`
	Error       string       `json:"error" xorm:"varchar(1000)"`
}

func (self *CommandModel) GetBlock(session *xorm.Session) (blocks []*Block, err error) {
	err = session.Where("current_command_id=?", self.Id).Find(&blocks)
	if err != nil {
		log.Error("%+v", err.Error())
		return
	}
	return
}
func (self *CommandModel) TableName() string {
	return "vechain_command"
}
