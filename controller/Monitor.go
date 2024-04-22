package controller

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
	"hello/api"
	"net/http"
	"time"
)

func monitorEntry(c *gin.Context) {
	//data := make(map[string]string)
	//data[`status`] = `状态`
	c.HTML(http.StatusOK, `monitor.gohtml`, nil)
}

func MonitorTrade(c *gin.Context) {
	wsHandler := func(client *api.WSAgent, event []byte) {
		received := string(event)
		fmt.Println(`receive from ws ` + received)
		//Manager.Broadcast <- jsonMessage
		//if strings.Contains(received, `refresh`) && model.StatusChanged {
		//	content := model.StatusInfo()
		//	client.Manager.Send(content, nil)
		//	//model.StatusChanged = false
		//} else if strings.Contains(received, `run`) {
		//	api.ChanCmd <- `run`
		//}
	}
	conn, err := (&websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true }}).Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		http.NotFound(c.Writer, c.Request)
		return
	}
	wsAgent := &api.WSAgent{
		ID:        uuid.NewV4().String(),
		Socket:    conn,
		ChanRead:  make(chan []byte),
		ChanWrite: make(chan []byte),
		Pinged:    true,
		Timer:     time.NewTimer(3 * time.Second),
		Manager:   &api.AppWSManager}
	wsAgent.Manager.Register <- wsAgent
	go wsAgent.Read(wsHandler)
	go wsAgent.Write()
}
