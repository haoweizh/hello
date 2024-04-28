package controller

import (
	"encoding/json"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
	"hello/model"
	"net/http"
	"time"
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
		c.JSON(http.StatusBadRequest, map[string]interface{}{`status`: `fail`, `msg`: `require login`, `data`: map[string]interface{}{}})
		return
	}
	var settingMonitors []*model.SettingMonitor
	model.AppDB.Where("mail_address = ?", user.(string)).Find(&settingMonitors)
	c.JSON(http.StatusOK, map[string]interface{}{`status`: `ok`, `msg`: `success`, `data`: map[string]interface{}{`monitors`: settingMonitors}})
}

func addSettingMonitor(c *gin.Context) {
	session := sessions.Default(c)
	user := session.Get(`user`)
	if user == nil || user == `` {
		c.JSON(http.StatusBadRequest, map[string]interface{}{`status`: `fail`, `msg`: `require login`, `data`: map[string]interface{}{}})
		return
	}
	settingMonitor := model.SettingMonitor{}
	data := c.PostForm(`data`)
	err := json.Unmarshal([]byte(data), &settingMonitor)
	if err != nil {
		c.JSON(http.StatusBadRequest, map[string]interface{}{`status`: `fail`, `msg`: `wrong json format to unmarshal setting monitor`, `data`: map[string]interface{}{}})
		return
	}
	settingMonitor.MailAddress = user.(string)
	model.AppDB.Save(&settingMonitor)
	var settingMonitors []*model.SettingMonitor
	model.AppDB.Where("mail_address = ? and market = ? and symbol = ?",
		user.(string), settingMonitor.Market, settingMonitor.Symbol).Find(&settingMonitors)
	if len(settingMonitors) > 0 {
		c.JSON(http.StatusOK, map[string]interface{}{`status`: `ok`, `msg`: `success`, `data`: settingMonitors[0]})
	} else {
		c.JSON(http.StatusOK, map[string]interface{}{`status`: `fail`, `msg`: `fail to insert`, `data`: map[string]interface{}{}})
	}
}

func removeSettingMonitor(c *gin.Context) {
	session := sessions.Default(c)
	user := session.Get(`user`)
	if user == nil || user == `` {
		c.JSON(http.StatusBadRequest, map[string]interface{}{`status`: `fail`, `msg`: `require login`, `data`: map[string]interface{}{}})
		return
	}
	id, getId := c.GetPostForm(`id`)
	if !getId {
		c.JSON(http.StatusBadRequest, map[string]interface{}{`status`: `fail`, `msg`: `require id`, `data`: map[string]interface{}{}})
		return
	}
	rowNum := model.AppDB.Delete(&model.SettingMonitor{}, id).RowsAffected
	c.JSON(http.StatusOK, map[string]interface{}{`status`: `ok`, `msg`: `success`, `data`: map[string]interface{}{`NUM`: rowNum}})
}

func MonitorTrade(c *gin.Context) {
	wsHandler := func(client *model.WSAgent, event []byte) {
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
	//c.Get()
	value := c.Query(`id`)
	session := sessions.Default(c)
	sessionValue := session.Get(`user`)
	//sessionValue = `haoweizh@qq.com`
	//value := `2`
	settingMonitor := &model.SettingMonitor{}
	model.AppDB.Where("id = ?", value).First(settingMonitor)
	if settingMonitor == nil {
		c.JSON(http.StatusBadRequest, map[string]interface{}{`status`: `fail`, `msg`: `id match nil monitor setting`, `data`: map[string]interface{}{}})
		return
	}
	if sessionValue == nil || sessionValue.(string) != settingMonitor.MailAddress {
		c.JSON(http.StatusBadRequest, map[string]interface{}{`status`: `fail`, `msg`: `require login`, `data`: map[string]interface{}{}})
		return
	}
	conn, err := (&websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true }}).Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		http.NotFound(c.Writer, c.Request)
		return
	}
	wsAgent := &model.WSAgent{
		ID:             uuid.NewV4().String(),
		Socket:         conn,
		ChanRead:       make(chan []byte),
		Pinged:         true,
		Manager:        model.AppEnvironment.WsManager,
		SettingMonitor: settingMonitor,
		AggregateCandle: &model.AggregateCandle{
			TimeInterval: time.Duration(settingMonitor.IntervalSeconds) * time.Second, SlideRing: &model.SlideRing{}},
	}
	model.AppEnvironment.WsManager.AddAgent(wsAgent)
	go wsAgent.ReadServe(wsHandler)
}
