package vechain

import (
	"github.com/myafeier/log"
	"math/rand"
	"sync/atomic"
	"time"
)

type IToken interface {
	UpdateToken()error
	GetToken() (token string)
}
type DefaultToken struct {
	config    *VechainConfig
	token     atomic.Value
	expire    int64     `json:"expire"`
	refreshed time.Time `json:"-"` // token更新时间
}

func init() {
	var _ IToken = &DefaultToken{}
}

func NewDefaultToken(config *VechainConfig) *DefaultToken {
	defaultToken := &DefaultToken{config: config}
	defaultToken.token.Store("")
	return defaultToken
}

func (self *DefaultToken) GetToken() string {
	Retry:
	old := (self.token.Load()).(string)
	if old == "" || time.Now().Sub(self.refreshed).Seconds() > float64(self.expire-1600) {
		err:=self.UpdateToken()
		if err!=nil{
			if err==refreshError{
				rand.Seed(time.Now().Unix())
				time.Sleep(time.Duration(rand.Intn(100))*time.Microsecond)
				goto Retry
			}else{
				log.Error(err.Error())
				time.Sleep(10*time.Second)
				goto Retry
			}
		}
		return (self.token.Load()).(string)
	} else {
		return old
	}
}

func (self *DefaultToken) UpdateToken()(err error) {

	token, err := GetToken(self.config)
	if err != nil {
		return
	}
	self.token.Store(token.Token)
	self.refreshed = time.Now()
	self.expire = token.Expire
	return
}
