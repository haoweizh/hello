package controller

import (
	"encoding/json"
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
	"hello/carry/monitor"
	"hello/model"
	"net/http"
)

func monitorEntry(c *gin.Context) {
	//data := make(map[string]string)
	//data[`status`] = `状态`
	c.HTML(http.StatusOK, `monitor.gohtml`, nil)
}

func getSettingMonitors(c *gin.Context) {
	session := sessions.Default(c)
	user := session.Get(`user`)
	if user == nil || user == `` {
		c.String(http.StatusNotAcceptable, `require login`)
		return
	}
	var settingMonitors []*monitor.SettingMonitor
	model.AppDB.Where("mail_address = ?", user.(string)).Find(&settingMonitors)
	c.JSON(http.StatusOK, settingMonitors)
}

func addSettingMonitor(c *gin.Context) {
	session := sessions.Default(c)
	user := session.Get(`user`)
	if user == nil || user == `` {
		c.String(http.StatusNotAcceptable, `require login`)
		return
	}
	settingMonitor := monitor.SettingMonitor{}
	data := c.PostForm(`data`)
	err := json.Unmarshal([]byte(data), &settingMonitor)
	if err != nil {
		c.JSON(http.StatusBadRequest, `wrong json format to unmarshal setting monitor`)
		return
	}
	settingMonitor.MailAddress = user.(string)
	model.AppDB.Save(&settingMonitor)
	c.String(http.StatusOK, fmt.Sprintf(`rows add %s`, data))
}

func removeSettingMonitor(c *gin.Context) {
	session := sessions.Default(c)
	user := session.Get(`user`)
	if user == nil || user == `` {
		c.String(http.StatusNotAcceptable, `require login`)
		return
	}
	id := c.Query(`id`)
	rowNum := model.AppDB.Delete(&monitor.SettingMonitor{}, id).RowsAffected
	c.JSON(http.StatusOK, rowNum)
}

func MonitorTrade(c *gin.Context) {
	wsHandler := func(client *monitor.WSAgent, event []byte) {
		//received := string(event)
		//fmt.Println(`receive from ws ` + received)
		//Manager.Broadcast <- jsonMessage
		//if strings.Contains(received, `refresh`) && model.StatusChanged {
		//	content := model.StatusInfo()
		//	client.Manager.Send(content, nil)
		//	//model.StatusChanged = false
		//} else if strings.Contains(received, `run`) {
		//	api.ChanCmd <- `run`
		//}
	}
	value := c.Query(`id`)
	session := sessions.Default(c)
	sessionValue := session.Get(`user`)
	settingMonitor := &monitor.SettingMonitor{}
	model.AppDB.Where("id = ?", value).First(settingMonitor)
	if settingMonitor == nil {
		c.String(http.StatusNotAcceptable, `id match nil monitor setting`)
		return
	}
	if sessionValue == nil || sessionValue.(string) != settingMonitor.MailAddress {
		c.String(http.StatusNotAcceptable, `require login`)
		return
	}
	conn, err := (&websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true }}).Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		http.NotFound(c.Writer, c.Request)
		return
	}
	wsAgent := &monitor.WSAgent{
		ID:             uuid.NewV4().String(),
		Socket:         conn,
		ChanRead:       make(chan []byte),
		Pinged:         true,
		Manager:        &monitor.AppWSManager,
		SettingMonitor: settingMonitor,
	}
	wsAgent.Manager.Register <- wsAgent
	go wsAgent.ReadServe(wsHandler)
}
