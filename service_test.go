package vechain

import (
	"crypto/sha256"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"strconv"
	"testing"
	"time"
	"xorm.io/xorm"
)

const (
	SiteUrl             string = "https://developer.vetoolchain.cn/api/"
	DeveloperId         string = "81e1ae4a965571384d96c92cb92b12a6"
	DeveloperKey        string = "3df2f8ee7cb4a3473194edca859dbb503408edea1c16b47571e7a6724011fe74"
	VeVid               string = "9227.cn.v"
	Address             string = "0xd96199f7c65e14f24943e90398a0d09d1cf3b615"
	UserIdOfYuanZhiLian string = ""
	ExploreLink         string = "https://insight.vecha.in/#/test/%s"
)

var config = &VechainConfig{
	SiteUrl:             SiteUrl,
	DeveloperId:         DeveloperId,
	DeveloperKey:        DeveloperKey,
	VeVid:               VeVid,
	Address:             Address,
	Nonce:               Nonce(),
	UserIdOfYuanZhiLian: UserIdOfYuanZhiLian,
	ExploreLink:         ExploreLink,
}
var engine *xorm.Engine

func init() {
	var err error
	engine, err = xorm.NewEngine("mysql", "test:test@tcp(127.0.0.1:3306)/test?charset=utf8mb4")
	if err != nil {
		log.Println(err)
		return
	}
	engine.SetMaxIdleConns(10)
	engine.SetMaxOpenConns(100)
	engine.SetConnMaxLifetime(100 * time.Second)
	engine.ShowSQL(true)
	go InitService(engine, config)
}
func TestCreateSubAccount(t *testing.T) {
	account := "yuanzhilian"
	//requestNo := fmt.Sprintf("T%d", time.Now().Unix())
	requestNo := "T1597111238"
	uid, err := Daemon.CreateSubAccount(requestNo, account)
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(uid)
}

func TestAsyncSubmit(t *testing.T) {
	var data []string
	for i := 1; i < 101; i++ {
		data = append(data, fmt.Sprintf("0x%X", sha256.Sum256([]byte(strconv.Itoa(i)))))
	}
	err := AsyncSubmit(data)
	if err != nil {
		t.Error(err.Error())
		return
	}

	for {
		var hasCommandRunning bool
		Daemon.RunningCommandIds.Range(func(key, value interface{}) bool {
			hasCommandRunning = true
			return true
		})

		if hasCommandRunning {
			t.Log("has command running...")
			time.Sleep(10 * time.Second)
		} else {
			return
		}
	}

}
