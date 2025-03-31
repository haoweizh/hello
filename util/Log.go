package util

import (
	"fmt"
	"github.com/gotoeasy/glang/cmn"
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
var logChan = make(chan cmn.GlcData, 100000)

var localCount = 0
var localFile *os.File
var localLogger *log.Logger

const logRoot = "./log/"
const LogLevelError = "error"
const LogLevelLocal = "local"
const LogLevelInfo = "info"
const LogLevelDebug = "debug"

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
	year, month, date := GetNow().Date()
	strDate := strconv.Itoa(year) + month.String() + strconv.Itoa(date)
	strTime := strconv.Itoa(GetNow().Hour()) + "_" + strconv.Itoa(GetNow().Minute())
	return logRoot + name + strDate + "_" + strTime + ".log"
}

func LogChanHandler(apiUrl, serverName string) {
	// 这里用手动初始化替代环境变量自动配置方式，更多选项详见GlcOptions字段说明
	cmn.SetGlcClient(cmn.NewGlcClient(&cmn.GlcOptions{
		ApiUrl:           apiUrl,
		Enable:           "true",
		EnableConsoleLog: "false",
		//ServerName:       serverName,
		System: serverName,
	}))
	for !Terminal {
		glcData := <-logChan
		//glcData.ServerName = serverName
		if apiUrl == `local` {
			if localCount%10000 == 0 {
				if localFile != nil {
					_ = localFile.Close()
				}
				localLogger, localFile, _ = initLog(getPath("local"))
			}
			localCount++
			localLogger.Println(glcData.Text)
		} else {
			if float64(len(logChan))/float64(cap(logChan)) > 0.9 {
				continue
			} else if len(logChan) == cap(logChan)/2 {
				cmn.Error(cmn.GlcData{Text: fmt.Sprintf(`log chan %d`, cap(logChan)/2), LogLevel: `error`})
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

func Log(logLevel, content string) {
	if logLevel == LogLevelLocal {
		if localCount%100000 == 0 {
			if localFile != nil {
				_ = localFile.Close()
			}
			localLogger, localFile, _ = initLog(getPath("local"))
		}
		localCount++
		localLogger.Println(content)
	} else {
		glcData := cmn.GlcData{Text: content, LogLevel: logLevel}
		logChan <- glcData
	}
}

func InfoSync(msg string) {
	logContent := &cmn.GlcData{Text: msg, System: "infoSync"}
	cmn.Info(logContent)
}

func LogLess(logLevel, content string) {
	if time.Now().Second() == 0 {
		Log(logLevel, content)
	}
}
