package vechain

import (
	"crypto/sha256"
	"fmt"
	"math/rand"
	"time"
)

type BlockState int8

const (
	BlockStateToOccupy BlockState = 1 //待抢占
	BlockStateToPost   BlockState = 2 //待上链
	BlockStatePosted   BlockState = 3 //已上链
)

type Block struct {
	CommonModel      `json:",inline" xorm:"extends"`
	Hash             string     `json:"hash"  xorm:"varchar(100) default '' index" `  //Hash值
	Vid              string     `json:"vid" xorm:"varchar(100) default ''"`           //occupy 之后的vid
	TxId             string     `json:"tx_id"  xorm:"varchar(100) default ''"`        //交易ID
	ClauseIndex      string     `json:"clause_index"  xorm:"varchar(100) default ''"` //上链分批索引
	State            BlockState `json:"state" xorm:"tinyint(2) default 0 index"`      //区块状态
	CurrentCommandId int64      `json:"current_command_id" xorm:"default 0 index"`    //当前进行中的命令id，停留在最后一个命令的id
	ExplorUrl        string     `json:"explor_url" xorm:"-"`
}

func (self *Block) TableName() string {
	return "vechain_block"
}

func (b *Block) GenerateVid() {
	rand.Seed(time.Now().UnixNano())
	b.Vid = fmt.Sprintf("0X%X", sha256.Sum256([]byte(fmt.Sprintf("%s%d", b.Hash, rand.Int63()))))
}
func (b *Block) GetExplorUrl() {
	b.ExplorUrl = BlockChainExploreLink(b.TxId, Daemon.config)
	return
}
