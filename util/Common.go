package util

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"github.com/bitly/go-simplejson"
	"io/ioutil"
	"math"
	"net/url"
	"strconv"
	"time"
)

func UnGzip(byte []byte) []byte {
	r, err := gzip.NewReader(bytes.NewBuffer(byte))
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}
	undatas, _ := ioutil.ReadAll(r)
	if r != nil {
		r.Close()
	}
	return undatas
}

// ToJson
func _(params *url.Values) string {
	paramMap := make(map[string]string)
	for k, v := range *params {
		paramMap[k] = v[0]
	}
	jsonData, _ := json.Marshal(paramMap)
	return string(jsonData)
}

func NewJSON(data []byte) (j *simplejson.Json, err error) {
	j, err = simplejson.NewJson(data)
	if err != nil {
		return nil, err
	}
	return j, nil
}

func JsonDecodeByte(bytes []byte) map[string]interface{} {
	jsonMap := make(map[string]interface{})
	_ = json.Unmarshal(bytes, &jsonMap)
	return jsonMap
}

func JsonEncodeToByte(stringMap interface{}) []byte {
	jsonBytes, err := json.Marshal(stringMap)
	if err != nil {
		return nil
	}
	return jsonBytes
}

func GetNow() time.Time {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err == nil {
		return time.Now().In(location)
	}
	return time.Now()
}

func GetNowUnixMillion() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

func FormatNum(input float64, decimal float64) (num float64, str string) {
	if decimal == 0.5 {
		base := float64(int(math.Round(input*2))) / 2
		return FormatNum(base, 1)
	}
	if decimal == 1.5 {
		base := float64(int(math.Round(input*20))) / 20
		return FormatNum(base, 2)
	}
	format := `%.` + strconv.Itoa(int(decimal)) + `f`
	str = fmt.Sprintf(format, input)
	num, _ = strconv.ParseFloat(str, 64)
	return num, str
}

//StartMidNightTimer
func _(f func()) {
	go func() {
		for {
			now := time.Now()
			next := now.Add(time.Hour * 24)
			next = time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, next.Location())
			t := time.NewTimer(next.Sub(now))
			<-t.C
			f()
		}
	}()
}
