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
	"math"
	"math/rand"
	"net/http"
	"os"
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
	router.GET(`cross`, crossPage)
	router.GET(`tick`, tickPage)
	router.GET(`test`, test)
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

func test(c *gin.Context) {
	carryRows, _ := model.AppDB.Model(&model.Order{}).Select(`market,amount_type,order_side,sum(price*amount),date(order_time),refresh_type`).
		Group(`market,order_side,date(order_time),amount_type,refresh_type`).Order(`date(order_time) desc`).Rows()
	carryBackMsg := "account info:\n"
	markets := model.GetMarkets()
	userKeys := make([]string, 0)
	for _, market := range markets {
		accounts := model.AppConfig.GetAccounts(market)
		for i, account := range accounts {
			if i > 0 {
				userKeys = append(userKeys, account.Key)
			}
			failNum := carry.GetCarryResult(account.Key)
			collateral := carry.GetCollateral(account.Key)
			if collateral != nil {
				carryBackMsg += fmt.Sprintf("fails %s 可用保证金: %f 占用保证金: %f 保证金率: rate: %f\n",
					account.Key, collateral.Available, collateral.Occupied, collateral.Rate)
			}
			carryBackMsg += fmt.Sprintf("current fails: %s %d\n", account.Key, failNum)
		}
	}
	if carryRows != nil {
		for carryRows.Next() {
			var market, side, date, amountType, refreshType string
			var value float64
			_ = carryRows.Scan(&market, &amountType, &side, &value, &date, &refreshType)
			if !strings.Contains(amountType, model.AppConfig.GetAccounts(model.Ftx)[0].Key) &&
				!strings.Contains(amountType, model.AppConfig.GetAccounts(model.OKEX)[0].Key) &&
				!strings.Contains(amountType, model.AppConfig.GetAccounts(model.Binance)[0].Key) &&
				!strings.Contains(amountType, model.AppConfig.GetAccounts(model.Huobi)[0].Key) {
				carryBackMsg += fmt.Sprintf("%s %s交易额 in USD: %s %s %f 类型：%s\n",
					market, amountType, date, side, value, refreshType)
			}
		}
		carryRows.Close()
	}
	for _, userKey := range userKeys {
		carryBackMsg += userKey + "\n" + model.GetCarryInfo(userKey, ``)
	}
	c.String(http.StatusOK, carryBackMsg)
}

func tickPage(c *gin.Context) {
	priceDis, tickInfo, recentTickInfo := model.AppMetric.ToArray()
	c.HTML(http.StatusOK, `tick.gohtml`, gin.H{
		`priceDis`: priceDis, `tickInfo`: tickInfo, `recentTickInfo`: recentTickInfo})
}

