package controller

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/satori/go.uuid"
	"hello/api"
	"hello/carry"
	"hello/carry/cross"
	"hello/model"
	"hello/util"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var codeGenTime int64
var code = ``

func ParameterServe() {
	router := gin.Default()
	router.LoadHTMLGlob("templates/*")
	router.GET("/", crossPage)
	router.GET(`refresh`, RefreshParameters)
	router.GET(`pw`, GetCode)
	router.GET(`cross`, crossPage)
	router.GET(`hold`, holdPage)
	router.GET(`tick`, tickPage)
	router.GET(`cross_refresh`, crossRefresh)
	router.GET(`test`, testSpeed)
	router.GET(`debug`, debug)
	router.GET(`wss`, WsPage)
	var err error
	if model.AppConfig.Port == `443` {
		err = router.RunTLS(":"+model.AppConfig.Port, `./server.pem`, `./server.key`)
	} else {
		err = router.Run(":" + model.AppConfig.Port)
	}
	if err != nil {
		fmt.Println(`port occupied, exit ` + err.Error())
		os.Exit(1)
	}
}

func WsPage(c *gin.Context) {
	wsHandler := func(client *api.WSClient, event []byte) {
		fmt.Println(`receive from ws ` + string(event))
		//Manager.Broadcast <- jsonMessage
		client.Manager.Send(event, nil)
	}
	conn, err := (&websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true }}).Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		http.NotFound(c.Writer, c.Request)
		return
	}
	wsClient := &api.WSClient{
		ID:        uuid.NewV4().String(),
		Socket:    conn,
		ChanRead:  make(chan []byte),
		ChanWrite: make(chan []byte),
		Pinged:    true,
		Timer:     time.NewTimer(3 * time.Second),
		Manager:   &api.AppWSManager}
	wsClient.Manager.Register <- wsClient
	go wsClient.Read(wsHandler)
	go wsClient.Write()
}

func debug(c *gin.Context) {
	doDebug := c.Query(`count`)
	if doDebug != `0` {
		util.DoDebug = true
		util.DebugCount = 0
	} else {
		util.DoDebug = false
	}
	c.String(http.StatusOK, fmt.Sprintf(`set do debug 0-false, !0-true %s`, doDebug))
}

func testSpeed(c *gin.Context) {
	param := c.Query(`markets`)
	markets := strings.Split(param, `,`)
	low := make(map[string]int64)
	high := make(map[string]int64)
	avg := make(map[string]int64)
	for _, market := range markets {
		for i := 0; i < 50; i++ {
			before := util.GetNowUnixMillion()
			api.GetMarketInfos(market)
			duration := util.GetNowUnixMillion() - before
			if low[market] == 0 || low[market] > duration {
				low[market] = duration
			}
			if high[market] < duration {
				high[market] = duration
			}
			avg[market] += duration
			time.Sleep(time.Millisecond * 200)
			util.Info(fmt.Sprintf(`test break 200 ms %s %d`, market, duration))
		}
		util.Info(fmt.Sprintf(`%s %d %d %d`, market, low[market], high[market], avg[market]))
	}
}

