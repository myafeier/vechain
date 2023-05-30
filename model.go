package vechain

import "time"

type CommonModel struct {
	Id      int64     `json:"id"`
	Created time.Time `json:"created" xorm:"created"`
	Updated time.Time `json:"updated" xorm:"updated"`
}
