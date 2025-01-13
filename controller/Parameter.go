package controller

import (
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"hello/api"
	"hello/carry/cross"
	"hello/model"
	"hello/regret"
	"hello/regret/Grid"
	"hello/util"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

var simulating = false
var candling = false

const RegretTurtleGridAmount = 1000

func ParameterServe() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	_ = router.SetTrustedProxies(nil)
	router.LoadHTMLGlob("templates/*")
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Max")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(200)
		}
		c.Next()
	})
	store := cookie.NewStore([]byte("secretKey&^%&^$&^%)*()9878687"))
	router.Use(sessions.Sessions("mysession", store))
	router.GET("/", GetParameters)
	//router.GET(`refresh`, RefreshParameters)
	//router.GET(`test`, testSpeed)
	router.GET(`currentUser`, currentUser)
	router.POST(`pw`, GetCode)
	router.GET(`simulate`, simulate)
	router.GET(`simulateGrid`, simulateGrid)
	router.GET(`cross`, crossPage)
	router.GET(`hold`, holdPage)
	router.GET(`tick`, tickPage)
	router.GET(`cross_refresh`, crossRefresh)
	router.GET(`debug`, debug)
	router.GET(`candles`, getCandles)
	router.GET(`mine`, mindZeroAddr)
	router.GET(`monitor`, MonitorTrade)
	router.GET(`entry`, monitorEntry)
	router.GET(`init`, InitFullMonitors)
	router.GET(`withdraws`, getWithdraws)
	router.POST(`login`, login)
	router.POST(`get_monitors`, getSettingMonitors)
	router.POST(`add_monitor`, addSettingMonitor)
	router.POST(`remove_monitor`, removeSettingMonitor)
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

func setCandling(value bool) {
	candling = value
}

func autoSimulate(market, coins string, begin, end time.Time, strBegin, strEnd string, useNear, useM bool,
	near, far, limit, allLimit, seconds int) {
	coinArray := strings.Split(coins, `,`)
	settings := make(map[string]*model.Setting)
	marketType := model.MarketTypePerp
	for _, coin := range coinArray {
		symbol := coin + model.UniStandardTail[marketType]
		if useNear {
			settings[symbol] = &model.Setting{Market: market, Symbol: symbol, AmountLimit: float64(limit), WSType: model.WSTypeTicker,
				GridAmount: RegretTurtleGridAmount, Seconds: int64(seconds), Near: int64(near), Far: int64(far)}
		} else { // 以useNear false测试龟汤，所以没有设置滑点即tradeCost=0
			settings[symbol] = &model.Setting{Market: market, Symbol: symbol, AmountLimit: float64(limit), WSType: model.WSTypeTicker,
				GridAmount: RegretTurtleGridAmount, Seconds: int64(seconds), Near: int64(near), Far: int64(far)}
		}
	}
	sign := fmt.Sprintf(`market%s,coins%s,%s~%s,far%d,near%d,limit%d,allLimit%d,useNear%v,useM%vseconds%d`,
		market, coins, strBegin, strEnd, far, near, limit, allLimit, useNear, useM, seconds)
	delNum := model.AppDB.Where(`function=?`, sign).Delete(&model.Order{}).RowsAffected
	util.Log(util.LogLevelInfo, fmt.Sprintf(`del %s %d rows affected`, sign, delNum))
	regret.ProcessCandles(begin, end, far, allLimit, useNear, useM, market, sign, settings)
	util.Log(util.LogLevelInfo, fmt.Sprintf(`auto simulation done %s`, sign))
	util.StoreSyncMap(&model.CarryInfo, fmt.Sprintf("done %s %s 使用回撤%v %d~%d 限制%d 总限制%d",
		strBegin, strEnd, useNear, near, far, limit, allLimit), `auto`)
}

