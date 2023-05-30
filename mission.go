package vechain

type MissionType int8
type MissionStatus int8

const (
	MissionTypeOfGenerate  MissionType   = 1  //生成vid
	MissionTypeOfSubmit    MissionType   = 2  //提交上链
	MissionStatusOfSuccess MissionStatus = 1  //成功
	MissionStatusOfFail    MissionStatus = -1 //失败
)

type Mission struct {
	CommonModel `json:",inline" xorm:"extends"`
	RequestNo   string        `json:"request_no,omitempty"`
	Status      MissionStatus `json:"status,omitempty"`
	MissionType MissionType   `json:"mission_type,omitempty"`
	BlockIds    []int64       `json:"block_ids,omitempty"`
	Message     string        `json:"message,omitempty"`
}

func (m *Mission) TableName() string {
	return "mission"
}
