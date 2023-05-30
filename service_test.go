package vechain

import (
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gf-third/yaml"
	_ "github.com/go-sql-driver/mysql"
	"xorm.io/xorm"
)

var engine *xorm.Engine

func ainit() {
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
	f, err := os.Open("./config.yml")
	if err != nil {
		panic(err)
	}
	decoder := yaml.NewDecoder(f)
	var config VechainConfig
	err = decoder.Decode(&config)
	if err != nil {
		panic(err)
	}
	go InitService(engine, &config)
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

}
