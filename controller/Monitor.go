package controller

import (
	"encoding/json"
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
	"hello/api"
	"hello/model"
	"hello/util"
	"net/http"
	"strconv"
	"strings"
)

func monitorEntry(c *gin.Context) {
	//data := make(map[string]string)
	//data[`status`] = `状态`
	c.HTML(http.StatusOK, `monitor.gohtml`, nil)
}

func InitFullMonitors(c *gin.Context) {
	session := sessions.Default(c)
	user := session.Get(`user`)
	if user == nil || user != `haoweizh@qq.com` {
		c.JSON(http.StatusBadRequest, map[string]interface{}{`status`: `fail`, `msg`: `require login`, `data`: map[string]interface{}{}})
		return
	}
	str := c.Query(`addresses`)
	strSeconds := c.Query(`seconds`)
	seconds, err := strconv.Atoi(strSeconds)
	if err != nil {
		c.JSON(http.StatusOK, map[string]interface{}{`status`: `fail`, `msg`: `wrong seconds ` + err.Error(), `data`: map[string]interface{}{}})
		return
	}
	api.InitMarketInfos(model.BinanceSpot)
	addresses := strings.Split(str, `,`)
	model.MarketInfos.Range(func(key, value any) bool {
		for _, address := range addresses {
			monitor := &model.SettingMonitor{
				MailAddress: address, Market: model.BinanceSpot,
				Symbol:          value.(*model.MarketInfo).Symbol,
				IntervalSeconds: seconds,
				WarnChange:      0.02,
				WarnIncrease:    0.01,
				WarnVolume:      200000,
				Volume24:        2000000}
			model.AppDB.Save(monitor)
		}
		return true
	})
	c.JSON(http.StatusOK, map[string]interface{}{`status`: `ok`, `msg`: `success`, `data`: map[string]interface{}{}})
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
	marshal, err := json.Marshal(settingMonitors)
	if err != nil {
		return
	} else {
		util.Log(util.LogLevelInfo, fmt.Sprintf(`get setting monitors: %s`, string(marshal)))
	}
	c.JSON(http.StatusOK, map[string]interface{}{`status`: `ok`, `msg`: `success`, `data`: map[string]interface{}{`monitors`: settingMonitors}})
}

func addSettingMonitor(c *gin.Context) {
	session := sessions.Default(c)
	user := session.Get(`user`)
	if user == nil || user == `` {
		c.JSON(http.StatusBadRequest, map[string]interface{}{`status`: `fail`, `msg`: `require login`, `data`: map[string]interface{}{}})
		return
	}
	value := make(map[string]string)
	data := c.PostForm(`data`)
	err := json.Unmarshal([]byte(data), &value)
	if err != nil {
		c.JSON(http.StatusBadRequest, map[string]interface{}{`status`: `fail`, `msg`: `wrong json format to unmarshal setting monitor`, `data`: map[string]interface{}{}})
		return
	}
	warnChange, _ := strconv.ParseFloat(value[`WarnChange`], 64)
	warnIncrease, _ := strconv.ParseFloat(value[`WarnIncrease`], 64)
	warnVolume, _ := strconv.ParseFloat(value[`WarnVolume`], 64)
	volume24, _ := strconv.ParseFloat(value[`Volume24`], 64)
	intervalSeconds, _ := strconv.Atoi(value[`IntervalSeconds`])
	value[`Market`] = model.BinanceSpot
	value[`Symbol`] = strings.ToUpper(strings.Trim(value[`Symbol`], ` `)) + `_USDT`
	if value[`Symbol`] != `_USDT` {
		marketInfo, _ := util.LoadSyncMap(model.MarketInfos, value[`Market`], value[`Symbol`])
		if marketInfo == nil {
			marketInit := false
			model.MarketInfos.Range(func(key, marketInfo any) bool {
				if key != nil && key.(string)[0:len(value[`Market`])] == value[`Market`] {
					marketInit = true
					return false
				}
				return true
			})
			if !marketInit {
				api.InitMarketInfos(value[`Market`])
				marketInfo, _ = util.LoadSyncMap(model.MarketInfos, value[`Market`], value[`Symbol`])
			}
			if marketInfo == nil {
				c.JSON(http.StatusOK, map[string]interface{}{`status`: `fail`, `msg`: `wrong market or symbol`, `data`: map[string]interface{}{}})
				return
			}
		}
	}
	settingMonitor := model.SettingMonitor{
		MailAddress:     user.(string),
		Market:          value[`Market`],
		Symbol:          value[`Symbol`],
		IntervalSeconds: intervalSeconds,
		WarnChange:      warnChange,
		WarnIncrease:    warnIncrease,
		WarnVolume:      warnVolume,
		Volume24:        volume24,
	}
	settingMonitor.MailAddress = user.(string)
	if value[`Symbol`] == `_USDT` {
		affected := model.AppDB.Model(&settingMonitor).Where("mail_address=? and market=? and interval_seconds=?",
			user.(string), settingMonitor.Market, settingMonitor.IntervalSeconds).Updates(map[string]interface{}{
			`volume24`: volume24, `warn_change`: warnChange, `warn_increase`: warnIncrease, `warn_volume`: warnVolume}).RowsAffected
		if affected > 0 {
			c.JSON(http.StatusOK, map[string]interface{}{`status`: `ok`, `msg`: `success update`, `data`: settingMonitor})
		} else {
			c.JSON(http.StatusOK, map[string]interface{}{`status`: `fail`, `msg`: `fail to insert or update`, `data`: map[string]interface{}{}})
		}
	} else if model.AppDB.Save(&settingMonitor).RowsAffected > 0 {
		util.Log(util.LogLevelInfo, fmt.Sprintf(
			`add setting monitor: %s %s %s`, settingMonitor.Market, settingMonitor.Symbol, settingMonitor.MailAddress))
		c.JSON(http.StatusOK, map[string]interface{}{`status`: `ok`, `msg`: `success insert`, `data`: settingMonitor})
	} else {
		affected := model.AppDB.Model(&settingMonitor).Where("mail_address=? and market=? and symbol=? and interval_seconds=?",
			user.(string), settingMonitor.Market, settingMonitor.Symbol, intervalSeconds).Updates(map[string]interface{}{
			`volume24`: volume24, `warn_change`: warnChange, `warn_increase`: warnIncrease, `warn_volume`: warnVolume}).RowsAffected
		if affected > 0 {
			c.JSON(http.StatusOK, map[string]interface{}{`status`: `ok`, `msg`: `success update`, `data`: settingMonitor})
		} else {
			c.JSON(http.StatusOK, map[string]interface{}{`status`: `fail`, `msg`: `fail to insert or update`, `data`: map[string]interface{}{}})
		}
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
	rowNum := model.AppDB.Where("id = ?", id).Delete(&model.SettingMonitor{}).RowsAffected
	util.Log(util.LogLevelInfo, fmt.Sprintf(`delete %s return %d`, id, rowNum))
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
	value := c.Query(`address`)
	conn, err := (&websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true }}).Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		http.NotFound(c.Writer, c.Request)
		return
	}
	wsAgent := &model.WSAgent{
		ID:        uuid.NewV4().String(),
		Socket:    conn,
		ChanRead:  make(chan []byte),
		ChanWrite: make(chan []byte),
		Pinged:    true,
		Manager:   model.AppEnvironment.WsManager,
		Address:   strings.ToLower(strings.Trim(value, ` `)),
	}
	model.AppEnvironment.WsManager.AddAgent(wsAgent)
	go wsAgent.ReadServe(wsHandler)
	go wsAgent.WriteServe()
}
