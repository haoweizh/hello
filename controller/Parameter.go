package controller

import (
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/satori/go.uuid"
	"hello/api"
	"hello/carry/cross"
	"hello/model"
	"hello/regret"
	"hello/regret/Grid"
	"hello/util"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

var codeGenTime int64
var codes = make(map[string]bool)
var simulating = false
var candling = false

const RegretTurtleGridAmount = 1000

func ParameterServe() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	store := cookie.NewStore([]byte("secretKey&^%&^$&^%)*()9878687"))
	router.Use(sessions.Sessions("mysession", store))
	_ = router.SetTrustedProxies(nil)
	router.LoadHTMLGlob("templates/*")
	router.GET("/", GetParameters)
	//router.GET(`refresh`, RefreshParameters)
	//router.GET(`test`, testSpeed)
	router.GET(`pw`, GetCode)
	router.GET(`simulate`, simulate)
	router.GET(`simulateGrid`, simulateGrid)
	router.GET(`cross`, crossPage)
	router.GET(`hold`, holdPage)
	router.GET(`tick`, tickPage)
	router.GET(`cross_refresh`, crossRefresh)
	router.GET(`debug`, debug)
	router.GET(`wss`, WsPage)
	router.GET(`gxzq`, simulateGXZQ)
	router.GET(`candles`, getCandles)
	router.GET(`mine`, mindZeroAddr)
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

//func clearSpot(c *gin.Context) {
//	accounts := model.GetAccounts(0)
//	util.Notice(fmt.Sprintf(`get accounts %d`, len(accounts)))
//	for _, account := range accounts {
//		if account == nil || (account.Market != model.Gate && account.Market != model.BinanceSpot) {
//			continue
//		}
//		util.Notice(fmt.Sprintf(`start to clear account %s %s`, account.Market, account.Key))
//		_, balances, _, _ := api.GetBalances(account.Key, account.Secret, account.Market)
//		for _, balance := range balances {
//			symbol := strings.ToUpper(balance.Coin + `_USDT`)
//			_, tick := model.AppMarkets.GetBidAsk(symbol, account.Market)
//			if tick == nil || tick.Bids[0].Price*balance.Amount < 20 {
//				continue
//			}
//			order := api.PlaceOrder(account.Key, account.Secret, model.OrderSideSell, model.OrderTypeLimit, account.Market, symbol,
//				``, tick.Bids[0].Price*0.98, tick.Bids[0].Price*0.98, balance.Amount*0.98, false, nil, nil)
//			util.Notice(fmt.Sprintf(`sell %s amt %f at %f orderId %s`, order.Symbol, order.Amount, order.Price, order.OrderId))
//			time.Sleep(time.Second)
//		}
//	}
//	c.String(http.StatusOK, `done`)
//}

func setSimulating(value bool) {
	simulating = value
}

func setCandling(value bool) {
	candling = value
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

func autoSimulate(market, coins string, begin, end time.Time, strBegin, strEnd string, useNear, useM bool,
	near, far, limit, allLimit, seconds int, fee float64) {
	coinArray := strings.Split(coins, `,`)
	settings := make(map[string]*model.Setting)
	marketType := model.MarketTypePerp
	if market == model.GXZQ {
		marketType = model.MarketTypeFuture
	}
	for _, coin := range coinArray {
		symbol := coin + model.UniStandardTail[marketType]
		if useNear {
			settings[symbol] = &model.Setting{Market: market, Symbol: symbol, AmountLimit: float64(limit),
				GridAmount: RegretTurtleGridAmount, Seconds: int64(seconds), Near: int64(near), Far: int64(far), TradeCost: fee}
		} else { // 以useNear false测试龟汤，所以没有设置滑点即tradeCost=0
			settings[symbol] = &model.Setting{Market: market, Symbol: symbol, AmountLimit: float64(limit),
				GridAmount: RegretTurtleGridAmount, Seconds: int64(seconds), Near: int64(near), Far: int64(far), TradeCost: 0}
		}
	}
	sign := fmt.Sprintf(`market%s,coins%s,%s~%s,far%d,near%d,limit%d,allLimit%d,useNear%v,useM%vseconds%d`,
		market, coins, strBegin, strEnd, far, near, limit, allLimit, useNear, useM, seconds)
	delNum := model.AppDB.Where(`function=?`, sign).Delete(&model.Order{}).RowsAffected
	util.Info(`del %s %d rows affected`, sign, delNum)
	regret.ProcessCandles(begin, end, far, allLimit, useNear, useM, market, sign, settings)
	util.Info(`auto simulation done %s`, sign)
	util.StoreSyncMap(&model.CarryInfo, fmt.Sprintf("done %s %s 使用回撤%v %d~%d 限制%d 总限制%d",
		strBegin, strEnd, useNear, near, far, limit, allLimit), `auto`)
}

func simulateGXZQ(c *gin.Context) {
	session := sessions.Default(c)
	value := c.Query(`code`)
	if codes[value] {
		session.Set(`code`, value)
		_ = session.Save()
	}
	sessionValue := session.Get(`code`)
	if sessionValue == nil || !codes[sessionValue.(string)] {
		c.String(http.StatusForbidden, `no right`)
		return
	}
	market := c.Query(`market`)
	if market == `` {
		market = model.GXZQ
	}
	strBegin := `2021-01-01T00:00:00+00:00`
	strEnd := `2023-01-01T00:00:00+00:00`
	//coins := `CZCE.FG,DCE.jm,DCE.eb,CZCE.TA,SHFE.fu,DCE.p,CZCE.SF,SHFE.hc,DCE.v,DCE.y`
	coins := `DOGE,SOL,MATIC,CHZ,LINK,ADA,BNB,FIL,SUSHI,AXS,ATOM,WAVES`
	for i := 3; i <= 21; i++ {
		sign := fmt.Sprintf(`market%s,coins%s,seconds86400,%s~%s,far%d,near%d,limit%d,allLimit%d,useNear%v`,
			market, coins, strBegin, strEnd, i*2, i, 3, -1, true)
		regret.CutTail(market, coins, sign)
		util.Notice(`done cut tail %s %s %s`, market, coins, sign)
		sign = fmt.Sprintf(`market%s,coins%s,seconds86400,%s~%s,far%d,near%d,limit%d,allLimit%d,useNear%v`,
			market, coins, strBegin, strEnd, i*2, i, 3, -1, false)
		regret.CutTail(market, coins, sign)
		util.Notice(`done cut tail %s %s %s`, market, coins, sign)
	}
	util.Notice(`done cut tail all`)
	c.String(http.StatusOK, `done`)
}

func simulateGrid(c *gin.Context) {
	if model.AppConfig.Simulation != `on` {
		c.String(http.StatusOK, `do not support simulate`)
		return
	}
	if simulating {
		msg, ok := util.LoadSyncMap(&model.CarryInfo, `gridInfo`)
		if ok && msg != nil {
			c.String(http.StatusOK, "simulating...\n"+msg.(string))
		} else {
			c.String(http.StatusBadRequest, `simulating can not be started`)
		}
		util.StoreSyncMap(&model.CarryInfo, nil, `gridInfo`)
		return
	} else {
		defer setSimulating(false)
		setSimulating(true)
	}
	strNew := c.Query(`new`)
	market := c.Query(`market`)
	coins := c.Query(`coin`)
	strSeconds := c.Query(`seconds`)
	strFar := c.Query(`far`) // both, buy, sell
	function := c.Query(`function`)
	far, errFar := strconv.ParseInt(strFar, 10, 64)
	seconds, errSeconds := strconv.ParseInt(strSeconds, 10, 64)
	strBegin := c.Query(`begin`) + `T00:00:00+00:00`
	strEnd := c.Query(`end`) + `T00:00:00+00:00`
	begin, errBegin := time.Parse(time.RFC3339, strBegin)
	end, errEnd := time.Parse(time.RFC3339, strEnd)
	session := sessions.Default(c)
	value := c.Query(`code`)
	if codes[value] {
		session.Set(`code`, value)
		_ = session.Save()
	}
	sessionValue := session.Get(`code`)
	if sessionValue == nil || !codes[sessionValue.(string)] || errFar != nil || errSeconds != nil || errBegin != nil || errEnd != nil {
		strNew = `false`
	}
	coinArr := strings.Split(coins, `,`)
	function = fmt.Sprintf(`%s_%d_%d`, function, far, seconds)
	for _, coin := range coinArr {
		setting := &model.Setting{Valid: true,
			Market:   market,
			Symbol:   strings.ToUpper(coin) + model.UniStandardTail[model.MarketTypePerp],
			Coin:     coin,
			Function: function,
			Far:      far,
			Seconds:  seconds}
		if strNew == `true` {
			go model.AppDB.Where(`function=? and market=? and symbol=? and order_time>? and order_time<?`,
				setting.Function, market, setting.Symbol, strBegin, strEnd).Delete(&model.Order{})
			Grid.ProcessGrid(begin, end, setting)
		}
	}
	util.Notice(fmt.Sprintf(`done simulate grid`))
	c.String(http.StatusOK, `done`)
}

func mindZeroAddr(c *gin.Context) {
	session := sessions.Default(c)
	value := c.Query(`code`)
	if codes[value] {
		session.Set(`code`, value)
		_ = session.Save()
	}
	sessionValue := session.Get(`code`)
	if sessionValue == nil || !codes[sessionValue.(string)] {
		c.String(http.StatusUnauthorized, `not authorized`)
	} else {
		c.String(http.StatusOK, util.RunMindZeroAddr(6, 11, 4))
	}
}

// simulate
// limit:仓数上限，可选，默认为3
// new:true为生成新的仿真否则为查看同参数历史仿真
// coin:币种
// begin:开始时间，格式2022-10-01 end：结束时间
// type:以秒计算单周期长度，例如3600 14400
func simulate(c *gin.Context) {
	if model.AppConfig.Simulation != `on` {
		c.String(http.StatusOK, `do not support simulate`)
		return
	}
	if simulating {
		msg, ok := util.LoadSyncMap(&model.CarryInfo, `GetCandle`)
		autoMsg, _ := util.LoadSyncMap(&model.CarryInfo, `auto`)
		if ok && msg != nil {
			if autoMsg != nil {
				c.String(http.StatusOK, fmt.Sprintf("simulating...\nanto：%s %s", msg.(string), autoMsg.(string)))
			} else {
				c.String(http.StatusOK, "simulating...\n"+msg.(string))
			}
		} else {
			if autoMsg != nil {
				c.String(http.StatusOK, "simulating...auto"+autoMsg.(string))
			} else {
				c.String(http.StatusBadRequest, `simulating can not be started`)
			}
		}
		util.StoreSyncMap(&model.CarryInfo, nil, `GetCandle`)
		return
	} else {
		defer setSimulating(false)
		setSimulating(true)
	}
	auto := c.Query(`auto`)
	market := c.Query(`market`)
	if strings.Trim(market, ` `) == `` {
		market = model.GXZQ
	}
	useMStr := c.Query(`useM`)
	nearStr := c.Query(`near`)
	near, nearErr := strconv.ParseInt(nearStr, 10, 64)
	farStr := c.Query(`far`)
	far, farErr := strconv.ParseInt(farStr, 10, 64)
	coins := c.Query(`coin`)
	coinArray := strings.Split(coins, `,`)
	strBegin := c.Query(`begin`) + `T00:00:00+00:00`
	strEnd := c.Query(`end`) + `T00:00:00+00:00`
	strNew := c.Query(`new`)
	strLimit := c.Query(`limit`)
	strAllLimit := c.Query(`allLimit`)
	strUseNear := c.Query(`useNear`)
	strSeconds := c.Query(`seconds`)
	strFee := c.Query(`fee`)
	seconds, _ := strconv.ParseInt(strSeconds, 10, 64)
	fee, _ := strconv.ParseFloat(strFee, 64)
	useNear, useNearErr := strconv.ParseBool(strUseNear)
	begin, errBegin := time.Parse(time.RFC3339, strBegin)
	end, errEnd := time.Parse(time.RFC3339, strEnd)
	useM := false
	if useMStr == `true` {
		useM = true
	}
	sign := fmt.Sprintf(`market%s,coins%s,%s~%s,far%s,near%s,limit%s,allLimit%s,useNear%s,useM%v,seconds%s`,
		market, coins, strBegin, strEnd, farStr, nearStr, strLimit, strAllLimit, strUseNear, useM, strSeconds)
	util.Info(fmt.Sprintf(`get simulation parameter %s auto%s`, sign, auto))
	limit, limitErr := strconv.ParseInt(strLimit, 10, 64)
	if limitErr != nil {
		limit = 3
	}
	allLimit, allLimitErr := strconv.ParseInt(strAllLimit, 10, 64)
	if allLimitErr != nil {
		allLimit = limit
	}
	session := sessions.Default(c)
	value := c.Query(`code`)
	if codes[value] {
		session.Set(`code`, value)
		_ = session.Save()
	}
	sessionValue := session.Get(`code`)
	if sessionValue == nil || !codes[sessionValue.(string)] {
		strNew = `false`
	} else if auto == `true` && strNew == `true` {
		for i := 3; i <= 25; i++ {
			//autoSimulate(market, coins, begin, end, strBegin, strEnd, true, false, i, 2*i, 3, int(allLimit), int(seconds), fee)
			autoSimulate(market, coins, begin, end, strBegin, strEnd, true, false, i, 2*i, 3, int(allLimit), int(seconds), fee)
			autoSimulate(market, coins, begin, end, strBegin, strEnd, true, false, i, 2*i, 3, int(allLimit), int(seconds), fee)
			autoSimulate(market, coins, begin, end, strBegin, strEnd, true, false, i, 2*i, 3, int(allLimit), int(seconds), fee)
			//autoSimulate(market, coins, begin, end, strBegin, strEnd, false, true, i, 2*i, 3, int(allLimit), int(seconds), fee)
		}
		util.StoreSyncMap(&model.CarryInfo, nil, `auto`)
		util.Info(fmt.Sprintf(`all auto simulation done fee %f`, fee))
		c.String(http.StatusOK, fmt.Sprintf(`auto done fee %f`, fee))
		return
	}
	if errBegin != nil || errEnd != nil || nearErr != nil || farErr != nil || useNearErr != nil {
		simulateGuide := "limit:仓数上限，可选，默认为3 \nnew:true为生成新的仿真否则为查看同参数历史仿真\n" +
			"type:海龟的计算周期，默认86400秒，即一天，取值范围：3600、14400或86400的倍数\nmarket:模拟市场\n" +
			"near:海龟近计算周期数，far:海龟远计算周期数\n" +
			"useNear:是否采用近几天高低点作为平仓条件\n" +
			"参数样例：\ncoin=xrp&begin=2022-10-01&end=2022-10-10&limit=3&type=86400&market=okex&near=10&far=20new=true&useNear=false\n"
		c.String(http.StatusMethodNotAllowed,
			fmt.Sprintf("参数错误，请参考:\n%s", simulateGuide))
		return
	}
	duration, _ := time.ParseDuration(`100000h`)
	if begin.Add(duration).Before(end) {
		c.String(http.StatusMethodNotAllowed, fmt.Sprintf(`模拟时间跨度%s~%s大于3800天`, begin.String(), end.String()))
		return
	}
	msg := ``
	settings := make(map[string]*model.Setting)
	for i := 0; i < len(coinArray); i++ {
		tail := model.UniStandardTail[model.MarketTypePerp]
		if market == model.GXZQ {
			tail = model.UniStandardTail[model.MarketTypeFuture]
		}
		symbol := coinArray[i] + tail
		settings[symbol] = &model.Setting{Market: market, Symbol: symbol, AmountLimit: float64(limit),
			GridAmount: RegretTurtleGridAmount, Near: near, Far: far, Seconds: seconds, TradeCost: fee}
	}
	if strNew == `true` {
		go model.AppDB.Where(`function=?`, sign).Delete(&model.Order{})
		regret.ProcessCandles(begin, end, int(far), int(allLimit), useNear, useM, market, sign, settings)
	} else {
		util.Notice(`no need process simulate new %s`, strNew)
	}
	orders := make([]*model.Order, 0)
	model.AppDB.Where(`function=?`, sign).Order(`order_time asc`).Find(&orders)
	msg += fmt.Sprintf("Get %d orders %s %v %s %s from %d settings\n",
		len(orders), strLimit, strUseNear, begin.String(), end.String(), len(settings))
	msg += regret.ToString(orders, market, strLimit, RegretTurtleGridAmount, begin, end) + "\n"
	c.String(http.StatusOK, msg)
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

// testSpeed
//func _(c *gin.Context) {
//	param := c.Query(`markets`)
//	markets := strings.Split(param, `,`)
//	low := make(map[string]int64)
//	high := make(map[string]int64)
//	avg := make(map[string]int64)
//	util.Info(fmt.Sprintf(`begin to test %s %d`, markets, len(markets)))
//	for _, market := range markets {
//		for i := 0; i < 50; i++ {
//			before := util.GetNowUnixMillion()
//			api.GetMarketInfos(market)
//			duration := util.GetNowUnixMillion() - before
//			if low[market] == 0 || low[market] > duration {
//				low[market] = duration
//			}
//			if high[market] < duration {
//				high[market] = duration
//			}
//			avg[market] += duration
//			time.Sleep(time.Millisecond * 200)
//			util.Info(fmt.Sprintf(`test break 200 ms %s %d`, market, duration))
//		}
//		util.Info(fmt.Sprintf(`%s %d %d %d`, market, low[market], high[market], avg[market]))
//	}
//}

func getCandles(c *gin.Context) {
	if candling {
		c.String(http.StatusLocked, `candling`)
		return
	}
	defer setCandling(false)
	setCandling(true)
	session := sessions.Default(c)
	value := c.Query(`code`)
	if codes[value] {
		session.Set(`code`, value)
		_ = session.Save()
	}
	sessionValue := session.Get(`code`)
	if sessionValue == nil || !codes[sessionValue.(string)] {
		c.String(http.StatusUnauthorized, `wrong code`)
		return
	}
	start := time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2023, 4, 1, 0, 0, 0, 0, time.UTC)
	settings := map[string]*model.Setting{`ETH_USDT`: nil, `BTC_USDT`: nil}
	accounts := model.GetAccounts(0)
	if accounts != nil {
		api.GetMultiCandle(accounts[model.BinanceSpot], model.BinanceSpot, 60, start, end, settings, true)
	}
	c.String(http.StatusOK, `done`)
}

func holdPage(c *gin.Context) {
	//api.InitCrossMarketInfos(strings.Split(`gate`, `,`))
	indexStr := c.Query(`index`)
	if len(indexStr) == 0 {
		indexStr = `0`
	}
	force := c.Query(`force`)
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		index = 0
	}
	queryAccounts := model.GetAccounts(index)
	if queryAccounts == nil {
		return
	}
	marketValues := make([][]string, 0)
	inAll := []float64{0, 0, 0, 0, 0, 0}
	for _, account := range queryAccounts {
		if account != nil {
			inAllSpot, contractAccountValue, holdingSpot, holdingFuture, unrealizedPnl, keepInU :=
				cross.GetCrossMarketValue(account.Key, account.Secret, account.Market, force == `true`)
			duplicated := false
			for _, value := range marketValues {
				if (strings.Contains(value[0], `binance`) && strings.Contains(account.Market, `binance`)) ||
					(strings.Contains(value[0], `bitget`) && strings.Contains(account.Market, `bitget`)) {
					duplicated = true
					break
				}
			}
			if duplicated {
				continue
			}
			marketValues = append(marketValues, []string{account.Market,
				strconv.FormatFloat(inAllSpot, 'f', 0, 64),
				strconv.FormatFloat(contractAccountValue, 'f', 0, 64),
				strconv.FormatFloat(holdingSpot, 'f', 0, 64),
				strconv.FormatFloat(holdingFuture, 'f', 0, 64),
				strconv.FormatFloat(unrealizedPnl, 'f', 0, 64)})
			inAll[0] += inAllSpot - keepInU
			// 统一账户不算期货总价值
			if !account.IsUnified {
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
	failRows, _ := model.AppDB.Model(model.Order{}).Select(`market,account_index,order_side,date(order_time),refresh_type,count(*)`).
		Where(`status=?`, `fail`).Group(`market,order_side,date(order_time),account_index,refresh_type`).
		Order(`date(order_time) desc,market`).Rows()
	failData := make(map[string]float64) // market - account_index - side - date - fail count
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
	carryRows, _ := model.AppDB.Model(model.Order{}).Select(`account_index,order_side,sum(price*abs(amount)),date(order_time),count(*),refresh_type`).
		Where(`refresh_type!=?`, model.FunctionSimulation).
		Group(`order_side,date(order_time),account_index,refresh_type`).Order(`date(order_time) desc`).Rows()
	if carryRows != nil {
		crossInU := map[string]map[string]float64{}
		crossCount := map[string]map[string]float64{}
		for carryRows.Next() {
			var side, date, refreshType string
			var value, orderNum float64
			var amountType int
			_ = carryRows.Scan(&amountType, &side, &value, &date, &orderNum, &refreshType)
			dates := strings.Split(date, `-`)
			date = fmt.Sprintf(`%s-%s`, dates[1], dates[2])
			date = date[0:strings.Index(date, `T`)]
			if amountType == index {
				if crossInU[date] == nil || crossCount[date] == nil {
					crossInU[date] = map[string]float64{}
					crossCount[date] = map[string]float64{}
				}
				crossInU[date][refreshType+`_`+side] += value
				crossCount[date][refreshType+`_`+side] += orderNum
			}
		}
		for date, m := range crossInU {
			for key, crossU := range m {
				refreshType := key[0:strings.Index(key, `_`)]
				side := key[strings.Index(key, `_`)+1:]
				tradeInfo = append(tradeInfo, []string{`ALL`, date, refreshType,
					strconv.FormatFloat(crossU, 'f', 0, 64), side,
					strconv.FormatFloat(crossCount[date][key], 'f', 0, 64), ``})
			}
		}
		err := carryRows.Close()
		if err != nil {
			util.Notice(fmt.Sprintf(`fail to close DB for carry rows`))
			return
		}
	}
	carryRows, _ = model.AppDB.Model(model.Order{}).Select(`market,account_index,order_side,sum(price*abs(amount)),date(order_time),refresh_type,count(*)`).
		Where(`refresh_type!=?`, model.FunctionSimulation).Group(`market,order_side,date(order_time),account_index,refresh_type`).Order(`date(order_time) desc, market`).Rows()
	if carryRows != nil {
		for carryRows.Next() {
			var marketName, side, date, refreshType string
			var value, orderNum, failRate float64
			var accountIdx int
			_ = carryRows.Scan(&marketName, &accountIdx, &side, &value, &date, &refreshType, &orderNum)
			dates := strings.Split(date, `-`)
			date = fmt.Sprintf(`%s-%s`, dates[1], dates[2])
			date = date[0:strings.Index(date, `T`)]
			key := fmt.Sprintf(`%s-%d-%s-%s-%s`, marketName, accountIdx, side, date, refreshType)
			account := model.AppConfig.GetAccountFromKeyIndex(marketName, ``, accountIdx)
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
		err := carryRows.Close()
		if err != nil {
			util.Notice(fmt.Sprintf(`fail to close DB for carry rows`))
			return
		}
	}
	c.HTML(http.StatusOK, `hold.gohtml`, gin.H{
		`marketValue`: marketValues, `trade`: tradeInfo, `holdings`: cross.GetHoldings(queryAccounts)})
}

func crossRefresh(c *gin.Context) {
	//session := sessions.Default(c)
	//value := c.Query(`code`)
	//if codes[value] {
	//	session.Set(`code`, value)
	//	_ = session.Save()
	//}
	//sessionValue := session.Get(`code`)
	//if sessionValue == nil || !codes[sessionValue.(string)] {
	//	c.String(http.StatusOK, `no correct code`)
	//} else {
	//}
	param := c.Query(`markets`)
	api.InitCrossMarketInfos(strings.Split(param, `,`))
	api.PrepareSettings()
	markets := api.GetMarkets()
	for _, market := range markets {
		api.SetRequireReset(market)
	}
	c.String(http.StatusOK, `init cross markets done`)
}

func tickPage(c *gin.Context) {
	//tickInfo, recentTickInfo := model.AppMetric.ToArray()
	c.HTML(http.StatusOK, `tick.gohtml`, gin.H{`tickInfo`: model.AppMetric.ToArray()})
	//c.HTML(http.StatusOK, `tick.gohtml`, gin.H{`tickInfo`: model.AppMetric.ToString()})
}

func crossPage(c *gin.Context) {
	indexStr := c.Query(`index`)
	if len(indexStr) == 0 {
		indexStr = `0`
	}
	crossInfo := model.GetMonitorInfo(indexStr, model.FunctionCross)
	c.HTML(http.StatusOK, `balance.gohtml`, gin.H{`cross`: crossInfo})
}

func GetCode(c *gin.Context) {
	waitTime := (util.GetNowUnixMillion() - codeGenTime) / 1000
	indexStr := c.Query(`index`)
	if waitTime < 30 {
		waitTime = 30 - waitTime
		c.String(http.StatusOK, fmt.Sprintf(`还要等待 %d 秒才能再次发送`, waitTime))
	} else if len(indexStr) > 0 {
		index, _ := strconv.ParseInt(indexStr, 10, 64)
		accountFtx := model.AppConfig.GetAccounts(model.Ftx)[index]
		balances := api.GetTransfers(accountFtx.Key, accountFtx.Secret, model.Ftx)
		for _, balance := range balances {
			model.AppDB.Save(balance)
		}
		accountOk := model.AppConfig.GetAccounts(model.OKEX)[index]
		balances = api.GetTransfers(accountOk.Key, accountOk.Secret, model.OKEX)
		for _, balance := range balances {
			model.AppDB.Save(balance)
		}
	} else {
		codeGenTime = util.GetNowUnixMillion()
		rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
		rnd = rand.New(rand.NewSource(rnd.Int63()))
		code := fmt.Sprintf("%06v", rnd.Int31n(1000000))
		if len(codes) < 1000 {
			codes[code] = true
		} else {
			codes = make(map[string]bool)
		}
		//ip, _ := util.ExternalIP()
		//verifyUrl := fmt.Sprintf(`http://%s:8080/set?pw=%s`, ip, code)
		go api.SendMails(`启动验证码`, `验证码是 `+code)
		util.Notice(fmt.Sprintf(`code is %s`, code))
		c.String(http.StatusOK, `调用成功，请查收邮箱，如果没有，检查日志`)
	}
}

func createTurtleLines(function, market string, account *model.Account) (msg string, size int) {
	settingMap := api.GetSettings(function, market)
	lines := make([]*model.Sortable, 0)
	commonLines := make([]string, 0)
	if settingMap == nil {
		return
	}
	settingMap.Range(func(symbol, value any) bool {
		if value == nil {
			return true
		}
		setting := value.(*model.Setting)
		now := time.Now()
		_, nowStr := model.GetNowPeriod(market, setting.Seconds, now)
		turtleData, _ := util.LoadSyncMap(&api.TurtleDataSet, function, market, symbol.(string), nowStr)
		msgKey := model.GetMsgKey(function, market, symbol.(string))
		needAdd := false
		util.Info(fmt.Sprintf(`create lines %s %s %s`, function, market, symbol))
		if turtleData != nil {
			if turtleData.(*model.TurtleData).OrderLong != nil || turtleData.(*model.TurtleData).OrderShort != nil {
				needAdd = true
			} else if function == model.FunctionCombineTurtle {
				settingNormal := api.GetSetting(model.FunctionTurtleNormal, market, symbol.(string))
				_, nowStr = model.GetNowPeriod(market, settingNormal.Seconds, now)
				turtleNormal, _ := util.LoadSyncMap(&api.TurtleDataSet, model.FunctionTurtleNormal, market, symbol.(string), nowStr)
				if turtleNormal != nil && (turtleNormal.(*model.TurtleData).OrderLong != nil ||
					turtleNormal.(*model.TurtleData).OrderShort != nil) {
					needAdd = true
				}
			}
		}
		if needAdd {
			msgValue, _ := util.LoadSyncMap(&model.CarryInfo, account.Key, msgKey)
			if msgValue != nil {
				size++
				sortable := &model.Sortable{Key: symbol.(string), Value: msgValue.(string) + "\n"}
				_, _, coin, _ := model.GetFromStandard(market, symbol.(string))
				if model.CommonCoins[strings.ToLower(coin)] {
					commonLines = append(commonLines, msgValue.(string))
				} else {
					lines = append(lines, sortable)
				}
			}
		}
		return true
	})
	sortedLines := &model.SortableArray{Array: lines}
	sort.Sort(sortedLines)
	for _, line := range sortedLines.Array {
		msg += line.Value.(string)
	}
	msg += "主流币\n"
	for _, line := range commonLines {
		msg += line + "\n"
	}
	return
}

func GetParameters(c *gin.Context) {
	msg := api.GetMarketEquity(0)
	markets := api.GetMarkets()
	userKeys := make([]string, 0)
	for _, market := range markets {
		account := model.AppConfig.GetAccounts(market)[0]
		userKeys = append(userKeys, account.Key)
		msgTurtle, sizeTurtle := createTurtleLines(model.FunctionTurtle, market, account)
		msgCombine, sizeCombine := createTurtleLines(model.FunctionCombineTurtle, market, account)
		msg += fmt.Sprintf("单一海龟%s 个数%d\n %s\n", market, sizeTurtle, msgTurtle)
		msg += fmt.Sprintf("组合海龟%s 个数%d\n %s\n", market, sizeCombine, msgCombine)
	}
	//setting := api.GetSetting(model.FunctionGrid, model.Ftx, `LINK-PERP`)
	//if setting != nil {
	//	msg += fmt.Sprintf("%s %s %s %f\n", setting.Function, setting.Market, setting.Symbol, setting.GridAmount)
	//}
	var orders model.Order
	turtleRows, _ := model.AppDB.Model(&orders).Select(`order_update_time,market,symbol,order_side,price,deal_price,deal_amount`).
		Where(`order_update_time>? and deal_amount>? and refresh_type!=?`, time.Now().Add(time.Hour*-240), 0, model.FunctionCross).
		Order(`order_update_time desc`).Limit(100).Rows()
	if turtleRows != nil {
		for turtleRows.Next() {
			var updateTime time.Time
			var market, symbol, orderSide, price, dealPrice, dealAmount string
			_ = turtleRows.Scan(&updateTime, &market, &symbol, &orderSide, &price, &dealPrice, &dealAmount)
			msg += fmt.Sprintf("[成交订单]%s %s %s %s 下单价格:%s 成交价格:%s 成交数量:%s\n",
				updateTime.String(), market, symbol, orderSide, price, dealPrice, dealAmount)
		}
		err := turtleRows.Close()
		if err != nil {
			return
		}
	}
	msg += model.AppMetric.ToString()
	c.String(http.StatusOK, msg)
}

//func RefreshParameters(c *gin.Context) {
//	util.Notice(`controller refreshing`)
//	api.InitApp()
//	for _, market := range api.GetMarkets() {
//		channels, _ := model.AppMarkets.WsDepth.Load(market)
//		if channels != nil {
//			carry.ResetChannels(market, channels.([]chan struct{}))
//		}
//	}
//	api.InitMarketInfos()
//	c.String(http.StatusOK, model.AppConfig.ToString())
//}