func currentUser(c *gin.Context) {
	c.String(http.StatusOK, `{"data": {"name": "Serati Ma","avatar":
		"https://gw.alipayobjects.com/zos/rmsportal/BiazfanxmamNRoxxVxka.png","userid": "00000001","email": "antdesign@alipay.com",
		"signature": "海纳百川，有容乃大","title": "交互专家","group": "蚂蚁金服－某某某事业群－某某平台部－某某技术部－UED","tags": 
		[{"key": "0","label": "很有想法的"},{"key": "1","label": "专注设计"},{"key": "2","label": "辣~"},{"key": "3","label": "大长腿"},
		{"key": "4","label": "川妹子"},{"key": "5","label": "海纳百川"}],"notifyCount": 12,"unreadCount": 11,"country": "China","geographic": 
		{"province": {"label": "浙江省","key": "330000"},"city": {"label": "杭州市","key": "330100"}},"address": "西湖区工专路 77 号","phone": "0752-268888888"}}`)
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
	//strSeconds := c.Query(`seconds`)
	//strFar := c.Query(`far`) // both, buy, sell
	//function := c.Query(`function`)
	//far, errFar := strconv.ParseInt(strFar, 10, 64)
	//seconds, errSeconds := strconv.ParseInt(strSeconds, 10, 64)
	strBegin := c.Query(`begin`) + `T00:00:00+00:00`
	strEnd := c.Query(`end`) + `T00:00:00+00:00`
	begin, _ := time.Parse(time.RFC3339, strBegin)
	end, _ := time.Parse(time.RFC3339, strEnd)
	session := sessions.Default(c)
	sessionValue := session.Get(`user`)
	if sessionValue != `haoweizh@qq.com` {
		strNew = `false`
	}
	coinArr := strings.Split(coins, `,`)
	for _, coin := range coinArr {
		if strNew == `true` {
			symbol := strings.ToUpper(coin) + model.UniStandardTail[model.MarketTypePerp]
			account := model.AppConfig.GetAccounts(market)[0]
			candles := api.GetMultiCandle(account, market, 60, begin, end,
				map[string]*model.Setting{symbol: {Market: market, Symbol: symbol, Coin: coin}}, false)
			util.StoreSyncMap(&model.CarryInfo, fmt.Sprintf(`get market candle %s %s from %s len %d`,
				market, symbol, begin.String(), len(candles)), `gridInfo`)
			settings := []*model.Setting{
				{Market: market, Symbol: symbol, Coin: coin, Function: fmt.Sprintf(`1_20_86400_%s_%s`, strBegin, strEnd), Far: 20, Near: 10, Seconds: 86400, CloseShortMargin: 1},
				{Market: market, Symbol: symbol, Coin: coin, Function: fmt.Sprintf(`1_60_14400_%s_%s`, strBegin, strEnd), Far: 60, Near: 30, Seconds: 14400, CloseShortMargin: 1},
				{Market: market, Symbol: symbol, Coin: coin, Function: fmt.Sprintf(`1.05_20_86400_%s_%s`, strBegin, strEnd), Far: 20, Near: 10, Seconds: 86400, CloseShortMargin: 1.05},
				{Market: market, Symbol: symbol, Coin: coin, Function: fmt.Sprintf(`1.05_60_14400_%s_%s`, strBegin, strEnd), Far: 60, Near: 30, Seconds: 14400, CloseShortMargin: 1.05},
				{Market: market, Symbol: symbol, Coin: coin, Function: fmt.Sprintf(`1.1_20_86400_%s_%s`, strBegin, strEnd), Far: 20, Near: 10, Seconds: 86400, CloseShortMargin: 1.1},
				{Market: market, Symbol: symbol, Coin: coin, Function: fmt.Sprintf(`1.1_60_14400_%s_%s`, strBegin, strEnd), Far: 60, Near: 30, Seconds: 14400, CloseShortMargin: 1.1},
				{Market: market, Symbol: symbol, Coin: coin, Function: fmt.Sprintf(`1.15_20_86400_%s_%s`, strBegin, strEnd), Far: 20, Near: 10, Seconds: 86400, CloseShortMargin: 1.15},
				{Market: market, Symbol: symbol, Coin: coin, Function: fmt.Sprintf(`1.15_60_14400_%s_%s`, strBegin, strEnd), Far: 60, Near: 30, Seconds: 14400, CloseShortMargin: 1.15},
			}
			for _, setting := range settings {
				delNum := model.AppDB.Where(`function=? and market=? and symbol=? and order_time>? and order_time<?`,
					setting.Function, market, setting.Symbol, strBegin, strEnd).Delete(&model.Order{}).RowsAffected
				util.Log(util.LogLevelInfo, fmt.Sprintf(`del rows %d %s %s %s %s~%s`, delNum, setting.Function, market, setting.Symbol, strBegin, strEnd))
				Grid.ProcessGrid(begin, end, setting, candles)
			}
		}
	}
	util.Log(util.LogLevelInfo, fmt.Sprintf(`done simulate grid %s %#v`, market, coins))
	c.String(http.StatusOK, `done`)
}

