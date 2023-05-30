package vechain

import "time"

type BlockState int8

const (
	BlockStateToPost BlockState = 2 //待上链
	BlockStatePosted BlockState = 3 //已上链
)

type Block struct {
	CommonModel `json:",inline" xorm:"extends"`
	Hash        string     `json:"hash"  xorm:"varchar(100) default '' index" `  //Hash值
	Vid         string     `json:"vid" xorm:"varchar(100) default ''"`           //occupy 之后的vid
	TxId        string     `json:"tx_id"  xorm:"varchar(100) default ''"`        //交易ID
	ClauseIndex string     `json:"clause_index"  xorm:"varchar(100) default ''"` //上链分批索引
	State       BlockState `json:"state" xorm:"tinyint(2) default 0 index"`      //区块状态
	SubmitTime  time.Time  `json:"submit_time" xorm:""`                          //上联时间
	ExplorUrl   string     `json:"explor_url" xorm:"-"`
}

func (self *Block) TableName() string {
	return "vechain_block"
}

func (b *Block) GetExplorUrl() {
	b.ExplorUrl = BlockChainExploreLink(b.TxId, Daemon.config)
	return
}
