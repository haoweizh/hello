package dtos

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	XT_VALIDATE_ALGORITHMS            = "HmacSHA256"
	XT_VALIDATE_RECVWINDOW            = "60000"
	XT_VALIDATE_CONTENTTYPE_URLENCODE = "application/x-www-form-urlencoded"
	XT_VALIDATE_CONTENTTYPE_JSON      = "application/json;charset=UTF-8"
)

type SignedFutureHttpAPI struct {
	Accesskey string
	Secretkey string
}

type SignedHttpAPI struct {
	Accesskey string
	Secretkey string
}

type Auth struct {
	urlencoded bool
	signed     SignedHttpAPI
	path       string
	method     string
}

type AuthFuture struct {
	urlencoded bool
	signed     SignedFutureHttpAPI
	path       string
	method     string
}

/**
 * @description:
 * @param {SignedHttpAPI} signed
 * @param {*} path
 * @param {string} method
 * @return {*}
 */
func NewAuth(signed SignedHttpAPI, path, method string) *Auth {

	return &Auth{
		signed: signed,
		path:   path,
		method: method,
	}
}

func NewAuthFuture(signed SignedFutureHttpAPI, path, method string) *AuthFuture {

	return &AuthFuture{
		signed: signed,
		path:   path,
		method: method,
	}
}

// To generate the signature
/**
 * @description:
 * @param {*} nil
 * @return {*}
 */
func createSigned(xy, secret string) string {
	keys := []byte(secret)
	h := hmac.New(sha256.New, keys)
	h.Write([]byte(xy))

	return hex.EncodeToString(h.Sum(nil))
}

// urlencode encoding is determined
/**
 * @description:
 * @param {bool} value
 * @return {*}
 */
func (a *Auth) SetUrlencode(value bool) {
	a.urlencoded = value
}

func (a *AuthFuture) SetUrlencode(value bool) {
	a.urlencoded = value
}

// Generating request headers
/**
 * @description:
 * @param {*} xt
 * @param {*} value
 * @return {*}
 */
func (a *Auth) createHeader() url.Values {
	u := url.Values{}
	u.Set("xt-validate-algorithms", XT_VALIDATE_ALGORITHMS)
	u.Set("xt-validate-appkey", a.signed.Accesskey)
	u.Set("xt-validate-recvwindow", XT_VALIDATE_RECVWINDOW)
	nt := time.Now().UnixMilli()
	value := strconv.FormatInt(nt, 10)
	u.Set("xt-validate-timestamp", value)

	return u
}

func (a AuthFuture) escape(data map[string]interface{}) (tmp string, err error) {

	u := make([]string, 0)
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		switch i := data[k].(type) {
		case string:
			u = append(u, fmt.Sprintf("%s=%s", k, i))
		case int64:
			value := strconv.FormatInt(i, 10)
			u = append(u, fmt.Sprintf("%s=%s", k, value))
		default:
			bt, err := json.Marshal(i)
			if err != nil {
				return "", err
			}
			u = append(u, fmt.Sprintf("%s=%s", k, string(bt)))
		}
	}
	tmp = strings.Join(u, "&")

	return
}

func (a *AuthFuture) createHeaderFuture() url.Values {
	nt := time.Now().UnixMilli()
	u := url.Values{}
	u.Set("xt-validate-appkey", a.signed.Accesskey)
	value := strconv.FormatInt(nt, 10)
	u.Set("xt-validate-timestamp", value)
	return u
}

func (a AuthFuture) CreatePayloadFuture(data map[string]interface{}) (headers map[string]string, err error) {
	var tmp, decode, X, Y string

	// 构造X
	header := a.createHeaderFuture()
	X = header.Encode()
	decode = XT_VALIDATE_CONTENTTYPE_JSON

	if a.urlencoded {
		tmp, err = a.escape(data)
		if err != nil {
			return
		}
		decode = XT_VALIDATE_CONTENTTYPE_URLENCODE
	}

	if len(data) <= 0 {
		Y = fmt.Sprintf("#%s", a.path)
	} else {
		bt, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}

		param := string(bt)
		if tmp != "" {
			Y = fmt.Sprintf("#%s#%s", a.path, tmp)
		} else {
			Y = fmt.Sprintf("#%s#%s", a.path, param)
		}
	}

	signature := createSigned(X+Y, a.signed.Secretkey)
	header.Set("xt-validate-signature", signature)
	header.Set("Content-Type", decode)

	headers = make(map[string]string)
	for k, v := range header {
		headers[k] = v[0]
	}

	return
}

func (a Auth) CreatePayload(data map[string]interface{}) (headers map[string]string, err error) {
	var tmp, decode, X, Y string

	// 构造X
	header := a.createHeader()
	X = header.Encode()
	decode = XT_VALIDATE_CONTENTTYPE_JSON

	if a.urlencoded {
		u := url.Values{}
		for k, v := range data {
			switch i := v.(type) {
			case string:
				u.Set(k, i)
			case int64:
				value := strconv.FormatInt(i, 10)
				u.Set(k, value)
			default:
				bt, err := json.Marshal(i)
				if err != nil {
					return nil, err
				}
				u.Set(k, string(bt))
			}
		}
		tmp = u.Encode()
		decode = XT_VALIDATE_CONTENTTYPE_URLENCODE
	}

	if len(data) <= 0 {
		Y = fmt.Sprintf("#%s#%s", a.method, a.path)
	} else {
		bt, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}

		param := string(bt)
		if tmp != "" {
			Y = fmt.Sprintf("#%s#%s#%s", a.method, a.path, tmp)
		} else {
			Y = fmt.Sprintf("#%s#%s#%s", a.method, a.path, param)
		}
	}

	signature := createSigned(X+Y, a.signed.Secretkey)
	header.Set("xt-validate-signature", signature)
	header.Set("Content-Type", decode)

	headers = make(map[string]string)
	for k, v := range header {
		headers[k] = v[0]
	}

	return
}