func mindZeroAddr(c *gin.Context) {
	session := sessions.Default(c)
	sessionValue := session.Get(`user`)
	if sessionValue != `haoweizh@qq.com` {
		c.String(http.StatusForbidden, `no right`)
		return
	}
	c.String(http.StatusOK, util.RunMindZeroAddr(6, 11, 4))
}

func getWithdraws(c *gin.Context) {
	session := sessions.Default(c)
	sessionValue := session.Get(`user`)
	if sessionValue != `haoweizh@qq.com` {
		c.String(http.StatusUnauthorized, `need login`)
		return
	}
	accounts := model.GetAccounts(0)
	var result string
	for _, account := range accounts {
		if account == nil {
			continue
		}
		balances := api.GetTransfers(account.Key, account.Secret, account.Market)
		for _, balance := range balances {
			if balance == nil {
				continue
			}
			result += fmt.Sprintf("%.0f %s %s %s %f add:%s txId %s status %s \n",
				balance.Action, balance.CreatedAt, balance.Market, balance.Coin, balance.Amount, balance.Address, balance.TransactionId, balance.Status)
		}
	}
	c.String(http.StatusOK, result)
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
	util.Log(util.LogLevelInfo, fmt.Sprintf(`get simulation parameter %s auto%s`, sign, auto))
	limit, limitErr := strconv.ParseInt(strLimit, 10, 64)
	if limitErr != nil {
		limit = 3
	}
	allLimit, allLimitErr := strconv.ParseInt(strAllLimit, 10, 64)
	if allLimitErr != nil {
		allLimit = limit
	}
	session := sessions.Default(c)
	sessionValue := session.Get(`user`)
	if sessionValue != `haoweizh@qq.com` {
		strNew = `false`
	} else if auto == `true` && strNew == `true` {
		for i := 3; i <= 25; i++ {
			//autoSimulate(market, coins, begin, end, strBegin, strEnd, true, false, i, 2*i, 3, int(allLimit), int(seconds), fee)
			autoSimulate(market, coins, begin, end, strBegin, strEnd, true, false, i, 2*i, 3, int(allLimit), int(seconds))
			autoSimulate(market, coins, begin, end, strBegin, strEnd, true, false, i, 2*i, 3, int(allLimit), int(seconds))
			autoSimulate(market, coins, begin, end, strBegin, strEnd, true, false, i, 2*i, 3, int(allLimit), int(seconds))
			//autoSimulate(market, coins, begin, end, strBegin, strEnd, false, true, i, 2*i, 3, int(allLimit), int(seconds), fee)
		}
		util.StoreSyncMap(&model.CarryInfo, nil, `auto`)
		util.Log(util.LogLevelInfo, fmt.Sprintf(`all auto simulation done fee %f`, fee))
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
		symbol := coinArray[i] + tail
		settings[symbol] = &model.Setting{Market: market, Symbol: symbol, AmountLimit: float64(limit), WSType: model.WSTypeTicker,
			GridAmount: RegretTurtleGridAmount, Near: near, Far: far, Seconds: seconds}
	}
	if strNew == `true` {
		go model.AppDB.Where(`function=?`, sign).Delete(&model.Order{})
		regret.ProcessCandles(begin, end, int(far), int(allLimit), useNear, useM, market, sign, settings)
	} else {
		util.Log(util.LogLevelInfo, fmt.Sprintf(`no need process simulate new %s`, strNew))
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
		//util.DebugCount = 0
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
	sessionValue := session.Get(`user`)
	if sessionValue != `haoweizh@qq.com` {
		c.String(http.StatusUnauthorized, `not logged in`)
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
			inAllSpot, contractAccountValue, holdingSpot, holdingFuture, unrealizedPnl :=
				cross.GetCrossMarketValue(account.Key, account.Secret, account.Market, force == `true`)
			marketValues = append(marketValues, []string{account.Market,
				strconv.FormatFloat(inAllSpot, 'f', 0, 64),
				strconv.FormatFloat(contractAccountValue, 'f', 0, 64),
				strconv.FormatFloat(holdingSpot, 'f', 0, 64),
				strconv.FormatFloat(holdingFuture, 'f', 0, 64),
				strconv.FormatFloat(unrealizedPnl, 'f', 0, 64)})
			if account.Market != model.BinanceSpot && account.Market != model.BitgetSpot {
				inAll[0] += contractAccountValue
			}
			// 统一账户不算现货总价值
			if !account.IsUnified && account.Market != model.BinancePerp && account.Market != model.BitgetPerp {
				inAll[0] += inAllSpot
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
	compRows, _ := model.AppDB.Model(model.Order{}).Select(`market,order_side,date(order_time),sum(price*abs(amount))`).
		Where(`refresh_type=? and account_index=?`, `comp`, indexStr).Group(`market,order_side,date(order_time)`).
		Order(`date(order_time) desc,market`).Rows()
	compData := make(map[string]float64) // market - account_index - side - date - fail count
	if compRows != nil {
		for compRows.Next() {
			var marketName, side, date string
			var compMoney float64
			_ = compRows.Scan(&marketName, &side, &date, &compMoney)
			dates := strings.Split(date, `-`)
			date = fmt.Sprintf(`%s-%s`, dates[1], dates[2])
			date = date[0:strings.Index(date, `T`)]
			key := fmt.Sprintf(`%s-%s-%s`, marketName, side, date)
			compData[key] = compMoney
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
		if carryRows.Close() != nil {
			util.Log(util.LogLevelError, fmt.Sprintf(`fail to close DB for carry rows`))
			return
		}
	}
	orders := make([][]string, 0)
	location, _ := time.LoadLocation("Asia/Shanghai")
	carryRows, _ = model.AppDB.Model(model.Order{}).Select(`order_id,market,symbol,order_time,order_side,price,amount,price*abs(amount),refresh_type,err_code,status`).
		Where(`account_index=? and (err_code!=? or status=? or (order_side=? and fee>?and price/fee>?) or (order_side=? and fee>?and fee/price>?))`,
			indexStr, ``, `fail`, model.OrderSideBuy, 0, 1.2, model.OrderSideSell, 0, 1.2).Order(`order_time desc`).Limit(100).Rows()
	if carryRows != nil {
		for carryRows.Next() {
			var orderId, market, symbol, orderSide, refreshType, errCode, status string
			var orderTime time.Time
			var price, amount, money float64
			_ = carryRows.Scan(&orderId, &market, &symbol, &orderTime, &orderSide, &price, &amount, &money, &refreshType, &errCode, &status)
			orders = append(orders, []string{orderId, market, symbol, orderTime.In(location).Format(time.DateTime), orderSide,
				fmt.Sprintf(`%f`, price), fmt.Sprintf(`%e`, amount), fmt.Sprintf(`%.0f`, money), refreshType, errCode, status})
		}
	}
	carryRows, _ = model.AppDB.Model(model.Order{}).Select(`market,order_side,sum(price*abs(amount)),date(order_time),refresh_type,count(*)`).
		Where(`refresh_type!=? and account_index=?`, model.FunctionSimulation, indexStr).Group(
		`market,order_side,date(order_time),refresh_type`).Order(`date(order_time) desc, market`).Rows()
	if carryRows != nil {
		for carryRows.Next() {
			var marketName, side, date, refreshType string
			var value, orderNum float64
			_ = carryRows.Scan(&marketName, &side, &value, &date, &refreshType, &orderNum)
			dates := strings.Split(date, `-`)
			date = fmt.Sprintf(`%s-%s`, dates[1], dates[2])
			date = date[0:strings.Index(date, `T`)]
			key := fmt.Sprintf(`%s-%s-%s`, marketName, side, date)
			compRate := 0.0
			if compData[key] > 0 {
				compRate = 100 * compData[key] / value
			}
			compStr := ``
			if refreshType == model.FunctionCross {
				compStr = strconv.FormatFloat(compRate, 'f', 2, 64)
			}
			tradeInfo = append(tradeInfo, []string{marketName, date, side,
				strconv.FormatFloat(value, 'f', 0, 64), refreshType,
				strconv.FormatFloat(orderNum, 'f', 0, 64), compStr})
		}
		if carryRows.Close() != nil {
			util.Log(util.LogLevelInfo, fmt.Sprintf(`fail to close DB for carry rows`))
			return
		}
	}
	carryCoins := make([][]string, 0)
	carryRows, _ = model.AppDB.Model(model.CarryCoin{}).Select(`coin,current_step,holding,money_per_step,money_cur_step,price`).
		Where(`account_index=?`, indexStr).Order(`current_step desc`).Rows()
	if carryRows != nil {
		for carryRows.Next() {
			var coin string
			var currentStep int
			var holding, moneyPerStep, moneyCurStep, price float64
			_ = carryRows.Scan(&coin, &currentStep, &holding, &moneyPerStep, &moneyCurStep, &price)
			if currentStep > 0 || holding > 0 || moneyCurStep > 0 {
				carryCoins = append(carryCoins, []string{coin, fmt.Sprintf(`%d`, currentStep), fmt.Sprintf(`%e`, holding),
					fmt.Sprintf(`%.1f`, moneyPerStep), fmt.Sprintf(`%.1f`, moneyCurStep), fmt.Sprintf(`%e`, price)})
			}
		}
		if carryRows.Close() != nil {
			util.Log(util.LogLevelInfo, fmt.Sprintf(`fail to close DB for carry rows`))
			return
		}
	}
	c.HTML(http.StatusOK, `hold.gohtml`, gin.H{`marketValue`: marketValues,
		`trade`: tradeInfo, `orders`: orders, `holdings`: cross.GetHoldings(indexStr, queryAccounts)})
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
	//markets := api.GetMarkets()
	//for _, market := range markets {
	//	api.SetRequireReset(market)
	//}
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

func createTurtleLines(function, market string, account *model.Account) (msg string, size int) {
	settingMap := api.GetSettings(function, market)
	lines := make([]*Sortable, 0)
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
		//util.Info(fmt.Sprintf(`create lines %s %s %s`, function, market, symbol))
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
				sortable := &Sortable{Key: symbol.(string), Value: msgValue.(string) + "\n"}
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
	sortedLines := &SortableArray{Array: lines}
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
	userKeys := make([]string, 0)
	for _, market := range model.AppEnvironment.Markets {
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
//		channels, _ := model.AppEnvironment.MsgChanTick.Load(market)
//		if channels != nil {
//			carry.ResetChannels(market, channels.([]chan struct{}))
//		}
//	}
//	api.InitMarketInfos()
//	c.String(http.StatusOK, model.AppConfig.ToString())
//}
