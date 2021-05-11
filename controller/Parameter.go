package controller

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/satori/go.uuid"
	"hello/api"
	"hello/carry"
	"hello/model"
	"hello/util"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

//var accessTime = make(map[string]int64)
var codeGenTime int64
var code = ``

func ParameterServe() {
	router := gin.Default()
	router.LoadHTMLGlob("templates/*")
	router.GET("/", GetParameters)
	router.GET("set", SetParameters)
	router.GET(`refresh`, RefreshParameters)
	router.GET(`pw`, GetCode)
	router.GET("balance", GetBalance)
	router.GET(`symbol`, setSymbol)
	router.GET(`test`, test)
	router.GET(`wss`, WsPage)
	router.GET(`api/master`, GetCarryInfo)
	router.GET(`api/slave`, GetCarryInfoSlave)
	if model.AppConfig.Port == `443` {
		_ = router.RunTLS(":"+model.AppConfig.Port, `./server.pem`, `./server.key`)
	} else {
		_ = router.Run(":" + model.AppConfig.Port)
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

func test(c *gin.Context) {
	carryRows, _ := model.AppDB.Model(&model.Order{}).Select(`market,amount_type,order_side,sum(price*amount),date(order_time),refresh_type`).
		Group(`market,order_side,date(order_time),amount_type,refresh_type`).Order(`date(order_time) desc`).Rows()
	carryBackMsg := ``
	keysFtx, _ := model.AppConfig.GetKeys(model.Ftx)
	keysOKEX, _ := model.AppConfig.GetKeys(model.OKEX)
	if carryRows != nil {
		for carryRows.Next() {
			var market, side, date, amountType, refreshType string
			var value float64
			_ = carryRows.Scan(&market, &amountType, &side, &value, &date, &refreshType)
			if !strings.Contains(amountType, keysFtx[0]) && !strings.Contains(amountType, keysOKEX[0]) {
				carryBackMsg += fmt.Sprintf("%s %s交易额 in USD: %s %s %f 类型：%s\n",
					market, amountType, date, side, value, refreshType)
			}
		}
		carryRows.Close()
	}
	carryBackMsg += model.GetCarryInfo(`dynamic`, ``)
	carryBackMsg += model.GetCarryInfo(`warning`, ``)
	c.String(http.StatusOK, carryBackMsg)
}

func setSymbol(c *gin.Context) {
	pw := c.Query(`pw`)
	if code == `` {
		c.String(http.StatusOK, `请先获取验证码`)
		return
	}
	if pw != code {
		c.String(http.StatusOK, `验证码错误`)
		return
	}
	waitTime := (util.GetNowUnixMillion() - codeGenTime) / 1000
	if waitTime > 300 {
		c.String(http.StatusOK, fmt.Sprintf(`验证码有效时间300秒，已超%d - %d > 300000`,
			util.GetNowUnixMillion(), codeGenTime))
		return
	}
	code = ``
	market := c.Query(`market`)
	symbol := c.Query(`symbol`)
	function := c.Query(`function`)
	strLimit := c.Query(`limit`)
	parameter := c.Query(`parameter`)
	binanceDisMin := c.Query(`binancedismin`)
	binanceDisMax := c.Query(`binancedismax`)
	refreshLimitLowStr := c.Query(`refreshlimitlow`)
	refreshLimitStr := c.Query(`refreshlimit`)
	chanceStr := c.Query(`chance`)
	refreshSameTime := c.Query(`refreshsametime`)
	gridAmountStr := c.Query(`gridamount`)
	gridDisStr := c.Query(`griddis`)
	priceXStr := c.Query(`pricex`)
	valid := false
	if market == `` || symbol == `` || function == `` {
		c.String(http.StatusOK, `market symbol function cannot be empty`)
	}
	op := c.Query(`op`)
	if op == `1` {
		valid = true
	} else if op == `0` {
		valid = false
	}
	var setting model.Setting
	amountLimit := 0.0
	if op != `` {
		model.AppDB.Model(&setting).Where("market= ? and symbol= ? and function= ?",
			market, symbol, function).Updates(map[string]interface{}{"valid": valid})
	}
	if parameter != `` {
		model.AppDB.Model(&setting).Where("market= ? and symbol= ? and function= ?",
			market, symbol, function).Updates(map[string]interface{}{`function_parameter`: parameter})
	}
	if priceXStr != `` {
		priceX, _ := strconv.ParseFloat(priceXStr, 64)
		model.AppDB.Model(&setting).Where("market= ? and symbol= ? and function= ?",
			market, symbol, function).Updates(map[string]interface{}{`price_x`: priceX})
	}
	if gridAmountStr != `` {
		gridAmount, _ := strconv.ParseFloat(gridAmountStr, 64)
		model.AppDB.Model(&setting).Where("market= ? and symbol= ? and function= ?",
			market, symbol, function).Updates(map[string]interface{}{`grid_amount`: gridAmount})
	}
	if gridDisStr != `` {
		gridPriceDistance, _ := strconv.ParseFloat(gridDisStr, 64)
		model.AppDB.Model(&setting).Where("market= ? and symbol= ? and function= ?",
			market, symbol, function).Updates(map[string]interface{}{`grid_price_distance`: gridPriceDistance})
	}
	if strLimit != `` {
		amountLimit, _ = strconv.ParseFloat(strLimit, 64)
		model.AppDB.Model(&setting).Where("market= ? and symbol= ? and function= ?",
			market, symbol, function).Updates(map[string]interface{}{`amount_limit`: amountLimit})
	}
	if binanceDisMin != `` {
		bDisMin, _ := strconv.ParseFloat(binanceDisMin, 64)
		model.AppDB.Model(&setting).Where("market= ? and symbol= ? and function= ?",
			market, symbol, function).Updates(map[string]interface{}{`binance_dis_min`: bDisMin})
	}
	if binanceDisMax != `` {
		bDisMax, _ := strconv.ParseFloat(binanceDisMax, 64)
		model.AppDB.Model(&setting).Where("market= ? and symbol= ? and function= ?",
			market, symbol, function).Updates(map[string]interface{}{`binance_dis_max`: bDisMax})
	}
	if refreshLimitLowStr != `` {
		refreshLimitLow, _ := strconv.ParseFloat(refreshLimitLowStr, 64)
		model.AppDB.Model(&setting).Where("market= ? and symbol= ? and function= ?",
			market, symbol, function).Updates(map[string]interface{}{`refresh_limit_low`: refreshLimitLow})
	}
	if refreshSameTime != `` {
		value, _ := strconv.ParseFloat(refreshSameTime, 64)
		model.AppDB.Model(&setting).Where("market= ? and symbol= ? and function= ?",
			market, symbol, function).UpdateColumn("refresh_same_time", value)
	}
	if refreshLimitStr != `` {
		refreshLimit, _ := strconv.ParseFloat(refreshLimitStr, 64)
		model.AppDB.Model(&setting).Where("market= ? and symbol= ? and function= ?",
			market, symbol, function).Updates(map[string]interface{}{`refresh_limit`: refreshLimit})
	}
	if chanceStr != `` {
		chance, _ := strconv.ParseFloat(chanceStr, 64)
		model.AppDB.Model(&setting).Where("market= ? and symbol= ? and function= ?",
			market, symbol, function).UpdateColumn("chance", chance)
	}
	rows, _ := model.AppDB.Model(&setting).
		Select(`market, symbol, function, function_parameter, amount_limit, refresh_same_time, valid`).Rows()
	defer rows.Close()
	msg := ``
	for rows.Next() {
		_ = rows.Scan(&market, &symbol, &function, &parameter, &amountLimit, &refreshSameTime, &valid)
		msg += fmt.Sprintf("%s %s %s %s %f %s\n", market, symbol, function, parameter, amountLimit,
			refreshSameTime)
	}
	model.LoadSettings()
	carry.MaintainMarketChan()
	c.String(http.StatusOK, msg)
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
		// iamp pjmyzgvrlifpcbci
	}
}

func GetBalance(c *gin.Context) {
	amount, transfer := api.GetWalletHistoryBitmex(``, ``)
	msg := fmt.Sprintf("[bitmex]\n%f \n%s\n", amount, transfer)
	amountBybit, msgBybit := api.GetWalletBybit(``, ``)
	msg = fmt.Sprintf("%s\n[bybit] %f \n%s", msg, amountBybit, msgBybit)
	c.String(http.StatusOK, msg)
}

func GetCarryInfoSlave(c *gin.Context) {
	info := model.GetCarryInfos(model.FunctionCarry + `_dynamic_`)
	tableDeal := make([]map[string]interface{}, 0)
	carryRows, _ := model.AppDB.Model(&model.Order{}).Select(`market,amount_type,order_side,sum(price*amount),date(order_time),refresh_type`).
		Group(`market,order_side,date(order_time),amount_type,refresh_type`).Order(`date(order_time) desc`).Rows()
	keys, _ := model.AppConfig.GetKeys(model.Ftx)
	if carryRows != nil {
		for carryRows.Next() {
			var market, side, date, amountType, refreshType string
			var value float64
			_ = carryRows.Scan(&market, &amountType, &side, &value, &date, &refreshType)
			if amountType != keys[0] {
				dealMsg := map[string]interface{}{`carry`: market, `交易额(usd)`: value, `date`: date, `side`: side,
					`type`: refreshType}
				tableDeal = append(tableDeal, dealMsg)
			}
		}
		carryRows.Close()
	}
	info = append(info, tableDeal)
	c.JSON(http.StatusOK, info)
}

func GetCarryInfo(c *gin.Context) {
	info := model.GetCarryInfos(model.FunctionCarry + `_dynamic_slave`)
	tableOrder := make([]map[string]interface{}, 0)
	tableDeal := make([]map[string]interface{}, 0)
	metricTables := model.AppMetric.ToTables()
	for _, table := range metricTables {
		info = append(info, table)
	}
	var orders model.Order
	turtleRows, _ := model.AppDB.Model(&orders).Select(`market,symbol,order_side,price,deal_price,deal_amount`).
		Where(`deal_amount>? and refresh_type=?`, 0, model.FunctionTurtle).
		Order(`order_time desc`).Limit(10).Rows()
	if turtleRows != nil {
		for turtleRows.Next() {
			var market, symbol, orderSide, price, dealPrice, dealAmount string
			_ = turtleRows.Scan(&market, &symbol, &orderSide, &price, &dealPrice, &dealAmount)
			turtleMsg := map[string]interface{}{`订单`: `turtle`, `market`: market, `symbol`: symbol, `side`: orderSide,
				`price`: price, `deal price`: dealPrice, `deal amount`: dealAmount}
			tableOrder = append(tableOrder, turtleMsg)
		}
		turtleRows.Close()
	}
	keys, _ := model.AppConfig.GetKeys(model.Ftx)
	carryRows, _ := model.AppDB.Model(&orders).Select(`market,amount_type,order_side,sum(price*amount),date(order_time),refresh_type`).
		Group(`market,order_side,date(order_time),amount_type,refresh_type`).Order(`date(order_time) desc`).Rows()
	if carryRows != nil {
		for carryRows.Next() {
			var market, side, date, amountType, refreshType string
			var value float64
			_ = carryRows.Scan(&market, &amountType, &side, &value, &date, &refreshType)
			if amountType == keys[0] {
				dealMsg := map[string]interface{}{`市场`: market, `交易额(usd)`: math.Round(value), `日期`: date[0:10],
					`side`: side, `type`: refreshType}
				tableDeal = append(tableDeal, dealMsg)
			}
		}
		carryRows.Close()
	}
	info = append(info, tableDeal)
	info = append(info, tableOrder)
	c.JSON(http.StatusOK, info)
}

func GetParameters(c *gin.Context) {
	msg := ``
	markets := model.GetMarkets()
	for _, market := range markets {
		marketInfos := model.GetMarketInfos(market)
		if marketInfos != nil && model.GetSettings(model.FunctionCarry, market) != nil {
			symbols := model.GetMarketSymbols(market)
			for symbol := range marketInfos {
				coin := model.GetCoin(market, symbol)
				symbolPerp := coin + api.GetPerpTail(market)
				symbolRelated := coin + api.GetSpotTail(market)
				if (marketInfos[symbolPerp] != nil && marketInfos[symbolRelated] != nil) && (symbols[symbolPerp] == false || symbols[symbolRelated] == false) && symbol == symbolPerp {
					msg += fmt.Sprintf("新币 %s %s\n", market, symbolPerp)
				}
			}
		}
	}
	settings := model.GetSetting(model.FunctionGrid, model.Ftx, `LINK-PERP`)
	for _, setting := range settings {
		msg += fmt.Sprintf("%s %s %s %f\n", setting.Function, setting.Market, setting.Symbol, setting.GridAmount)
	}
	var orders model.Order
	turtleRows, _ := model.AppDB.Model(&orders).Select(`market,symbol,order_side,price,deal_price,deal_amount`).
		Where(`deal_amount>? and refresh_type=?`, 0, model.FunctionTurtle).
		Order(`order_time desc`).Limit(10).Rows()
	if turtleRows != nil {
		for turtleRows.Next() {
			var market, symbol, orderSide, price, dealPrice, dealAmount string
			_ = turtleRows.Scan(&market, &symbol, &orderSide, &price, &dealPrice, &dealAmount)
			msg += fmt.Sprintf("[turtle订单]%s %s %s 下单价格:%s 成交价格:%s 成交数量:%s\n",
				market, symbol, orderSide, price, dealPrice, dealAmount)
		}
		turtleRows.Close()
	}
	keyFtx, _ := model.AppConfig.GetKeys(model.Ftx)
	keyOKEX, _ := model.AppConfig.GetKeys(model.OKEX)
	duration, _ := time.ParseDuration(`-96h`)
	timeBegin := time.Now().Add(duration)
	timeBegin = time.Date(timeBegin.Year(), timeBegin.Month(), timeBegin.Day(), 0, 0, 0, 0, timeBegin.Location())
	model.AppDB.Model(&orders).Delete(`order_time<? and refresh_type like ？`, timeBegin.String()[0:10], `carry%`)
	carryRows, _ := model.AppDB.Model(&orders).Select(`market,amount_type,order_side,sum(price*amount),date(order_time),refresh_type`).
		Group(`market,order_side,date(order_time),amount_type,refresh_type`).Order(`date(order_time) desc`).Rows()
	carryFrontMsg := ``
	carryBackMsg := ``
	if carryRows != nil {
		for carryRows.Next() {
			var marketName, side, date, amountType, refreshType string
			var value float64
			_ = carryRows.Scan(&marketName, &amountType, &side, &value, &date, &refreshType)
			if (marketName == model.Ftx && strings.Contains(amountType, keyFtx[0])) || (marketName == model.OKEX && strings.Contains(amountType, keyOKEX[0])) {
				carryFrontMsg += fmt.Sprintf("%s交易额 in USD: %s %s %f 类型：%s\n",
					marketName, date, side, value, refreshType)
			}
		}
		carryRows.Close()
	}
	msg += carryFrontMsg
	msg += model.GetCarryInfo(``, `slave`)
	msg += model.AppMetric.ToString() + "\n"
	msg += model.AppConfig.ToString()
	msg += carryBackMsg
	c.String(http.StatusOK, msg)
}

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

func SetParameters(c *gin.Context) {
	handle := c.Query("handle")
	openShortMargin := c.Query(`open`)
	disStr := c.Query(`dis`)
	var setting model.Setting
	if len(handle) > 0 {
		pw := c.Query(`pw`)
		if code == `` {
			c.String(http.StatusOK, `请先获取验证码`)
			return
		}
		if pw != code {
			c.String(http.StatusOK, `验证码错误`)
			return
		}
		waitTime := (util.GetNowUnixMillion() - codeGenTime) / 1000
		if waitTime > 300 {
			c.String(http.StatusOK, fmt.Sprintf(`验证码有效时间300秒，已超%d - %d > 300000`,
				util.GetNowUnixMillion(), codeGenTime))
			return
		}
		code = ``
	}
	if handle != `` {
		model.AppConfig.Handle = handle
	}
	if disStr != `` {
		gridPriceDistance, _ := strconv.ParseFloat(disStr, 64)
		model.AppDB.Model(&setting).Where("market= ? and function= ?",
			model.Ftx, model.FunctionCarry).Updates(map[string]interface{}{`grid_price_distance`: gridPriceDistance})
	}
	if openShortMargin != `` {
		openValue, err := strconv.ParseFloat(openShortMargin, 64)
		if err == nil {
			openValue = math.Abs(openValue)
			closeValue := -1 * openValue
			model.AppDB.Model(&setting).Where(`market=? and function=? and close_short_margin>-1`,
				model.Ftx, model.FunctionCarry).Updates(map[string]interface{}{
				`open_short_margin`: openValue, `close_short_margin`: closeValue})
			model.AppDB.Model(&setting).Where(`market=? and function=? and close_short_margin=-1`,
				model.Ftx, model.FunctionCarry).Updates(map[string]interface{}{`open_short_margin`: openValue})

		}
	}
	amountRate := c.Query("amountrate")
	if amountRate != `` {
		model.AppConfig.AmountRate, _ = strconv.ParseFloat(amountRate, 64)
	}
	channelSlot := c.Query("channelslot")
	if len(strings.TrimSpace(channelSlot)) > 0 {
		value, _ := strconv.ParseFloat(channelSlot, 64)
		if value > 0 {
			model.AppConfig.ChannelSlot = value
		}
	}
	delay := c.Query("delay")
	if len(strings.TrimSpace(delay)) > 0 {
		strDelay := strings.Replace(delay, " ", "", -1)
		model.AppConfig.Delay, _ = strconv.ParseFloat(strDelay, 64)
	}
	model.LoadSettings()
	carry.MaintainMarketChan()
	c.String(http.StatusOK, model.AppConfig.ToString())
}
