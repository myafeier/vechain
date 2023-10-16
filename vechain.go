package vechain

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	perrors "github.com/pkg/errors"

	"github.com/myafeier/log"
)

// ============common============

// 通用请求表单
type Form struct {
	AppId     string `json:"appid"`
	AppKey    string `json:"-"`
	Nonce     string `json:"nonce"`
	Timestamp string `json:"timestamp"`
	Signature string `json:"signature"`
}

func (f *Form) sign() {
	str := fmt.Sprintf("appid=%s&appkey=%s&nonce=%s&timestamp=%s", f.AppId, f.AppKey, f.Nonce, f.Timestamp)
	f.Signature = fmt.Sprintf("%x", sha256.Sum256([]byte(strings.ToLower(str))))
}

// 通用返回结构
type ResponseData struct {
	Data    interface{} `json:"data,omitempty"`
	Code    string      `json:"code,omitempty"`
	Message string      `json:"message,omitempty"`
}

// 区块链浏览器浏览地址
func BlockChainExploreLink(transactionId string, config *VechainConfig) string {
	return fmt.Sprintf(config.ExploreLink, transactionId)
}

// =========================Token======================
// 返回Token结构
type Token struct {
	Token      string `json:"token"`
	Expire     int64  `json:"expire"`
	TimeToLive int64  `json:"timeTolive"` //剩余时间
}

var lock int32 = 0
var refreshError = errors.New("token refreshing")

func GetToken(config *VechainConfig) (token *Token, err error) {
	if atomic.LoadInt32(&lock) == 1 {
		err = refreshError
		return
	}
	atomic.StoreInt32(&lock, 1)
	defer atomic.StoreInt32(&lock, 0)

	retryTimes := 0
Retry:
	if retryTimes > 10 {
		err = errors.New("获取token失败")
		return
	}
	token, err = getToken(retryTimes, config)
	if err != nil {
		retryTimes++
		goto Retry
	}
	return
}

func getToken(retryTimes int, config *VechainConfig) (token *Token, err error) {

	timestamp := time.Now().Unix()
	form := new(Form)
	form.AppId = config.DeveloperId
	form.AppKey = config.DeveloperKey
	form.Nonce = Nonce()
	form.Timestamp = strconv.FormatInt(timestamp, 10)
	form.sign()

	log.Debug("get token: No.%d, form %+v", retryTimes, *form)

	requestUrl := config.SiteUrl + "v2/tokens"
	formByte, err := json.Marshal(form)
	if err != nil {
		panic(err)
	}

	data := bytes.NewReader(formByte)

	time.Sleep(time.Duration(retryTimes) * time.Minute)
	request, err := http.NewRequest("POST", requestUrl, data)
	if err != nil {
		log.Error("%s", err.Error())
		return
	}
	defer request.Body.Close()

	request.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		log.Error("%s", err.Error())
		return
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		log.Error("%s", err.Error())
		return
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Error("%s", err.Error())
		return
	}
	log.Debug("toke response :%s \n", body)
	respData := new(ResponseData)
	respData.Data = new(Token)
	err = json.Unmarshal(body, respData)
	if respData.Code != "common.success" {
		err = fmt.Errorf("responseCode:%s error,message:%+v", respData.Code, respData.Data)
		log.Error(err.Error())
		return
	}
	token = respData.Data.(*Token)
	return
}

type GenerateRequest struct {
	RequestNo string `json:"requestNo,omitempty"`
	Quantity  int    `json:"quantity,omitempty"`
}
type GenerateResponse struct {
	RequestNo   string   `json:"requestNo,omitempty"`
	OrderStatus string   `json:"orderStatus,omitempty"`
	Quantity    int      `json:"quantity,omitempty"`
	VidList     []string `json:"vidList,omitempty"`
	Status      string   `json:"status,omitempty"`
}

