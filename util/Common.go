package util

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"github.com/bitly/go-simplejson"
	"io"
	"math"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var Terminal = false

func Compress(content []byte) []byte {
	var b bytes.Buffer
	writer := gzip.NewWriter(&b)
	if _, err := writer.Write(content); err != nil {
		Log(LogLevelError, `fail to compress `+err.Error())
	}
	if err := writer.Close(); err != nil {
		Log(LogLevelError, `fail to compress `+err.Error())
	}
	return b.Bytes()
}

func UnGzip(byte []byte) []byte {
	r, err := gzip.NewReader(bytes.NewBuffer(byte))
	if err != nil {
		Log(LogLevelError,
			fmt.Sprintf(`fail to un-compress %s, %d`, err.Error(), len(byte)))
		return nil
	}
	var data, _ = io.ReadAll(r)
	if r != nil {
		err = r.Close()
		if err != nil {
			return nil
		}
	}
	return data
}

func CutTailZero(in string) (out string) {
	out = strings.Trim(in, ` `)
	if strings.Contains(out, `.`) {
		out = strings.Trim(out, `0`)
	}
	if out[0] == '.' {
		out = `0` + out
	}
	return strings.Trim(out, `.`)
}

func EndWith(full, part string) bool {
	beginLen := len(full) - len(part)
	if beginLen < 0 {
		return false
	}
	return full[beginLen:] == part
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

func JsonEncodeToByte(stringMap interface{}) []byte {
	if stringMap == nil {
		return []byte(``)
	}
	jsonBytes, err := json.Marshal(stringMap)
	if err != nil {
		return nil
	}
	return jsonBytes
}

func GetToday() (today time.Time) {
	today = GetNow()
	return time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
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

// NumDecPlaces 返回小数点后有效数字位数
func NumDecPlaces(v float64) int {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	i := strings.IndexByte(s, '.')
	if i > -1 {
		return len(s) - i - 1
	}
	return 0
}

func DelSyncMap(syncMap *sync.Map, keys ...string) {
	if syncMap == nil {
		return
	}
	key := ``
	for i := 0; i < len(keys); i++ {
		key += keys[i] + `*`
	}
	syncMap.Delete(key)
}

func LoadSyncMap(syncMap *sync.Map, keys ...string) (interface{}, bool) {
	if syncMap == nil {
		Log(LogLevelError, fmt.Sprintf(`syncMap is nil %v`, keys))
		return nil, false
	}
	key := ``
	for i := 0; i < len(keys); i++ {
		key += keys[i] + `*`
	}
	return syncMap.Load(key)
}

func StoreSyncMap(syncMap *sync.Map, value interface{}, keys ...string) {
	//if syncMap == nil {
	//	return
	//}
	key := ``
	for i := 0; i < len(keys); i++ {
		key += keys[i] + `*`
	}
	syncMap.Store(key, value)
}
