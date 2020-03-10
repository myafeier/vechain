package vechain

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/go-xorm/xorm"
)

const (
	CheckFailDuration     = 10 * time.Minute //检查错误的时间间隔
	CommandExpireDuration = 24 * time.Hour   //命令超时时间
	ItemAmountPerRequest  = 100              //一次抢占包含的vid数量
)

type VechainConfig struct {
	SiteUrl             string `yaml:"SiteUrl"`
	DeveloperId         string `yaml:"DeveloperId"`
	DeveloperKey        string `yaml:"DeveloperKey"`
	VeVid               string `yaml:"VeVid"`
	Address             string `yaml:"Address"`
	Nonce               string `yaml:"-"`
	UserIdOfYuanZhiLian string `yaml:"UserIdOfYuanZhiLian"`
	ExploreLink         string `yaml:"ExploreLink"`
}

func Nonce() string {
	return fmt.Sprintf("%016v", rand.New(rand.NewSource(time.Now().UnixNano())).Int63n(9999999999999999))
}

func initTable(session *xorm.Session) (err error) {
	var tables = []interface{}{&Block{}, &CommandModel{}}

	for _, v := range tables {
		var isExist bool
		isExist, err = session.IsTableExist(v)
		if err != nil {
			panic(v)
		}
		if !isExist {
			err = session.CreateTable(v)
			if err != nil {
				panic(err)
				return
			}
			err = session.CreateIndexes(v)
			if err != nil {
				panic(err)
				return
			}
		} else {
			err = session.Sync2(v)
			if err != nil {
				panic(err)
			}
		}
	}
	return
}
