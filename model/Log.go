package model

import (
	"github.com/gotoeasy/glang/cmn"
	"hello/util"
	"log"
	"os"
	"strconv"
	"time"
)

// var socket, info, notice, debug *log.Logger
// var socketFile, infoFile, noticeFile, debugFile *os.File
// var socketCount, infoCount, noticeCount int
// var DebugCount int

var DoDebug = false
var logChan = make(chan cmn.GlcData, 10000)

var localCount = 0
var localFile *os.File
var localLogger *log.Logger

const logRoot = "./log/"
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

func initLog(path string) (*log.Logger, *os.File, error) {
	//removeOldFiles()
	_, err := os.Stat(logRoot)
	if err != nil && os.IsNotExist(err) {
		_ = os.Mkdir(logRoot, os.ModePerm)
	}
	file, errFile := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, os.ModePerm)
	if errFile != nil {
		return nil, nil, errFile
	}
	return log.New(file, "", log.Ldate|log.Ltime|log.Ldate|log.Lmicroseconds), file, nil
}

func getPath(name string) string {
	year, month, date := util.GetNow().Date()
	strDate := strconv.Itoa(year) + month.String() + strconv.Itoa(date)
	strTime := strconv.Itoa(util.GetNow().Hour()) + "_" + strconv.Itoa(util.GetNow().Minute())
	return logRoot + name + strDate + "_" + strTime + ".log"
}

func logChanHandler() {
	// 这里用手动初始化替代环境变量自动配置方式，更多选项详见GlcOptions字段说明
	cmn.SetGlcClient(cmn.NewGlcClient(&cmn.GlcOptions{
		ApiUrl:           AppConfig.Log,
		Enable:           "true",
		EnableConsoleLog: "false",
	}))
	for {
		glcData := <-logChan
		if AppConfig.Log == `local` {
			if localCount%10000 == 0 {
				if localFile != nil {
					_ = localFile.Close()
				}
				localLogger, localFile, _ = initLog(getPath("local"))
			}
			localCount++
			localLogger.Println(glcData.Text)
		} else {
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
}

func Log(accountKey, logLevel, traceId, systemName, content string) {
	glcData := cmn.GlcData{Text: content, User: accountKey, Date: util.GetNow().String(), LogLevel: logLevel, TraceId: traceId, System: systemName}
	glcData.ServerName = AppConfig.Port
	logChan <- glcData
}

func InfoSync(msg string) {
	logContent := &cmn.GlcData{Text: msg, Date: util.GetNow().String(), System: "infoSync"}
	cmn.Info(logContent)
}

func LogLess(accountKey, logLevel, traceId, systemName, content string) {
	if time.Now().Second() == 0 {
		Log(accountKey, logLevel, traceId, systemName, content)
	}
}
