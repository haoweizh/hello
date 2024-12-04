package util

import (
	"github.com/gotoeasy/glang/cmn"
	"time"
)

// var socket, info, notice, debug *log.Logger
// var socketFile, infoFile, noticeFile, debugFile *os.File
// var socketCount, infoCount, noticeCount int
// var DebugCount int

var DoDebug = false
var logChan = make(chan cmn.GlcData, 10000)

const LogLevelError = "error"
const LogLevelInfo = "info"
const LogLevelDebug = "debug"
const SystemOther = `other`
const SystemAPI = "api"
const SystemCarry = `carry`
const SystemNetwork = "network"

//const logRoot = "./log/"

func init() {
	go logChanHandler()
}

func logChanHandler() {
	// 这里用手动初始化替代环境变量自动配置方式，更多选项详见GlcOptions字段说明
	cmn.SetGlcClient(cmn.NewGlcClient(&cmn.GlcOptions{
		ApiUrl:           "http://47.97.2.61:8080/",
		Enable:           "true",
		EnableConsoleLog: "false",
	}))
	for {
		glcData := <-logChan
		if len(logChan) > 9000 {
			continue
		} else if len(logChan) == 9000 {
			cmn.Error(cmn.GlcData{Text: `log chan 9000`, LogLevel: `error`})
		}
		switch glcData.LogLevel {
		case LogLevelError:
			cmn.Error(glcData)
		case LogLevelInfo:
			cmn.Info(glcData)
		case LogLevelDebug:
			cmn.Debug(glcData)
		}
	}
}

func Log(accountKey, logLevel, traceId, systemName, content string) {
	glcData := cmn.GlcData{Text: content, User: accountKey, Date: GetNow().String(), LogLevel: logLevel, TraceId: traceId, System: systemName}
	logChan <- glcData
}
func InfoSync(msg string) {
	logContent := &cmn.GlcData{Text: msg, Date: GetNow().String(), System: "infoSync"}
	cmn.Info(logContent)
}

func LogLess(accountKey, logLevel, traceId, systemName, content string) {
	if time.Now().Second() == 0 {
		Log(accountKey, logLevel, traceId, systemName, content)
	}
}
