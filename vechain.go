package vechain

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/myafeier/log"
)

// ============common============

//通用请求表单
type Form struct {
	AppId     string `json:"appid"`
	AppKey    string `json:"appkey"`
	Nonce     string `json:"nonce"`
	Timestamp string `json:"timestamp"`
	Signature string `json:"signature"`
}

//通用返回结构
type ResponseData struct {
	Data    interface{} `json:"data"`
	Code    int         `json:"code"`
	Message string      `json:"message"`
}

func sign(timestamp int64, config *VechainConfig) (signature string) {
	str := fmt.Sprintf("appid=%s&appkey=%s&nonce=%s&timestamp=%d", config.DeveloperId, config.DeveloperKey, config.Nonce, timestamp)
	signature = fmt.Sprintf("%x", sha256.Sum256([]byte(strings.ToLower(str))))
	return
}

//区块链浏览器浏览地址
func BlockChainExploreLink(transactionId string, config *VechainConfig) string {
	return fmt.Sprintf(config.ExploreLink, transactionId)
}

//=========================Token======================
//返回Token结构
type Token struct {
	Token  string `json:"token"`
	Expire int64  `json:"expire"`
}

var lock int32 = 0
var refreshError = fmt.Errorf("token refreshing")

func GetToken(config *VechainConfig) (token *Token, err error) {
	if atomic.LoadInt32(&lock) == 1 {
		err = refreshError
		return
	}
	atomic.StoreInt32(&lock, 1)
	defer atomic.StoreInt32(&lock, 0)

	timestamp := time.Now().Unix()
	form := new(Form)
	form.AppId = config.DeveloperId
	form.AppKey = config.DeveloperKey
	form.Nonce = config.Nonce
	form.Timestamp = strconv.FormatInt(timestamp, 10)
	form.Signature = sign(timestamp, config)

	requestUrl := config.SiteUrl + "v1/tokens"
	formByte, err := json.Marshal(form)
	if err != nil {
		log.Error("%s", err.Error())
		return
	}
	log.Debug("%+v", *form)
	data := bytes.NewReader(formByte)
	retryTimes := 0

Retry:
	retryTimes++
	if retryTimes > 100 {
		time.Sleep(1 * time.Minute)
	} else if retryTimes > 1000 {
		time.Sleep(1 * time.Hour)
	}
	request, err := http.NewRequest("POST", requestUrl, data)
	if err != nil {
		log.Error("%s", err.Error())
		goto Retry
	}
	defer request.Body.Close()

	request.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		log.Error("%s", err.Error())
		goto Retry
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		log.Error("%s", err.Error())
		goto Retry
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Error("%s", err.Error())
		goto Retry
	}
	log.Debug("toke response :%s \n", body)
	respData := new(ResponseData)
	respData.Data = new(Token)
	err = json.Unmarshal(body, respData)
	if respData.Code != 1 {
		err = fmt.Errorf("responseCode:%d error,message:%s\n", respData.Code, respData.Message)
		log.Error(err.Error())
		goto Retry
	}
	token = respData.Data.(*Token)
	return
}

//======================Occupy=============
//抢占请求表单
type OccupyVidRequest struct {
	RequestNo string   `json:"requestNo"`
	VidList   []string `json:"vidList"`
}

//抢占响应结构
type OccupyVidResponse struct {
	RequestNo   string   `json:"requestNo,omitempty"`   // 请求编号
	Url         string   `json:"url,omitempty"`         // 扫码 url
	Quantity    int      `json:"quantity,omitempty"`    //请求的vid个数
	Status      string   `json:"status,omitempty"`      // 生成状态(GENERATING:抢占中，SUCCESS：成功)
	SuccessList []string `json:"successList,omitempty"` // 抢占成功 vid 列表
	FailureList []string `json:"failureList,omitempty"` // 抢占失败 vid 列表
}

// 抢占vid
func OccupyVid(ctx context.Context, config *VechainConfig, tokenServer IToken) (response *OccupyVidResponse, err error) {

	url := "v1/vid/occupy"
	request := (ctx.Value("request")).(*OccupyVidRequest)
	data, err := json.Marshal(request)
	if err != nil {
		log.Error(err.Error())
		return
	}
	//log.Debug("request: %s \n",data)

	var justReturn bool
	go func() {
		<-ctx.Done()
		err = ctx.Err()
		justReturn = true
	}()

	retryTimes := 0

RetryWithNewToken:
	token := tokenServer.GetToken()
Retry:
	retryTimes++

	if justReturn {
		return
	}
	if retryTimes > 100 {
		time.Sleep(1 * time.Minute)
	} else if retryTimes > 1000 {
		time.Sleep(1 * time.Hour)
	}
	req, err := http.NewRequest("POST", config.SiteUrl+url, bytes.NewBuffer(data))
	if err != nil {
		log.Error(err.Error())
		goto Retry
	}

	req.Header.Add("Content-Type", "application/json;charset=utf-8")
	req.Header.Add("language", "zh_hans")
	req.Header.Add("x-api-token", token)

	client := http.DefaultClient

	resp, err := client.Do(req)
	if err != nil {
		log.Error(err.Error())
		goto Retry
	}
	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err.Error())
		goto Retry
	}

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("RemoteServerStatusError code:%d,body:%s", resp.StatusCode, respBody)
		log.Error(err.Error())
		goto Retry
	}

	respData := new(ResponseData)
	respData.Data = new(OccupyVidResponse)

	err = json.Unmarshal(respBody, respData)
	if err != nil {
		log.Error(err.Error())
		return
	}

	if respData.Code == 1 {
		response = respData.Data.(*OccupyVidResponse)
		log.Debug("response %+v \n", *response)
		if response.Status == "GENERATING" {
			time.Sleep(1 * time.Minute)
			goto Retry
		}
		return
	} else if respData.Code == 100004 {
		goto RetryWithNewToken
	} else {
		err = fmt.Errorf("Occupy vid error, remote response Code:%d, MSG: %s.", respData.Code, respData.Message)
		log.Error(err.Error())
	}
	return
}