func holdPage(c *gin.Context) {
	indexStr := c.Query(`index`)
	if len(indexStr) == 0 {
		indexStr = `0`
	}
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		index = 0
	}
	queryAccounts := model.GetAccounts(index)
	marketValues := make([][]string, 0)
	inAll := []float64{0, 0, 0, 0, 0, 0}
	for _, account := range queryAccounts {
		if account != nil {
			market, inAllSpot, contractAccountValue, holdingSpot, holdingFuture, unrealizedPnl := cross.GetCrossMarketValue(account.Key)
			marketValues = append(marketValues, []string{market,
				strconv.FormatFloat(inAllSpot, 'f', 0, 64),
				strconv.FormatFloat(contractAccountValue, 'f', 0, 64),
				strconv.FormatFloat(holdingSpot, 'f', 0, 64),
				strconv.FormatFloat(holdingFuture, 'f', 0, 64),
				strconv.FormatFloat(unrealizedPnl, 'f', 0, 64)})
			inAll[0] += inAllSpot
			if market != model.Ftx && market != model.OKEX {
				inAll[0] += contractAccountValue
			}
			inAll[1] += inAllSpot
			inAll[2] += contractAccountValue
			inAll[3] += holdingSpot
			inAll[4] += holdingFuture
			inAll[5] += unrealizedPnl
		}
	}
	marketValues = append(marketValues, []string{strconv.FormatFloat(inAll[0], 'f', 0, 64),
		strconv.FormatFloat(inAll[1], 'f', 0, 64), strconv.FormatFloat(inAll[2], 'f', 0, 64),
		strconv.FormatFloat(inAll[3], 'f', 0, 64), strconv.FormatFloat(inAll[4], 'f', 0, 64),
		strconv.FormatFloat(inAll[5], 'f', 0, 64)})
	tradeInfo := make([][]string, 0)
	duration, _ := time.ParseDuration(`-96h`)
	timeBegin := time.Now().Add(duration)
	timeBegin = time.Date(timeBegin.Year(), timeBegin.Month(), timeBegin.Day(), 0, 0, 0, 0, timeBegin.Location())
	failRows, _ := model.AppDB.Model(model.Order{}).Select(`market,amount_type,order_side,date(order_time),refresh_type,count(*)`).
		Where(`status=?`, `fail`).Group(`market,order_side,date(order_time),amount_type,refresh_type`).
		Order(`date(order_time) desc`).Rows()
	failData := make(map[string]float64) // market - amount_type - side - date - fail count
	if failRows != nil {
		for failRows.Next() {
			var marketName, side, date, amountType, refreshType string
			var orderNum float64
			_ = failRows.Scan(&marketName, &amountType, &side, &date, &refreshType, &orderNum)
			dates := strings.Split(date, `-`)
			date = fmt.Sprintf(`%s-%s`, dates[1], dates[2])
			date = date[0:strings.Index(date, `T`)]
			key := fmt.Sprintf(`%s-%s-%s-%s-%s`, marketName, amountType, side, date, refreshType)
			for _, account := range queryAccounts {
				if account != nil && account.Key == amountType {
					failData[key] = orderNum
				}
			}
		}
	}
	carryRows, _ := model.AppDB.Model(model.Order{}).Select(`market,amount_type,order_side,sum(price*abs(amount)),date(order_time),refresh_type,count(*)`).
		Group(`market,order_side,date(order_time),amount_type,refresh_type`).Order(`date(order_time) desc`).Rows()
	if carryRows != nil {
		for carryRows.Next() {
			var marketName, side, date, amountType, refreshType string
			var value, orderNum, failRate float64
			_ = carryRows.Scan(&marketName, &amountType, &side, &value, &date, &refreshType, &orderNum)
			dates := strings.Split(date, `-`)
			date = fmt.Sprintf(`%s-%s`, dates[1], dates[2])
			date = date[0:strings.Index(date, `T`)]
			key := fmt.Sprintf(`%s-%s-%s-%s-%s`, marketName, amountType, side, date, refreshType)
			account := model.AppConfig.GetAccountFromKey(marketName, amountType)
			if account != nil && model.AppConfig.GetAccounts(marketName)[index].Key == account.Key {
				if orderNum > 0 {
					failRate = 100 * failData[key] / orderNum
				}
				tradeInfo = append(tradeInfo, []string{marketName, date, side,
					strconv.FormatFloat(value, 'f', 0, 64), refreshType,
					strconv.FormatFloat(orderNum, 'f', 0, 64),
					strconv.FormatFloat(failRate, 'f', 2, 64)})
			}
		}
		carryRows.Close()
	}
	c.HTML(http.StatusOK, `hold.gohtml`, gin.H{
		`marketValue`: marketValues, `trade`: tradeInfo, `holdings`: cross.GetHoldings(queryAccounts)})
}

func crossRefresh(c *gin.Context) {
	api.InitCrossMarketInfos()
	c.String(http.StatusOK, `init cross markets done`)
}

func tickPage(c *gin.Context) {
	tickInfo, recentTickInfo := model.AppMetric.ToArray()
	c.HTML(http.StatusOK, `tick.gohtml`, gin.H{`tickInfo`: tickInfo, `recentTickInfo`: recentTickInfo})
}

func crossPage(c *gin.Context) {
	indexStr := c.Query(`index`)
	if len(indexStr) == 0 {
		indexStr = `0`
	}
	crossInfo := model.GetMonitorInfo(indexStr, `cross`)
	c.HTML(http.StatusOK, `balance.gohtml`, gin.H{`cross`: crossInfo})
}

func GetCode(c *gin.Context) {
	waitTime := (util.GetNowUnixMillion() - codeGenTime) / 1000
	if waitTime < 30 {
		waitTime = 30 - waitTime
		c.String(http.StatusOK, fmt.Sprintf(`还要等待 %d 秒才能再次发送`, waitTime))
	} else {
		codeGenTime = util.GetNowUnixMillion()
		rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
		rnd = rand.New(rand.NewSource(rnd.Int63()))
		code = fmt.Sprintf("%06v", rnd.Int31n(1000000))
		//ip, _ := util.ExternalIP()
		//verifyUrl := fmt.Sprintf(`http://%s:8080/set?pw=%s`, ip, code)
		err := util.SendMail(model.AppConfig.FromMail, model.AppConfig.FromMailAuth, model.AppConfig.Mail,
			`启动验证码`, `验证码是 `+code)
		if err == nil {
			c.String(http.StatusOK, `发送成功，请查收邮箱`)
		} else {
			c.String(http.StatusOK, `邮件发送失败`+err.Error())
		}
	}
}