// 批量生成
func Generate(ctx context.Context, config *VechainConfig, tokenServer IToken) (response *GenerateResponse, err error) {

	url := "v2/vid/generate"
	request := (ctx.Value("request")).(*GenerateRequest)
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
		time.Sleep(1 * time.Hour)
	} else if retryTimes > 10 {
		time.Sleep(1 * time.Minute)
	}
	req, err := http.NewRequest("POST", config.SiteUrl+url, bytes.NewBuffer(data))
	if err != nil {
		log.Error(err.Error())
		goto Retry
	}

	req.Header.Add("Content-Type", "application/json;charset=utf-8")
	req.Header.Add("x-api-token", token)

	client := http.DefaultClient

	resp, err := client.Do(req)
	if err != nil {
		log.Error(err.Error())
		goto Retry
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
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
	respData.Data = new(GenerateResponse)

	err = json.Unmarshal(respBody, respData)
	if err != nil {
		log.Error(err.Error())
		return
	}

	if respData.Code == "common.success" {
		response = respData.Data.(*GenerateResponse)
		log.Debug("response %+v \n", *response)
		if response.Status == "PROCESSING" {
			time.Sleep(1 * time.Minute)
			goto Retry
		}
		if len(response.VidList) != request.Quantity {
			err = perrors.WithStack(fmt.Errorf("生成的vid数量:%d和请求数量不一致:%d", len(response.VidList), request.Quantity))
			return
		}
		return
	} else if respData.Code == "" {
		goto RetryWithNewToken
	} else {
		err = fmt.Errorf("occupy vid error, remote response code:%s, msg: %s", respData.Code, respData.Message)
		log.Error(err.Error())
	}
	return
}

type Hash struct {
	Vid      string `json:"vid"`
	DataHash string `json:"dataHash"`
}
type SubmitRequest struct {
	RequestNo   string  `json:"requestNo,omitempty"`
	OperatorUID string  `json:"operatorUID,omitempty"`
	HashList    []*Hash `json:"hashList"`
}

type Tx struct {
	TxId        string `json:"txId"` // 交易号
	Vid         string `json:"vid"`  //
	DataHash    string `json:"dataHash"`
	SubmitTime  int64  `json:"submitTime"`
	ClauseIndex int64  `json:"clauseIndex"` //当前区块链交易号下的clause index
	Duplicated  bool   `json:"duplicated"`  //是否重复上链
}
type SubmitResponse struct {
	RequestNo string `json:"requestNo,omitempty"`
	Status    string `json:"orderStatus,omitempty"`
	TxList    []*Tx  `json:"txList"`
}

// 上链
func Submit(ctx context.Context, config *VechainConfig, tokenServer IToken) (response *SubmitResponse, err error) {

	url := "v2/provenance/hash/create"
	request := (ctx.Value("request")).(*SubmitRequest)
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
		time.Sleep(1 * time.Hour)
	} else if retryTimes > 10 {
		time.Sleep(1 * time.Minute)
	}
	req, err := http.NewRequest("POST", config.SiteUrl+url, bytes.NewBuffer(data))
	if err != nil {
		log.Error(err.Error())
		goto Retry
	}

	req.Header.Add("Content-Type", "application/json;charset=utf-8")
	req.Header.Add("x-api-token", token)

	client := http.DefaultClient

	resp, err := client.Do(req)
	if err != nil {
		log.Error(err.Error())
		goto Retry
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error(err.Error())
		goto Retry
	}
	log.Debug("%s", respBody)

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("RemoteServerStatusError code:%d,body:%s", resp.StatusCode, respBody)
		log.Error(err.Error())
		goto Retry
	}

	respData := new(ResponseData)
	respData.Data = new(SubmitResponse)

	err = json.Unmarshal(respBody, respData)
	if err != nil {
		log.Error(err.Error())
		return
	}

	if respData.Code == "common.success" {
		response = respData.Data.(*SubmitResponse)
		log.Debug("response %+v \n", *response)
		if response.Status == "PROCESSING" {
			time.Sleep(1 * time.Minute)
			goto Retry
		}
		return
	} else if respData.Code == "" {
		goto RetryWithNewToken
	} else {
		err = fmt.Errorf("occupy vid error, remote response code:%s, msg: %s", respData.Code, respData.Message)
		log.Error(err.Error())
	}
	return
}

// ================CreateAccount============
type CreateUser struct {
	RequestNo string `json:"requestNo"` //请求编号
	Uid       string `json:"uid"`       //用户 Id（已分配时返回）
	Status    string `json:"status"`    //状态（PROCESSING:处理中，SUCCESS：成功，FAILURE：失败）
}

// 创建账号
//
//	在此系统只只需创建一个账号，无多账户的需求。
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

	respBody, err := io.ReadAll(resp.Body)
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

	if respData.Code == "" {
		log.Debug("response %d,%s,%+v \n", resp.StatusCode, resp.Status, *(respData.Data.(*CreateUser)))
	}
	uid = respData.Data.(*CreateUser).Uid
	return

}
