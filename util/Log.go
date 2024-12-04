package util

import (
	"fmt"
	"github.com/gotoeasy/glang/cmn"
	"time"
)

// var socket, info, notice, debug *log.Logger
// var socketFile, infoFile, noticeFile, debugFile *os.File
// var socketCount, infoCount, noticeCount int
// var DebugCount int
var DoDebug = false
var logChan = make(chan string, 10000)

const logRoot = "./log/"

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
		msg := <-logChan
		msgType := msg[0:7]
		msgContent := msg[7:]
		switch msgType {
		case `info   `:
			logContent := &cmn.GlcData{Text: msgContent, Date: GetNow().String(), System: "info"}
			cmn.Info(logContent)
			//info.Println(msgContent)
		case `notice `:
			logContent := &cmn.GlcData{Text: msgContent, Date: GetNow().String(), System: "notice"}
			cmn.Info(logContent)
			//notice.Println(msgContent)
		case `socket `:
			logContent := &cmn.GlcData{Text: msgContent, Date: GetNow().String(), System: "socket"}
			cmn.Info(logContent)
			//socket.Println(msgContent)
		case `debug  `:
			logContent := &cmn.GlcData{Text: msgContent, Date: GetNow().String(), System: "debug"}
			cmn.Debug(logContent)
			//debug.Println(msgContent)
		}
	}
}

//func initLog(path string) (*log.Logger, *os.File, error) {
//	//removeOldFiles()
//	_, err := os.Stat(logRoot)
//	if err != nil && os.IsNotExist(err) {
//		_ = os.Mkdir(logRoot, os.ModePerm)
//	}
//	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, os.ModePerm)
//	if err != nil {
//		return nil, nil, err
//	}
//	return log.New(file, "", log.Ldate|log.Ltime|log.Ldate|log.Lmicroseconds), file, nil
//}

//func removeOldFiles() {
//	year, month, date := GetNow().Date()
//	strDate := strconv.Itoa(year) + month.String() + strconv.Itoa(date)
//	err := filepath.Walk(logRoot, func(path string, f os.FileInfo, err error) error {
//		if f == nil {
//			return err
//		}
//		if f.IsDir() {
//			return nil
//		}
//		fmt.Printf(path)
//		if !strings.Contains(f.Name(), strDate) {
//			rmErr := os.Remove(logRoot + f.Name())
//			if rmErr != nil {
//				fmt.Println(logRoot + f.Name() + "can not Remove " + rmErr.Error())
//			}
//		}
//		return nil
//	})
//	if err != nil {
//		fmt.Println("can not walk folder " + err.Error())
//	}
//}

//func getPath(name string) string {
//	year, month, date := GetNow().Date()
//	strDate := strconv.Itoa(year) + month.String() + strconv.Itoa(date)
//	strTime := strconv.Itoa(GetNow().Hour()) + "_" + strconv.Itoa(GetNow().Minute())
//	return logRoot + name + strDate + "_" + strTime + ".log"
//}

func SocketInfo(format string, a ...interface{}) {
	//if socketCount%10000 == 0 {
	//	if socketFile != nil {
	//		_ = socketFile.Close()
	//	}
	//	socket, socketFile, _ = initLog(getPath("socketInfo"))
	//}
	//socketCount++
	msg := `socket ` + fmt.Sprintf(format, a...)
	logChan <- msg
}

func Debug(format string, a ...interface{}) {
	if DoDebug {
		//if DebugCount == 0 {
		//	if debugFile != nil {
		//		_ = debugFile.Close()
		//	}
		//	debug, debugFile, _ = initLog(getPath(`debug`))
		//}
		//DebugCount++
		//if DebugCount > 500000 {
		//	DoDebug = false
		//}
		msg := `debug  ` + fmt.Sprintf(format, a...)
		logChan <- msg
	}
}
func InfoSync(msg string) {
	//if infoCount%10000 == 0 {
	//	if infoFile != nil {
	//		_ = infoFile.Close()
	//	}
	//	info, infoFile, _ = initLog(getPath("info"))
	//}
	//info.Println(msg)
	logContent := &cmn.GlcData{Text: msg, Date: GetNow().String(), System: "infoSync"}
	cmn.Info(logContent)
}

func Info(format string, a ...interface{}) {
	//if infoCount%10000 == 0 {
	//	if infoFile != nil {
	//		_ = infoFile.Close()
	//	}
	//	info, infoFile, _ = initLog(getPath("info"))
	//}
	//infoCount++
	msg := `info   ` + fmt.Sprintf(format, a...)
	logChan <- msg
}

func NoticeLess(format string, a ...interface{}) {
	if time.Now().Second() == 0 {
		Notice(format, a...)
	}
}

func Notice(format string, a ...interface{}) {
	//if noticeCount%10000 == 0 {
	//	if noticeFile != nil {
	//		_ = noticeFile.Close()
	//	}
	//	notice, noticeFile, _ = initLog(getPath("notice"))
	//}
	//noticeCount++
	msg := `notice ` + fmt.Sprintf(format, a...)
	logChan <- msg
}
