package dtos

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"hello/util"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	ContentType        = "Content-Type"
	BgAccessKey        = "ACCESS-KEY"
	BgAccessSign       = "ACCESS-SIGN"
	BgAccessTimestamp  = "ACCESS-TIMESTAMP"
	BgAccessPassphrase = "ACCESS-PASSPHRASE"
	ApplicationJson    = "application/json"
)

//type Signer struct {
//	secretKey []byte
//}

func Sign(method string, requestPath string, body string, timesStamp string, secret []byte) string {
	var payload strings.Builder
	payload.WriteString(timesStamp)
	payload.WriteString(method)
	payload.WriteString(requestPath)
	if body != "" && body != "?" {
		payload.WriteString(body)
	}
	hash := hmac.New(sha256.New, secret)
	hash.Write([]byte(payload.String()))
	result := base64.StdEncoding.EncodeToString(hash.Sum(nil))
	return result
}

type BitgetRestClient struct {
	ApiKey       string
	ApiSecretKey string
	Passphrase   string
	BaseUrl      string
	//HttpClient   http.Client
	//Signer       *Signer
}

func Headers(request *http.Request, apikey string, timestamp string, sign string, passphrase string) {
	request.Header.Add(ContentType, ApplicationJson)
	request.Header.Add(BgAccessKey, apikey)
	request.Header.Add(BgAccessSign, sign)
	request.Header.Add(BgAccessTimestamp, timestamp)
	request.Header.Add(BgAccessPassphrase, passphrase)
}

func (p *BitgetRestClient) DoPost(uri string, params string) ([]byte, error) {
	//if p.ApiKey == "" || p.ApiSecretKey == "" || p.Passphrase == "" {
	//	keys, secrets := model.AppConfig.GetKeys(model.Bitget)
	//	p.ApiKey = keys[0]
	//	p.ApiSecretKey = secrets[0]
	//	p.Passphrase = model.AppConfig.Phase
	//}
	timesStamp := strconv.FormatInt(time.Now().Unix()*1000, 10)
	//body, _ := BuildJsonParams(params)
	sign := Sign(http.MethodPost, uri, params, timesStamp, []byte(p.ApiSecretKey))
	requestUrl := p.BaseUrl + uri
	buffer := strings.NewReader(params)
	request, err := http.NewRequest(http.MethodPost, requestUrl, buffer)
	Headers(request, p.ApiKey, timesStamp, sign, p.Passphrase)
	if err != nil {
		return []byte(""), err
	}
	client := http.Client{
		Timeout: time.Duration(30) * time.Second,
	}
	var response *http.Response
	response, err = client.Do(request)
	if err != nil {
		return []byte(""), err
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			util.Log(util.LogLevelError, err.Error())
		}
	}(response.Body)
	var bodyStr []byte
	bodyStr, err = io.ReadAll(response.Body)
	if err != nil {
		return []byte(""), err
	}
	//responseBodyString := string(bodyStr)
	return bodyStr, err
}

func (p *BitgetRestClient) DoGet(uri string, params map[string]string) ([]byte, error) {
	//if p.ApiKey == "" || p.ApiSecretKey == "" || p.Passphrase == "" {
	//	keys, secrets := model.AppConfig.GetKeys(model.Bitget)
	//	p.ApiKey = keys[0]
	//	p.ApiSecretKey = secrets[0]
	//	p.Passphrase = model.AppConfig.Phase
	//}
	timesStamp := strconv.FormatInt(time.Now().Unix()*1000, 10)
	body := BuildGetParams(params)
	sign := Sign(http.MethodGet, uri, body, timesStamp, []byte(p.ApiSecretKey))
	requestUrl := p.BaseUrl + uri + body
	request, err := http.NewRequest(http.MethodGet, requestUrl, nil)
	if err != nil {
		return []byte(""), err
	}
	Headers(request, p.ApiKey, timesStamp, sign, p.Passphrase)
	client := http.Client{
		Timeout: time.Duration(30) * time.Second,
	}
	var response *http.Response
	response, err = client.Do(request)
	if err != nil {
		return []byte(""), err
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			util.Log(util.LogLevelError, err.Error())
		}
	}(response.Body)
	var bodyStr []byte
	bodyStr, err = io.ReadAll(response.Body)
	if err != nil {
		return []byte(""), err
	}
	//responseBodyString := string(bodyStr)
	return bodyStr, err
}

func BuildJsonParams(params map[string]string) (string, error) {
	if params == nil {
		return "", errors.New("illegal parameter")
	}
	data, err := json.Marshal(params)
	if err != nil {
		return "", errors.New("json convert string error")
	}
	jsonBody := string(data)
	return jsonBody, nil
}

func BuildGetParams(params map[string]string) string {
	urlParams := url.Values{}
	if params != nil && len(params) > 0 {
		for k := range params {
			urlParams.Add(k, params[k])
		}
	}
	return "?" + urlParams.Encode()
}

func JSONToMap(str string) map[string]interface{} {
	var tempMap map[string]interface{}
	err := json.Unmarshal([]byte(str), &tempMap)
	if err != nil {
		//panic(err)
	}
	return tempMap
}

func NewParams() map[string]string {
	return make(map[string]string)
}

func ToJson(v interface{}) (string, error) {
	result, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(result), nil
}
func powerf(x float64, n int) float64 {
	ans := 1.0
	for n != 0 {
		if n%2 == 1 {
			ans *= x
		}
		x *= x
		n /= 2
	}
	return ans
}

func GetSignedInt(checksum string) string {
	c, _ := strconv.ParseUint(checksum, 10, 64)
	if c > math.MaxInt32 {
		a := c - (1<<31-1)*2 - 2
		return strconv.FormatUint(a, 10)
	}
	return checksum
}