//func GetParameters(c *gin.Context) {
//	msg := ``
//	markets := model.GetMarkets()
//	userKeys := make([]string, 0)
//	for _, market := range markets {
//		account := model.AppConfig.GetAccounts(market)[0]
//		userKeys = append(userKeys, account.Key)
//		marketInfos := model.GetMarketInfos(market)
//		if marketInfos != nil && model.GetSettings(model.FunctionCarry, market) != nil {
//			symbols := model.GetMarketSymbols(market)
//			for symbol := range marketInfos {
//				coin := model.GetCoin(market, symbol)
//				symbolPerp := coin + model.GetPerpTail(market)
//				symbolRelated := coin + model.GetSpotTail(market)
//				if (marketInfos[symbolPerp] != nil && marketInfos[symbolRelated] != nil) && (symbols[symbolPerp] == false || symbols[symbolRelated] == false) && symbol == symbolPerp {
//					msg += fmt.Sprintf("新币 %s %s\n", market, symbolPerp)
//				}
//			}
//		}
//		settingMap := model.GetSettings(model.FunctionTurtle, market)
//		msg += fmt.Sprintf("海龟币种：%s \n", market)
//		for symbol, setting := range settingMap {
//			if setting == nil {
//				continue
//			}
//			msgTail := ``
//			if setting.OpenShortMargin == 0 {
//				msgTail = `(去除平仓中)`
//			}
//			msg += fmt.Sprintf("%s%s, ", symbol, msgTail)
//		}
//		msg += "\n"
//	}
//	setting := model.GetSetting(model.FunctionGrid, model.Ftx, `LINK-PERP`)
//	if setting != nil {
//		msg += fmt.Sprintf("%s %s %s %f\n", setting.Function, setting.Market, setting.Symbol, setting.GridAmount)
//	}
//	for _, setting := range model.AppSettings {
//		if setting.Valid == false {
//			msg += fmt.Sprintf("[pause carry] %s %s %s at %s \n",
//				setting.Function, setting.Market, setting.Symbol, setting.UpdatedAt.String())
//		}
//	}
//	var orders model.Order
//	turtleRows, _ := model.AppDB.Model(&orders).Select(`market,symbol,order_side,price,deal_price,deal_amount`).
//		Where(`deal_amount>? and refresh_type=?`, 0, model.FunctionTurtle).
//		Order(`order_time desc`).Limit(10).Rows()
//	if turtleRows != nil {
//		for turtleRows.Next() {
//			var market, symbol, orderSide, price, dealPrice, dealAmount string
//			_ = turtleRows.Scan(&market, &symbol, &orderSide, &price, &dealPrice, &dealAmount)
//			msg += fmt.Sprintf("[turtle订单]%s %s %s 下单价格:%s 成交价格:%s 成交数量:%s\n",
//				market, symbol, orderSide, price, dealPrice, dealAmount)
//		}
//		turtleRows.Close()
//	}
//	duration, _ := time.ParseDuration(`-96h`)
//	timeBegin := time.Now().Add(duration)
//	timeBegin = time.Date(timeBegin.Year(), timeBegin.Month(), timeBegin.Day(), 0, 0, 0, 0, timeBegin.Location())
//	//model.AppDB.Model(&orders).Delete(`order_time<? and refresh_type=？`, timeBegin.String()[0:10], `carry`)
//	//model.AppDB.Model(&orders).Delete(`order_time<? and refresh_type=？`, timeBegin.String()[0:10],  `comp`)
//	failRows, _ := model.AppDB.Model(&orders).Select(`market,amount_type,order_side,date(order_time),refresh_type,count(*)`).
//		Where(`status=?`, `fail`).Group(`market,order_side,date(order_time),amount_type,refresh_type`).
//		Order(`date(order_time) desc`).Rows()
//	failData := make(map[string]float64) // market - amount_type - side - date - fail count
//	if failRows != nil {
//		for failRows.Next() {
//			var marketName, side, date, amountType, refreshType string
//			var orderNum float64
//			_ = failRows.Scan(&marketName, &amountType, &side, &date, &refreshType, &orderNum)
//			key := fmt.Sprintf(`%s-%s-%s-%s-%s`, marketName, amountType, side, date, refreshType)
//			if (marketName == model.OKEX && amountType == model.AppConfig.GetAccounts(model.OKEX)[0].Key) ||
//				(marketName == model.Binance && amountType == model.AppConfig.GetAccounts(model.Binance)[0].Key) ||
//				(marketName == model.Kucoin && amountType == model.AppConfig.GetAccounts(model.Kucoin)[0].Key) {
//				failData[key] = orderNum
//			}
//		}
//	}
//	failRows, _ = model.AppDB.Model(&orders).Select(`market,amount_type,order_side,date(order_time-interval '8 hour'),refresh_type,count(*)`).
//		Where(`status=?`, `fail`).Group(`market,order_side,date(order_time-interval '8 hour'),amount_type,refresh_type`).
//		Order(`date(order_time-interval '8 hour') desc`).Rows()
//	if failRows != nil {
//		for failRows.Next() {
//			var marketName, side, date, amountType, refreshType string
//			var orderNum float64
//			_ = failRows.Scan(&marketName, &amountType, &side, &date, &refreshType, &orderNum)
//			key := fmt.Sprintf(`%s-%s-%s-%s-%s`, marketName, amountType, side, date, refreshType)
//			if (marketName == model.Ftx && amountType == model.AppConfig.GetAccounts(model.Ftx)[0].Key) ||
//				(marketName == model.Gate && amountType == model.AppConfig.GetAccounts(model.Gate)[0].Key) {
//				failData[key] = orderNum
//			}
//		}
//	}
//	carryRows, _ := model.AppDB.Model(&orders).Select(`market,amount_type,order_side,sum(price*abs(amount)),date(order_time),refresh_type,count(*)`).
//		Group(`market,order_side,date(order_time),amount_type,refresh_type`).Order(`date(order_time) desc`).Rows()
//	carryFrontMsg := ``
//	if carryRows != nil {
//		for carryRows.Next() {
//			var marketName, side, date, amountType, refreshType string
//			var value, orderNum, failRate float64
//			_ = carryRows.Scan(&marketName, &amountType, &side, &value, &date, &refreshType, &orderNum)
//			key := fmt.Sprintf(`%s-%s-%s-%s-%s`, marketName, amountType, side, date, refreshType)
//			if (refreshType != model.FunctionTurtle && refreshType != model.FunctionGrid) &&
//				((marketName == model.OKEX && amountType == model.AppConfig.GetAccounts(model.OKEX)[0].Key) ||
//					(marketName == model.Binance && amountType == model.AppConfig.GetAccounts(model.Binance)[0].Key) ||
//					(marketName == model.Kucoin && amountType == model.AppConfig.GetAccounts(model.Kucoin)[0].Key)) {
//				if orderNum > 0 {
//					failRate = failData[key] / orderNum
//				}
//				carryFrontMsg += fmt.Sprintf("%s交易额 in USD: %s %s %f 类型：%s 单数:%f 失败率: %f\n",
//					marketName, date, side, value, refreshType, orderNum, failRate)
//			}
//		}
//		carryRows.Close()
//	}
//	carryRows, _ = model.AppDB.Model(&orders).Select(`market,amount_type,order_side,sum(price*abs(amount)),date(order_time-interval '8 hour'),refresh_type,count(*)`).
//		Group(`market,order_side,date(order_time-interval '8 hour'),amount_type,refresh_type`).Order(`date(order_time-interval '8 hour') desc`).Rows()
//	if carryRows != nil {
//		for carryRows.Next() {
//			var marketName, side, date, amountType, refreshType string
//			var value, orderNum, failRate float64
//			_ = carryRows.Scan(&marketName, &amountType, &side, &value, &date, &refreshType, &orderNum)
//			key := fmt.Sprintf(`%s-%s-%s-%s-%s`, marketName, amountType, side, date, refreshType)
//			if (marketName == model.Ftx && amountType == model.AppConfig.GetAccounts(model.Ftx)[0].Key) ||
//				(marketName == model.Gate && amountType == model.AppConfig.GetAccounts(model.Gate)[0].Key) {
//				if orderNum > 0 {
//					failRate = failData[key] / orderNum
//				}
//				carryFrontMsg += fmt.Sprintf("%s交易额 in USD: %s %s %f 类型：%s 单数:%f 失败率: %f\n",
//					marketName, date, side, value, refreshType, orderNum, failRate)
//			}
//		}
//		carryRows.Close()
//	}
//	msg += carryFrontMsg
//	for _, userKey := range userKeys {
//		msg += model.GetCarryInfo(userKey, ``) + "\n"
//	}
//	msg += model.AppMetric.ToString() + "\n"
//	msg += model.AppConfig.ToString()
//	c.String(http.StatusOK, msg)
//}

func RefreshParameters(c *gin.Context) {
	util.Notice(`controller refreshing`)
	model.LoadSettings()
	for _, market := range model.GetMarkets() {
		channels := model.AppMarkets.GetDepthChan(market)
		carry.ResetChannels(market, channels)
	}
	api.InitMarketInfos()
	c.String(http.StatusOK, model.AppConfig.ToString())
}
