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
		c.JSON(http.StatusBadRequest, map[string]interface{}{`status`: `fail`, `msg`: `require login`, `data`: "{}"})
		return
	}
	var settingMonitors []*monitor.SettingMonitor
	model.AppDB.Where("mail_address = ?", user.(string)).Find(&settingMonitors)
	c.JSON(http.StatusOK, map[string]interface{}{`status`: `ok`, `msg`: `success`, `data`: settingMonitors})
}

func addSettingMonitor(c *gin.Context) {
	session := sessions.Default(c)
	user := session.Get(`user`)
	if user == nil || user == `` {
		c.JSON(http.StatusBadRequest, map[string]interface{}{`status`: `fail`, `msg`: `require login`, `data`: "{}"})
		return
	}
	settingMonitor := monitor.SettingMonitor{}
	data := c.PostForm(`data`)
	err := json.Unmarshal([]byte(data), &settingMonitor)
	if err != nil {
		c.JSON(http.StatusBadRequest, map[string]interface{}{`status`: `fail`, `msg`: `wrong json format to unmarshal setting monitor`, `data`: "{}"})
		return
	}
	settingMonitor.MailAddress = user.(string)
	model.AppDB.Save(&settingMonitor)
	c.JSON(http.StatusOK, map[string]interface{}{`status`: `ok`, `msg`: `success`, `data`: "{}"})
}

func removeSettingMonitor(c *gin.Context) {
	session := sessions.Default(c)
	user := session.Get(`user`)
	if user == nil || user == `` {
		c.JSON(http.StatusBadRequest, map[string]interface{}{`status`: `fail`, `msg`: `require login`, `data`: "{}"})
		return
	}
	id, getId := c.GetPostForm(`id`)
	if !getId {
		c.JSON(http.StatusBadRequest, map[string]interface{}{`status`: `fail`, `msg`: `require id`, `data`: "{}"})
		return
	}
	rowNum := model.AppDB.Delete(&monitor.SettingMonitor{}, id).RowsAffected
	c.JSON(http.StatusOK, map[string]interface{}{`status`: `ok`, `msg`: `success`, `data`: fmt.Sprintf("{NUM:%d}", rowNum)})
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
	value, getId := c.GetPostForm(`id`)
	if !getId {
		c.JSON(http.StatusBadRequest, map[string]interface{}{`status`: `fail`, `msg`: `require id`, `data`: "{}"})
		return
	}
	session := sessions.Default(c)
	sessionValue := session.Get(`user`)
	settingMonitor := &monitor.SettingMonitor{}
	model.AppDB.Where("id = ?", value).First(settingMonitor)
	if settingMonitor == nil {
		c.JSON(http.StatusBadRequest, map[string]interface{}{`status`: `fail`, `msg`: `id match nil monitor setting`, `data`: "{}"})
		return
	}
	if sessionValue == nil || sessionValue.(string) != settingMonitor.MailAddress {
		c.JSON(http.StatusBadRequest, map[string]interface{}{`status`: `fail`, `msg`: `require login`, `data`: "{}"})
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
