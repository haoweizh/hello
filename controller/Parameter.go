package controller

import (
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/satori/go.uuid"
	"hello/api"
	"hello/carry"
	"hello/carry/cross"
	"hello/model"
	"hello/regret"
	"hello/util"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var codeGenTime int64
var codes = make(map[string]bool)
var simulating = false

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
	router.GET(`pw`, GetCode)
	router.GET(`simulate`, simulate)
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

func setSimulating(value bool) {
	simulating = value
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

func autoSimulate(market, coins string, begin, end time.Time, strBegin, strEnd string, useNear bool, near, far, limit, allLimit int) {
	coinArray := strings.Split(coins, `,`)
	settings := make(map[string]*model.Setting)
	for _, coin := range coinArray {
		symbol := strings.ToUpper(coin) + model.UniStandardTail[model.MarketTypePerp]
		settings[symbol] = &model.Setting{Market: market, Symbol: symbol, AmountLimit: float64(limit), GridAmount: RegretTurtleGridAmount}
	}
	sign := fmt.Sprintf(`market%s,coins%s,seconds86400,%s~%s,far%d,near%d,limit%d,allLimit%d,useNear%v`,
		market, coins, strBegin, strEnd, far, near, limit, allLimit, useNear)
	delNum := model.AppDB.Where(`function=?`, sign).Delete(&model.Order{}).RowsAffected
	util.Info(`del %s %d rows affected`, sign, delNum)
	regret.ProcessCandles(begin, end, near, far, 86400, allLimit, useNear, market, sign, settings)
	util.StoreSyncMap(&model.CarryInfo, fmt.Sprintf("done %s %s 使用回撤%v %d~%d 限制%d 总限制%d",
		strBegin, strEnd, useNear, near, far, limit, allLimit), `auto`)
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
				c.String(http.StatusOK, fmt.Sprintf("simulating...\nanto："+msg.(string), autoMsg.(string)))
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
		market = model.BinancePerp
	}
	strTurtleSeconds := c.Query(`seconds`)
	turtleSeconds, errTurtleSeconds := strconv.ParseInt(strTurtleSeconds, 10, 64)
	if errTurtleSeconds != nil {
		turtleSeconds = 86400
		strTurtleSeconds = `86400`
		errTurtleSeconds = nil
	}
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
	useNear, useNearErr := strconv.ParseBool(strUseNear)
	begin, errBegin := time.Parse(time.RFC3339, strBegin)
	end, errEnd := time.Parse(time.RFC3339, strEnd)
	sign := fmt.Sprintf(`market%s,coins%s,seconds%s,%s~%s,far%s,near%s,limit%s,allLimit%s,useNear%s`,
		market, coins, strTurtleSeconds, strBegin, strEnd, farStr, nearStr, strLimit, strAllLimit, strUseNear)
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
		for i := 7; i <= 25; i++ {
			if allLimit == 12 {
				autoSimulate(market, coins, begin, end, strBegin, strEnd, true, i, 2*i, 3, int(allLimit))
				autoSimulate(market, coins, begin, end, strBegin, strEnd, false, i, 2*i, 3, int(allLimit))
			} else {
				if market != model.GXZQ {
					autoSimulate(market, coins, begin, end, strBegin, strEnd, true, i, 2*i, 3, 3)
					autoSimulate(market, coins, begin, end, strBegin, strEnd, false, i, 2*i, 3, 3)
					autoSimulate(market, coins, begin, end, strBegin, strEnd, true, i, 2*i, 4, 4)
					autoSimulate(market, coins, begin, end, strBegin, strEnd, false, i, 2*i, 4, 4)
				} else {
					coinNames := strings.Split(coins, `,`)
					for _, coinName := range coinNames {
						autoSimulate(market, coinName, begin, end, strBegin, strEnd, true, i, 2*i, 3, 3)
						autoSimulate(market, coinName, begin, end, strBegin, strEnd, false, i, 2*i, 3, 3)
						autoSimulate(market, coinName, begin, end, strBegin, strEnd, true, i, 2*i, 4, 4)
						autoSimulate(market, coinName, begin, end, strBegin, strEnd, false, i, 2*i, 4, 4)
					}
				}
			}
		}
		util.StoreSyncMap(&model.CarryInfo, nil, `auto`)
		c.String(http.StatusOK, `auto done`)
		return
	}
	if errBegin != nil || errEnd != nil || turtleSeconds <= 0 || nearErr != nil || farErr != nil ||
		useNearErr != nil || (turtleSeconds != 1800 && turtleSeconds != 14400 && turtleSeconds%86400 != 0) {
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
		symbol := strings.ToUpper(coinArray[i]) + tail
		settings[symbol] = &model.Setting{Market: market, Symbol: symbol, AmountLimit: float64(limit), GridAmount: RegretTurtleGridAmount}
	}
	if strNew == `true` {
		go model.AppDB.Where(`function=?`, sign).Delete(&model.Order{})
		regret.ProcessCandles(begin, end, int(near), int(far), int(turtleSeconds), int(allLimit), useNear, market, sign, settings)
	} else {
		util.Notice(`no need process simulate new %s`, strNew)
	}
	orders := make([]*model.Order, 0)
	model.AppDB.Where(`function=?`, sign).Order(`order_time asc`).Find(&orders)
	msg += fmt.Sprintf("Get %d orders %s %s %v %s %s from %d settings\n",
		len(orders), strTurtleSeconds, strLimit, strUseNear, begin.String(), end.String(), len(settings))
	msg += regret.ToString(orders, market, strTurtleSeconds, strLimit, RegretTurtleGridAmount, begin, end) + "\n"
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
	force := c.Query(`force`)
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		index = 0
	}
	queryAccounts := api.GetAccounts(index)
	marketValues := make([][]string, 0)
	inAll := []float64{0, 0, 0, 0, 0, 0}
	for _, account := range queryAccounts {
		if account != nil {
			inAllSpot, contractAccountValue, holdingSpot, holdingFuture, unrealizedPnl, keepInU :=
				cross.GetCrossMarketValue(account.Key, account.Secret, account.Market, force == `true`)
			duplicated := false
			for _, value := range marketValues {
				if (strings.Contains(value[0], `binance`) && strings.Contains(account.Market, `binance`)) ||
					(strings.Contains(value[0], `bybit`) && strings.Contains(account.Market, `bybit`)) {
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
			inAll[0] += inAllSpot
			if index > 0 {
				inAll[0] -= keepInU
			}
			if account.Market != model.Ftx && account.Market != model.OKEX {
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
		Order(`date(order_time) desc,market`).Rows()
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
	carryRows, _ := model.AppDB.Model(model.Order{}).Select(`amount_type,order_side,sum(price*abs(amount)),date(order_time),count(*),refresh_type`).
		Where(`refresh_type!=?`, model.FunctionSimulation).
		Group(`order_side,date(order_time),amount_type,refresh_type`).Order(`date(order_time) desc`).Rows()
	if carryRows != nil {
		crossInU := map[string]map[string]float64{}
		crossCount := map[string]map[string]float64{}
		for carryRows.Next() {
			var side, date, amountType, refreshType string
			var value, orderNum float64
			_ = carryRows.Scan(&amountType, &side, &value, &date, &orderNum, &refreshType)
			dates := strings.Split(date, `-`)
			date = fmt.Sprintf(`%s-%s`, dates[1], dates[2])
			date = date[0:strings.Index(date, `T`)]
			i := model.AppConfig.GetIndexFromKey(amountType)
			if i == index {
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
		carryRows.Close()
	}
	carryRows, _ = model.AppDB.Model(model.Order{}).Select(`market,amount_type,order_side,sum(price*abs(amount)),date(order_time),refresh_type,count(*)`).
		Where(`refresh_type!=?`, model.FunctionSimulation).Group(`market,order_side,date(order_time),amount_type,refresh_type`).Order(`date(order_time) desc, market`).Rows()
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
	param := c.Query(`markets`)
	api.InitCrossMarketInfos(strings.Split(param, `,`))
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
	crossInfo := model.GetMonitorInfo(indexStr, `cross`)
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
		c.String(http.StatusOK, `调用成功，请查收邮箱，如果没有，检查日志`)
	}
}

func GetParameters(c *gin.Context) {
	msg := ``
	markets := api.GetMarkets()
	userKeys := make([]string, 0)
	for _, market := range markets {
		account := model.AppConfig.GetAccounts(market)[0]
		userKeys = append(userKeys, account.Key)
		settingMap := api.GetSettings(model.FunctionTurtle, market)
		msg += fmt.Sprintf("海龟币种：%s \n", market)
		if settingMap != nil {
			settingMap.Range(func(symbol, setting interface{}) bool {
				if setting == nil || setting.(*model.Setting).Function != model.FunctionTurtle {
					return true
				}
				turtleData := carry.GetTurtleData(account.Key, account.Secret, setting.(*model.Setting))
				isTop := true
				if setting.(*model.Setting).SymbolRelated == model.SettingTurtleRemoved {
					isTop = false
				}
				msg += fmt.Sprintf("%s 仓数:%d 持仓:%f 成交价:%f top:%v %s\n",
					symbol, setting.(*model.Setting).Chance, setting.(*model.Setting).GridAmount, setting.(*model.Setting).PriceX, isTop, turtleData.ToString())
				if setting.(*model.Setting).Function == model.FunctionTurtle {
					showMsg := fmt.Sprintf("%s_%s_%s", model.FunctionTurtle, market, symbol)
					msgValue, ok := util.LoadSyncMap(&model.CarryInfo, account.Key, showMsg)
					if ok && msgValue != nil {
						msg += msgValue.(string) + "\n"
					}
				}
				return true
			})
		}
		msg += "\n"
	}
	util.Notice(`finish print turtle settings`)
	setting := api.GetSetting(model.FunctionGrid, model.Ftx, `LINK-PERP`)
	if setting != nil {
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
	msg += model.AppMetric.ToString()
	c.String(http.StatusOK, msg)
}

//func RefreshParameters(c *gin.Context) {
//	util.Notice(`controller refreshing`)
//	api.LoadSettings()
//	for _, market := range api.GetMarkets() {
//		channels, _ := model.AppMarkets.WsDepth.Load(market)
//		if channels != nil {
//			carry.ResetChannels(market, channels.([]chan struct{}))
//		}
//	}
//	api.InitMarketInfos()
//	c.String(http.StatusOK, model.AppConfig.ToString())
//}
