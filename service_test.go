package vechain

import (
	"crypto/sha256"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/go-xorm/xorm"
	"log"
	"strconv"
	"testing"
	"time"
)

const (
	SiteUrl             string = "https://developer.vetoolchain.cn/api/"
	DeveloperId         string = "d3cc3ba9cfcd8ee16cf23da6248ec387"
	DeveloperKey        string = "64afabfabb7abfce475c2c22d09b163230e4ed7827730d16cc040a6ffea8e88f"
	VeVid               string = "9026.cn.v"
	Address             string = "0x56fdca23fe3be4a760ec4811a9c5de37c0c1c425"
	UserIdOfYuanZhiLian string = "0x9b35ab7887e1e58452638f02797833d412c61956a869e36d728fa4fd215ef798"
	ExploreLink         string = "https://insight.vecha.in/#/test/%s"
)

var config =&VechainConfig{
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
	go InitService(engine,config)
}

func TestAsyncSubmit(t *testing.T) {
	data:=make(map[int64]string)
	for i:=1;i<101;i++{
		data[int64(i)]=fmt.Sprintf("0x%X",sha256.Sum256([]byte(strconv.Itoa(i))))
	}
	err:=AsyncSubmit(data)
	if err != nil {
		t.Error(err.Error())
		return
	}

	for {
		var hasCommandRunning bool
		Daemon.RunningCommandIds.Range(func(key, value interface{}) bool {
			hasCommandRunning=true
			return true
		})

		if hasCommandRunning{
			t.Log("has command running...")
			time.Sleep(10*time.Second)
		}else{
			return
		}
	}


}
