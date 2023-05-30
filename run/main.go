/*
这是一个调用示例
*/
package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/myafeier/log"
	"github.com/myafeier/vechain"
	"gopkg.in/yaml.v2"
	"xorm.io/xorm"
)

var engine *xorm.Engine
var config vechain.VechainConfig

func init() {
	fs, err := os.Open("./config.yml")
	if err != nil {
		panic(err)
	}
	defer fs.Close()
	err = yaml.NewDecoder(fs).Decode(&config)
	if err != nil {
		panic(err)
	}

	engine, err = xorm.NewEngine("mysql", "test:test@tcp(127.0.0.1:3306)/test?charset=utf8mb4")
	if err != nil {
		log.Error(err.Error())
		return
	}
	engine.SetMaxIdleConns(10)
	engine.SetMaxOpenConns(100)
	engine.SetConnMaxLifetime(100 * time.Second)
	engine.ShowSQL(true)

}

func main() {
	var data []string
	for i := 1; i < 5; i++ {
		data = append(data, fmt.Sprintf("0x%X", sha256.Sum256([]byte("s"+strconv.Itoa(i)))))
	}
	ctx, cancel := context.WithCancel(context.Background())
	vechain.InitService(ctx, engine, &config)
	time.Sleep(2 * time.Second)

	err := vechain.AsyncSubmit(data)
	if err != nil {
		log.Error(err.Error())
		return
	}
	cancel()
}