func crossPage(c *gin.Context) {
	indexStr := c.Query(`index`)
	if len(indexStr) == 0 {
		indexStr = `0`
	}
	index, err := strconv.ParseInt(indexStr, 10, 64)
	if err != nil {
		index = 0
	}
	var accountFtx, accountOkex, accountBinance, accountGate, accountBybit, accountHuobi, accountKucoin *model.Account
	tempAccounts := model.AppConfig.GetAccounts(model.Ftx)
	if len(tempAccounts) > int(index) {
		accountFtx = tempAccounts[index]
	}
	tempAccounts = model.AppConfig.GetAccounts(model.OKEX)
	if len(tempAccounts) > int(index) {
		accountOkex = tempAccounts[index]
	}
	tempAccounts = model.AppConfig.GetAccounts(model.Binance)
	if len(tempAccounts) > int(index) {
		accountBinance = tempAccounts[index]
	}
	tempAccounts = model.AppConfig.GetAccounts(model.Gate)
	if len(tempAccounts) > int(index) {
		accountGate = tempAccounts[index]
	}
	tempAccounts = model.AppConfig.GetAccounts(model.Bybit)
	if len(tempAccounts) > int(index) {
		accountBybit = tempAccounts[index]
	}
	tempAccounts = model.AppConfig.GetAccounts(model.Huobi)
	if len(tempAccounts) > int(index) {
		accountHuobi = tempAccounts[index]
	}
	tempAccounts = model.AppConfig.GetAccounts(model.Kucoin)
	if len(tempAccounts) > int(index) {
		accountKucoin = tempAccounts[index]
	}
	accounts := []*model.Account{accountFtx, accountOkex, accountBinance, accountGate, accountBybit, accountKucoin}
	marketValues := make([][]string, 0)
	inAll := []float64{0, 0, 0, 0, 0, 0}
	for _, account := range accounts {
		if account != nil {
			market, inAllSpot, collateral, holdingSpot, holdingFuture, unrealizedPnl := cross.GetCrossMarketValue(account.Key)
			marketValues = append(marketValues, []string{market,
				strconv.FormatFloat(inAllSpot, 'f', 0, 64),
				strconv.FormatFloat(collateral+unrealizedPnl, 'f', 0, 64),
				strconv.FormatFloat(collateral, 'f', 0, 64),
				strconv.FormatFloat(holdingSpot, 'f', 0, 64),
				strconv.FormatFloat(holdingFuture, 'f', 0, 64)})
			inAll[0] += inAllSpot
			if market != model.Ftx && market != model.OKEX {
				inAll[0] += collateral
			}
			inAll[1] += inAllSpot
			inAll[2] += collateral + unrealizedPnl
			inAll[3] += collateral
			inAll[4] += holdingSpot
			inAll[5] += holdingFuture
		}
	}
	marketValues = append(marketValues, []string{strconv.FormatFloat(inAll[0], 'f', 0, 64),
		strconv.FormatFloat(inAll[1], 'f', 0, 64), strconv.FormatFloat(inAll[1], 'f', 0, 64),
		strconv.FormatFloat(inAll[2], 'f', 0, 64), strconv.FormatFloat(inAll[3], 'f', 0, 64),
		strconv.FormatFloat(inAll[4], 'f', 0, 64), strconv.FormatFloat(inAll[5], 'f', 0, 64),
	})
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
			if (marketName == model.OKEX && amountType == accountOkex.Key) ||
				(marketName == model.Binance && amountType == accountBinance.Key) ||
				(marketName == model.Huobi && amountType == accountHuobi.Key) ||
				(marketName == model.Kucoin && amountType == accountKucoin.Key) {
				failData[key] = orderNum
			}
		}
	}
	failRows, _ = model.AppDB.Model(model.Order{}).Select(`market,amount_type,order_side,date(order_time-interval '8 hour'),refresh_type,count(*)`).
		Where(`status=?`, `fail`).Group(`market,order_side,date(order_time-interval '8 hour'),amount_type,refresh_type`).
		Order(`date(order_time-interval '8 hour') desc`).Rows()
	if failRows != nil {
		for failRows.Next() {
			var marketName, side, date, amountType, refreshType string
			var orderNum float64
			_ = failRows.Scan(&marketName, &amountType, &side, &date, &refreshType, &orderNum)
			dates := strings.Split(date, `-`)
			date = fmt.Sprintf(`%s-%s`, dates[1], dates[2])
			date = date[0:strings.Index(date, `T`)]
			key := fmt.Sprintf(`%s-%s-%s-%s-%s`, marketName, amountType, side, date, refreshType)
			if (marketName == model.Ftx && amountType == accountFtx.Key) ||
				(marketName == model.Gate && amountType == accountGate.Key) {
				failData[key] = orderNum
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
			if (marketName == model.OKEX && amountType == model.AppConfig.GetAccounts(model.OKEX)[0].Key) ||
				(marketName == model.Binance && amountType == model.AppConfig.GetAccounts(model.Binance)[0].Key) ||
				(marketName == model.Huobi && amountType == model.AppConfig.GetAccounts(model.Huobi)[0].Key) ||
				(marketName == model.Kucoin && amountType == model.AppConfig.GetAccounts(model.Kucoin)[0].Key) {
				if orderNum > 0 {
					failRate = failData[key] / orderNum
				}
				tradeInfo = append(tradeInfo, []string{marketName, date, side,
					strconv.FormatFloat(value, 'f', 0, 64), refreshType,
					strconv.FormatFloat(orderNum, 'f', 0, 64),
					strconv.FormatFloat(failRate, 'f', 2, 64)})
			}
		}
		carryRows.Close()
	}
	carryRows, _ = model.AppDB.Model(model.Order{}).Select(`market,amount_type,order_side,sum(price*abs(amount)),date(order_time-interval '8 hour'),refresh_type,count(*)`).
		Group(`market,order_side,date(order_time-interval '8 hour'),amount_type,refresh_type`).Order(`date(order_time-interval '8 hour') desc`).Rows()
	if carryRows != nil {
		for carryRows.Next() {
			var marketName, side, date, amountType, refreshType string
			var value, orderNum, failRate float64
			_ = carryRows.Scan(&marketName, &amountType, &side, &value, &date, &refreshType, &orderNum)
			dates := strings.Split(date, `-`)
			date = fmt.Sprintf(`%s-%s`, dates[1], dates[2])
			date = date[0:strings.Index(date, `T`)]
			key := fmt.Sprintf(`%s-%s-%s-%s-%s`, marketName, amountType, side, date, refreshType)
			if (marketName == model.Ftx && amountType == model.AppConfig.GetAccounts(model.Ftx)[0].Key) ||
				(marketName == model.Gate && amountType == model.AppConfig.GetAccounts(model.Gate)[0].Key) {
				if orderNum > 0 {
					failRate = failData[key] / orderNum
				}
				tradeInfo = append(tradeInfo, []string{marketName, date, side,
					strconv.FormatFloat(value, 'f', 0, 64), refreshType,
					strconv.FormatFloat(orderNum, 'f', 0, 64),
					strconv.FormatFloat(failRate, 'f', 2, 64)})
			}
		}
		carryRows.Close()
	}
	crossInfo := model.GetMonitorInfo(indexStr, `cross`)
	c.HTML(http.StatusOK, `balance.gohtml`, gin.H{`marketValue`: marketValues, `trade`: tradeInfo, `cross`: crossInfo})
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

func GetParameters(c *gin.Context) {
	msg := ``
	markets := model.GetMarkets()
	userKeys := make([]string, 0)
	for _, market := range markets {
		account := model.AppConfig.GetAccounts(market)[0]
		userKeys = append(userKeys, account.Key)
		marketInfos := model.GetMarketInfos(market)
		if marketInfos != nil && model.GetSettings(model.FunctionCarry, market) != nil {
			symbols := model.GetMarketSymbols(market)
			for symbol := range marketInfos {
				coin := model.GetCoin(market, symbol)
				symbolPerp := coin + model.GetPerpTail(market)
				symbolRelated := coin + model.GetSpotTail(market)
				if (marketInfos[symbolPerp] != nil && marketInfos[symbolRelated] != nil) && (symbols[symbolPerp] == false || symbols[symbolRelated] == false) && symbol == symbolPerp {
					msg += fmt.Sprintf("新币 %s %s\n", market, symbolPerp)
				}
			}
		}
		settingMap := model.GetSettings(model.FunctionTurtle, market)
		msg += fmt.Sprintf("海龟币种：%s \n", market)
		for symbol, setting := range settingMap {
			if setting == nil {
				continue
			}
			msgTail := ``
			if setting.OpenShortMargin == 0 {
				msgTail = `(去除平仓中)`
			}
			msg += fmt.Sprintf("%s%s, ", symbol, msgTail)
		}
		msg += "\n"
	}
	setting := model.GetSetting(model.FunctionGrid, model.Ftx, `LINK-PERP`)
	if setting != nil {
		msg += fmt.Sprintf("%s %s %s %f\n", setting.Function, setting.Market, setting.Symbol, setting.GridAmount)
	}
	for _, setting := range model.AppSettings {
		if setting.Valid == false {
			msg += fmt.Sprintf("[pause carry] %s %s %s at %s \n",
				setting.Function, setting.Market, setting.Symbol, setting.UpdatedAt.String())
		}
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
	duration, _ := time.ParseDuration(`-96h`)
	timeBegin := time.Now().Add(duration)
	timeBegin = time.Date(timeBegin.Year(), timeBegin.Month(), timeBegin.Day(), 0, 0, 0, 0, timeBegin.Location())
	//model.AppDB.Model(&orders).Delete(`order_time<? and refresh_type=？`, timeBegin.String()[0:10], `carry`)
	//model.AppDB.Model(&orders).Delete(`order_time<? and refresh_type=？`, timeBegin.String()[0:10],  `comp`)
	failRows, _ := model.AppDB.Model(&orders).Select(`market,amount_type,order_side,date(order_time),refresh_type,count(*)`).
		Where(`status=?`, `fail`).Group(`market,order_side,date(order_time),amount_type,refresh_type`).
		Order(`date(order_time) desc`).Rows()
	failData := make(map[string]float64) // market - amount_type - side - date - fail count
	if failRows != nil {
		for failRows.Next() {
			var marketName, side, date, amountType, refreshType string
			var orderNum float64
			_ = failRows.Scan(&marketName, &amountType, &side, &date, &refreshType, &orderNum)
			key := fmt.Sprintf(`%s-%s-%s-%s-%s`, marketName, amountType, side, date, refreshType)
			if (marketName == model.OKEX && amountType == model.AppConfig.GetAccounts(model.OKEX)[0].Key) ||
				(marketName == model.Binance && amountType == model.AppConfig.GetAccounts(model.Binance)[0].Key) ||
				(marketName == model.Huobi && amountType == model.AppConfig.GetAccounts(model.Huobi)[0].Key) ||
				(marketName == model.Kucoin && amountType == model.AppConfig.GetAccounts(model.Kucoin)[0].Key) {
				failData[key] = orderNum
			}
		}
	}
	failRows, _ = model.AppDB.Model(&orders).Select(`market,amount_type,order_side,date(order_time-interval '8 hour'),refresh_type,count(*)`).
		Where(`status=?`, `fail`).Group(`market,order_side,date(order_time-interval '8 hour'),amount_type,refresh_type`).
		Order(`date(order_time-interval '8 hour') desc`).Rows()
	if failRows != nil {
		for failRows.Next() {
			var marketName, side, date, amountType, refreshType string
			var orderNum float64
			_ = failRows.Scan(&marketName, &amountType, &side, &date, &refreshType, &orderNum)
			key := fmt.Sprintf(`%s-%s-%s-%s-%s`, marketName, amountType, side, date, refreshType)
			if (marketName == model.Ftx && amountType == model.AppConfig.GetAccounts(model.Ftx)[0].Key) ||
				(marketName == model.Gate && amountType == model.AppConfig.GetAccounts(model.Gate)[0].Key) {
				failData[key] = orderNum
			}
		}
	}
	carryRows, _ := model.AppDB.Model(&orders).Select(`market,amount_type,order_side,sum(price*abs(amount)),date(order_time),refresh_type,count(*)`).
		Group(`market,order_side,date(order_time),amount_type,refresh_type`).Order(`date(order_time) desc`).Rows()
	carryFrontMsg := ``
	if carryRows != nil {
		for carryRows.Next() {
			var marketName, side, date, amountType, refreshType string
			var value, orderNum, failRate float64
			_ = carryRows.Scan(&marketName, &amountType, &side, &value, &date, &refreshType, &orderNum)
			key := fmt.Sprintf(`%s-%s-%s-%s-%s`, marketName, amountType, side, date, refreshType)
			if (marketName == model.OKEX && amountType == model.AppConfig.GetAccounts(model.OKEX)[0].Key) ||
				(marketName == model.Binance && amountType == model.AppConfig.GetAccounts(model.Binance)[0].Key) ||
				(marketName == model.Huobi && amountType == model.AppConfig.GetAccounts(model.Huobi)[0].Key) ||
				(marketName == model.Kucoin && amountType == model.AppConfig.GetAccounts(model.Kucoin)[0].Key) {
				if orderNum > 0 {
					failRate = failData[key] / orderNum
				}
				carryFrontMsg += fmt.Sprintf("%s交易额 in USD: %s %s %f 类型：%s 单数:%f 失败率: %f\n",
					marketName, date, side, value, refreshType, orderNum, failRate)
			}
		}
		carryRows.Close()
	}
	carryRows, _ = model.AppDB.Model(&orders).Select(`market,amount_type,order_side,sum(price*abs(amount)),date(order_time-interval '8 hour'),refresh_type,count(*)`).
		Group(`market,order_side,date(order_time-interval '8 hour'),amount_type,refresh_type`).Order(`date(order_time-interval '8 hour') desc`).Rows()
	if carryRows != nil {
		for carryRows.Next() {
			var marketName, side, date, amountType, refreshType string
			var value, orderNum, failRate float64
			_ = carryRows.Scan(&marketName, &amountType, &side, &value, &date, &refreshType, &orderNum)
			key := fmt.Sprintf(`%s-%s-%s-%s-%s`, marketName, amountType, side, date, refreshType)
			if (marketName == model.Ftx && amountType == model.AppConfig.GetAccounts(model.Ftx)[0].Key) ||
				(marketName == model.Gate && amountType == model.AppConfig.GetAccounts(model.Gate)[0].Key) {
				if orderNum > 0 {
					failRate = failData[key] / orderNum
				}
				carryFrontMsg += fmt.Sprintf("%s交易额 in USD: %s %s %f 类型：%s 单数:%f 失败率: %f\n",
					marketName, date, side, value, refreshType, orderNum, failRate)
			}
		}
		carryRows.Close()
	}
	msg += carryFrontMsg
	for _, userKey := range userKeys {
		msg += model.GetCarryInfo(userKey, ``) + "\n"
	}
	msg += model.AppMetric.ToString() + "\n"
	msg += model.AppConfig.ToString()
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