//================================Post========

type PostArtifactResponse struct {
	RequestNo string                      `json:"requestNo,omitempty"` // 请求编号
	Uid       string                      `json:"uid,omitempty"`       // 上链子账户id
	Status    string                      `json:"status,omitempty"`    // 生成状态(PROCESSING:上链中，SUCCESS：成功，FAILURE： 失败,INSUFFICIENT:费用不足)
	TxList    []*PostArtifactResponseData `json:"txList，omitempty"`    //上链结果
}
type PostArtifactResponseData struct {
	TxId        string `json:"txid"`        //上链事务id
	ClauseIndex string `json:"clauseIndex"` // 每40个vid组成一个clause
	Vid         string `json:"vid"`         //商品ID
	DataHash    string `json:"dataHash"`    //？
}

type PostArtifactRequest struct {
	RequestNo string                     `json:"requestNo"` //请求编号
	Uid       string                     `json:"uid"`       //用户 Id
	Data      []*PostArtifactRequestData `json:"data,omitempty"`
}
type PostArtifactRequestData struct {
	DataHash string `json:"dataHash"`
	Vid      string `json:"vid"`
}

// 异步上链
//
func PostArtifact(ctx context.Context, config *VechainConfig, tokenServer IToken) (response *PostArtifactResponse, err error) {

	url := "v1/artifacts/hashinfo/create"

	request := (ctx.Value("request")).(*PostArtifactRequest)
	var data []byte
	data, err = json.Marshal(request)
	if err != nil {
		log.Error(err.Error())
		return
	}

	justReturn := false
	go func() {
		<-ctx.Done()
		justReturn = true
		err = ctx.Err()
		log.Debug("ctx deadLine: %s", err.Error())
		return
	}()

	retryTimes := 0
RetryWithNewToken:
	token := tokenServer.GetToken()
Retry:
	retryTimes++

	if justReturn {
		return
	}

	if retryTimes > 100 {
		time.Sleep(1 * time.Minute)
	} else if retryTimes > 1000 {
		time.Sleep(1 * time.Hour)
	}

	req, err := http.NewRequest("POST", config.SiteUrl+url, bytes.NewBuffer(data))
	if err != nil {
		log.Error(err.Error())
		return
	}
	req.Header.Add("Content-Type", "application/json;charset=utf-8")
	req.Header.Add("language", "zh_hans")
	req.Header.Add("x-api-token", token)

	client := http.DefaultClient

	resp, err := client.Do(req)
	if err != nil {
		log.Error(err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("[Error] After 10 times retry,RemoteServerStatusError code:%d", resp.StatusCode)
		log.Error(err.Error())
		goto Retry

	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err.Error())
		goto Retry
	}

	respData := new(ResponseData)
	respData.Data = new(PostArtifactResponse)

	err = json.Unmarshal(respBody, respData)
	if err != nil {
		log.Error(err.Error())
		goto Retry
	}

	if respData.Code == 1 {
		log.Debug("postArtifact response %s,%+v \n", respData.Message, respData.Data)
		response = respData.Data.(*PostArtifactResponse)
		if response.Status == "PROCESSING" {
			time.Sleep(1 * time.Minute)
			goto Retry
		}
	} else if respData.Code == 100004 {
		goto RetryWithNewToken
	} else {
		err = fmt.Errorf("PostArtifactResponseerror, remote response Code:%d, MSG: %s.", respData.Code, respData.Message)
		log.Error(err.Error())
		return
	}
	return
}

//================CreateAccount============
type CreateUser struct {
	RequestNo string `json:"requestNo"` //请求编号
	Uid       string `json:"uid"`       //用户 Id（已分配时返回）
	Status    string `json:"status"`    //状态（PROCESSING:处理中，SUCCESS：成功，FAILURE：失败）
}

// 创建账号
//  在此系统只只需创建一个账号，无多账户的需求。
func GenerateSubAccount(requestNo, accountName string, config *VechainConfig, tokenServer IToken) (uid string, err error) {
	url := "v1/artifacts/user/create"
	postData := `
		{
			"requestNo":"%s",
			"name":"%s"
		}
	`

	req, err := http.NewRequest("POST", config.SiteUrl+url, bytes.NewBufferString(fmt.Sprintf(postData, requestNo, accountName)))
	if err != nil {
		log.Error(err.Error())
		return
	}
	req.Header.Add("Content-Type", "application/json;charset=utf-8")
	req.Header.Add("language", "zh_hans")
	req.Header.Add("x-api-token", tokenServer.GetToken())

	client := http.DefaultClient

	resp, err := client.Do(req)
	if err != nil {
		log.Error(err.Error())
		return
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err.Error())
		return
	}

	respData := new(ResponseData)
	respData.Data = new(CreateUser)

	err = json.Unmarshal(respBody, respData)
	if err != nil {
		log.Error(err.Error())
		return
	}

	if respData.Code == 1 {
		log.Debug("response %d,%s,%+v \n", resp.StatusCode, resp.Status, *(respData.Data.(*CreateUser)))
	}
	uid = respData.Data.(*CreateUser).Uid
	return

}
