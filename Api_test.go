package main

import (
	"flag"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/jinzhu/configor"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"hello/api"
	"hello/model"
	"hello/regret"
	"hello/util"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

var msgChan = make(chan string, 10)

func init() {
	go handleMsg()
}

func handleMsg() {
	for true {
		msg := <-msgChan
		fmt.Println(fmt.Sprintf(`%s %d`, msg, len(msgChan)))
	}
}

func timeWriter(conn *websocket.Conn) {
	for {
		time.Sleep(time.Minute * 5)
		_ = conn.WriteMessage(websocket.TextMessage, []byte(time.Now().Format("2006-01-02 15:04:05")+`ping`))
	}
}

func Test_ws(t *testing.T) {
	var addr = flag.String("addr", "ec2-18-179-17-108.ap-northeast-1.compute.amazonaws.com:443", "http service address")
	//var addr = flag.String("addr", "localhost:443", "http service address")

	u := url.URL{Scheme: "wss", Host: *addr, Path: "/wss"}
	var dialer *websocket.Dialer

	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	go timeWriter(conn)
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("read:", err)
			return
		}
		fmt.Printf("received: %s\n", message)
	}
}

func Test_getCommonMarketInfos(t *testing.T) {
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	//api.InitCrossMarketInfos([]string{model.Gate})
	api.InitMarketInfos()
	order1 := api.PlaceOrder(model.AppConfig.GateKey, model.AppConfig.GateSecret, model.OrderSideBuy, model.OrderTypeLimit,
		model.Gate, `KISHU_USDT`, ``,
		0.00000000038, 0.00000000038, 12000000000, false, nil, nil)
	fmt.Println(order1.OrderId)
	success, pos, value, u := api.GetPositions(model.AppConfig.GateKey, model.AppConfig.GateSecret, model.Gate)
	fmt.Println(fmt.Sprintf(`%v %v %v %v`, success, pos, value, u))
	markets := api.GetMarketInfos(model.Gate)
	for s, info := range markets {
		fmt.Println(fmt.Sprintf(`%s %s %s`, s, info.Name, info.Market))
	}
	//api.InitCrossMarketInfos([]string{model.OKEX, model.Ftx, model.Gate})
}

func TestWs(t *testing.T) {
	market := model.Ftx
	model.NewConfig()
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	//api.InitMarketInfos()
	api.CreateMarketDepthServer(model.AppMarkets, market, nil)
	select {}
}

func Test_WsAndOrderApi(t *testing.T) {
	market := model.Mexc
	coin := `ETH`
	orderType := model.OrderTypeLimit
	orderSide := model.OrderSideSell
	symbols := []string{coin + model.UniStandardTail[model.MarketTypePerp]}
	//coin + model.UniStandardTail[model.MarketTypeSpot]}
	model.NewConfig()
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	api.InitMarketInfos()
	account := model.AppConfig.GetAccounts(market)[0]
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	api.CreateMarketDepthServer(model.AppMarkets, market, nil)
	for _, symbol := range symbols {
		api.CancelOrders(account.Key, account.Secret, market, symbol)
		getTick := false
		var tick *model.BidAsk
		for !getTick {
			time.Sleep(time.Minute * 5)
			getTick, tick = model.AppMarkets.GetBidAsk(symbol, market)
		}
		price := tick.Bids[len(tick.Bids)-1].Price * 1.05
		amount := 20 / price
		order := api.PlaceOrder(account.Key, account.Secret, orderSide, orderType, market,
			symbol, ``, price, price, amount, false, nil, nil)
		fmt.Println(fmt.Sprintf(`1. place order return %v`, order))
		if order != nil && order.OrderId != `` {
			queryOrder := api.QueryOrderById(account.Key, account.Secret, market, symbol, orderType, order.OrderId)
			fmt.Println(fmt.Sprintf(`2. query order %s return %s %s %v`, order.OrderId, queryOrder.OrderId, queryOrder.Status, queryOrder))
		} else {
			fmt.Println(fmt.Sprintf(`1. fail to place order`))
			continue
		}
		//cancelResult, errCode, errMsg, cancelOrder := api.CancelOrder(account.Key, account.Secret, market, symbol,
		//	orderType, order.OrderId)
		//fmt.Println(fmt.Sprintf(`3. cancel %s return %v %s %s %v`, order.OrderId, cancelResult, errCode, errMsg, cancelOrder))
		//queryOrder := api.QueryOrderById(account.Key, account.Secret, market, symbol, orderType, order.OrderId)
		//fmt.Println(fmt.Sprintf(`4. query order %s return %s %s %v`, order.OrderId, queryOrder.OrderId, queryOrder.Status, queryOrder))
		//order1 := api.PlaceOrder(account.Key, account.Secret, orderSide, orderType, market,
		//	symbol, ``, ``, price, price, amount, false, true, nil, nil)
		//order2 := api.PlaceOrder(account.Key, account.Secret, orderSide, orderType, market,
		//	symbol, ``, ``, price, price, amount, false, true, nil, nil)
		//fmt.Println(fmt.Sprintf(`5. place order return %v %v`, order1, order2))
		api.PlacePairOKEX(account.Key, symbol, symbol, model.OrderTypeLimit, price*0.9, price*1.1, amount)
		api.CancelOrders(account.Key, account.Secret, market, symbol)
		//if order1 != nil {
		//	time.Sleep(time.Second)
		//	queryOrder = api.QueryOrderById(account.Key, account.Secret, market, symbol, orderType, order1.OrderId)
		//	fmt.Println(fmt.Sprintf(`6. query order %s return %s %s %v`, order1.OrderId, queryOrder.OrderId, queryOrder.Status, queryOrder))
		//}
		//if order2 != nil {
		//	queryOrder = api.QueryOrderById(account.Key, account.Secret, market, symbol, orderType, order2.OrderId)
		//	fmt.Println(fmt.Sprintf(`6. query order %s return %s %s %v`, order2.OrderId, queryOrder.OrderId, queryOrder.Status, queryOrder))
		//}
	}
	select {}
}

func Test_BalAndPos(t *testing.T) {
	model.NewConfig()
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	api.GetMarketInfos(model.Gate)
	order := api.QueryOrderById(model.AppConfig.GateKey, model.AppConfig.GateSecret, `gate`, `MGA_USDT`,
		model.OrderTypeLimit, `144149811503`)
	fmt.Println(order)
	api.InitMarketInfos()
	//balMarkets := []string{model.OKEX, model.BybitSpot, model.Ftx, model.Gate}
	//for _, market := range balMarkets {
	//	account := model.AppConfig.GetAccounts(market)[0]
	//	success, balances, total, collateral := api.GetBalances(account.Key, account.Secret, market)
	//	fmt.Println(fmt.Sprintf(`%v %f %v %d`, success, total, collateral, len(balances)))
	//	for _, balance := range balances {
	//		if balance.Coin == `USDT` || balance.Coin == `USD` {
	//			fmt.Println(fmt.Sprintf(`usd amount %s %f`, market, balance.Amount))
	//		}
	//	}
	//}
	posMarkets := []string{model.BybitPerp}
	//posMarkets := []string{model.OKEX, model.BybitPerp, model.Ftx}
	for _, market := range posMarkets {
		account := model.AppConfig.GetAccounts(market)[0]
		success, positions, total, available := api.GetPositions(account.Key, account.Secret, market)
		fmt.Println(fmt.Sprintf(`%v %f %f %d`, success, total, available, len(positions)))
		for _, position := range positions {
			fmt.Println(fmt.Sprintf(`%s %f`, position.Currency, position.Holding))
			api.CancelOrders(account.Key, account.Secret, market, position.Currency)
		}
		success, positions, total, available = api.GetPositions(account.Key, account.Secret, market)
		for _, position := range positions {
			fmt.Println(fmt.Sprintf(`%s %f`, position.Currency, position.Holding))
			api.CancelOrders(account.Key, account.Secret, market, position.Currency)
		}
	}
}

func Test_CreateReport(t *testing.T) {
	model.NewConfig()
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	//begin, _ := time.Parse(time.RFC3339, `2019-01-01T00:00:00+08:00`)
	//end, _ := time.Parse(time.RFC3339, `2023-01-01T00:00:00+08:00`)
	market := model.GXZQ
	//coins := `doge,sol,matic,chz,link,ada,bnb,fil,sushi,axs,atom,waves`
	timeRage := `2019-01-01T00:00:00+08:00~2023-01-01T00:00:00+08:00`
	coins := `CZCE.TA,SHFE.rb,CZCE.MA,DCE.m,CZCE.FG,DCE.c,DCE.i,CZCE.SA,DCE.v,SHFE.hc`
	regret.CreateReport(market, coins, timeRage)
	//coins := `btc,eth`
}

func Test_CutTail(t *testing.T) {
	model.NewConfig()
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	//coins := `doge,sol,matic,chz,link,ada,bnb,fil,sushi,axs,atom,waves`
	//allLimit := 12
	market := model.GXZQ
	strBegin := `2019-01-01T00:00:00+08:00`
	strEnd := `2023-01-01T00:00:00+08:00`
	coinNames := strings.Split(`CZCE.CY,CZCE.RM,CZCE.OI,CZCE.SR,CZCE.ZC,CZCE.SM,CZCE.SF,CZCE.UR,CZCE.PF,CZCE.SA,DCE.p,DCE.l,DCE.y,DCE.pp,DCE.j,DCE.jm,DCE.eg,DCE.eb,SHFE.cu,SHFE.al,SHFE.zn,SHFE.pb,SHFE.nl,SHFE.sn,SHFE.bu,SHFE.fu,SHFE.ru`, `,`)
	for i := 7; i <= 25; i++ {
		for _, coins := range coinNames {
			sign := fmt.Sprintf(`market%s,coins%s,seconds86400,%s~%s,far%d,near%d,limit%d,allLimit%d,useNear%v`,
				market, coins, strBegin, strEnd, i*2, i, 3, 12, true)
			regret.CutTail(market, coins, sign)
			sign = fmt.Sprintf(`market%s,coins%s,seconds86400,%s~%s,far%d,near%d,limit%d,allLimit%d,useNear%v`,
				market, coins, strBegin, strEnd, i*2, i, 3, 12, false)
			regret.CutTail(market, coins, sign)
			sign = fmt.Sprintf(`market%s,coins%s,seconds86400,%s~%s,far%d,near%d,limit%d,allLimit%d,useNear%v`,
				market, coins, strBegin, strEnd, i*2, i, 4, 12, true)
			regret.CutTail(market, coins, sign)
			sign = fmt.Sprintf(`market%s,coins%s,seconds86400,%s~%s,far%d,near%d,limit%d,allLimit%d,useNear%v`,
				market, coins, strBegin, strEnd, i*2, i, 4, 12, false)
			regret.CutTail(market, coins, sign)
		}
	}
	fmt.Println(`done`)
}

func Test_initTurtleN(t *testing.T) {
	model.NewConfig()
	//model.AppRedis.Set(context.Background(), `binanceperp_BTC_PERP_30m_1673913600000_1673962933185_27`, `1test`, 0)
	//res, err := model.AppRedis.Get(context.Background(), `binanceperp_BTC_PERP_30m_1673913600000_1673962933185_27`).Result()
	//fmt.Println(fmt.Sprintf(`%s %s`, res, err.Error()))
	today, _ := model.GetMarketToday(model.BinancePerp)
	settings := map[string]*model.Setting{`BTC_PERP`: nil, `ETH_PERP`: nil}
	api.GetCandle(model.AppConfig.BinanceKey, model.AppConfig.BinanceSecret, model.BinancePerp, `BTC_PERP`,
		60, time.Now().Add(time.Minute*-1839600), today)
	sortedCandles := api.GetMultiCandle(model.AppConfig.BinanceKey, model.AppConfig.BinanceSecret, model.BinancePerp, 60,
		today.Add(time.Minute*-490), today, settings)
	fmt.Println(len(sortedCandles))
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	_ = model.AppDB.AutoMigrate(&model.Setting{})
	_ = model.AppDB.AutoMigrate(&model.Order{})
	_ = model.AppDB.AutoMigrate(&model.Balance{})
	_ = configor.Load(model.AppConfig, "./config.yml")
	//start, _ := time.Parse(time.RFC3339, `2022-10-01T00:00:00+00:00`)
	//end, _ := time.Parse(time.RFC3339, `2022-10-02T00:00:00+00:00`)
	//setting := &model.Setting{Market: model.Ftx, Symbol: `RVN_PERP`, AmountLimit: 3, GridAmount: 10000}
	//regret.ProcessCandles(setting.Market, setting.Symbol, start, end, setting)
	marketInfos := api.GetMarketInfos(model.BinancePerp)
	model.SetMarketInfos(model.BinancePerp, marketInfos)
	order := api.PlaceOrder(model.AppConfig.BinanceKey, model.AppConfig.BinanceSecret, model.OrderSideSell,
		model.OrderTypeStop, model.BinancePerp, `ETH_PERP`, ``, 1222, 1255, 0.1,
		false, nil, nil)
	fmt.Println(order.OrderId)
	api.CancelOrder(model.AppConfig.BinanceKey, model.AppConfig.BinanceSecret, model.BinancePerp, `ETH_PERP`, ``, order.OrderId)
	//today, _ := model.GetMarketToday(model.BinancePerp)
	duration, _ := time.ParseDuration(fmt.Sprintf(`%dh`, -24))
	day := today.Add(duration)
	candle := api.GetTurtleCandle(model.AppConfig.BinanceKey, model.AppConfig.BinanceSecret, model.BinancePerp,
		`BTC_PERP`, 86400, day)
	fmt.Println(candle.Begin)
	marketInfos = api.GetMarketInfos(model.BinancePerp)
	marketInfoArray := model.MarketInfoArray{}
	for _, info := range marketInfos {
		marketInfoArray = append(marketInfoArray, info)
	}
	sort.Sort(sort.Reverse(marketInfoArray))
	for _, info := range marketInfoArray {
		_, _, coin, _ := model.GetFromStandard(model.BinancePerp, info.Name)
		if !model.CommonCoins[strings.ToLower(coin)] {
			fmt.Println(fmt.Sprintf(`%s %f`, coin, info.TradeAmount))
		}
	}
	_, balances, total, collateral := api.GetBalances(model.AppConfig.GateKey, model.AppConfig.GateSecret, model.Gate)
	fmt.Println(collateral)
	fmt.Println(total)
	for _, balance := range balances {
		if balance.Amount > 0 {
			fmt.Println(fmt.Sprintf(`%s %f %f`, balance.Coin, balance.Amount, balance.UsdValue))
		}
	}
	accounts := make([]*model.Account, 0)
	account := model.AppConfig.GetAccounts(model.BinancePerp)[0]
	if account == nil {
		fmt.Println(`right`)
	} else {
		fmt.Println(`wrong` + account.Key)
	}
	accounts = append(accounts, account)
	accounts = append(accounts, nil)
	fmt.Println(len(accounts))
	result, _, _ := api.CancelOrder(model.AppConfig.GateKey, model.AppConfig.GateSecret, `gate`, `SUN_USDT`,
		model.OrderTypeLimit, `86007650678`)
	fmt.Println(result)
	testOrder := api.QueryOrderById(model.AppConfig.FtxKey, model.AppConfig.FtxSecret, model.Ftx, ``, model.OrderTypeLimit, `82424115039`)
	fmt.Println(testOrder.Market)
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	//api.GetFundingRate(model.Binance, `BTCUSDT`, nil)
	api.InitMarketInfos()
}

func downList(workId, pageId int) (fileNum int) {
	targetUrl := fmt.Sprintf(`https://user-api.foundingaz.com/api/submit-work/student-works?work_id=%d&page=%d&limit=1000`, workId, pageId)
	headers := map[string]string{`Accept`: `application/json, text/plain, */*`, `Origin`: `https://xuexi.cyjiaomu.com`,
		`token`: `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ0b2tlbi1kYXRhIjoieDlFdWd0ZmhmLVZRRWxoMmhSV2pTQ09XWHkxNUJxZnBJZE1Jc2FFZy13YS13MEV5UHV1LWNyYktFVmZuc3FNMHk1ZHB3LVpvanJkd1N4Li0uRjRmcGoxbUlZOGhabWV2V2RPTDR1Q21NZFVaLWtxS3JUdkNqVnVtTE8uQ0JYUGlBQlBDcmRMYXN5Ry1RNFFsNXdGRWNhIn0.vwFkT2FBH4ck3KprFyfuxJxSdL29TQGxa09KvgvOjR4`}
	response, _ := util.HttpRequest(http.MethodGet, targetUrl, ``, headers, 200)
	json, _ := util.NewJSON(response)
	list := json.GetPath(`data`, `list`).MustArray()
	for _, value := range list {
		item := value.(map[string]interface{})[`file_list`]
		if item == nil {
			continue
		}
		files := item.([]interface{})
		for _, file := range files {
			fileValue := file.(map[string]interface{})[`content`]
			if fileValue != nil {
				if strings.Contains(strings.ToLower(fileValue.(string)), `review`) {
					util.Notice(fmt.Sprintf(`%s`, fileValue))
					fmt.Println(fileValue.(string))
					fileNum++
				}
			}
		}
	}
	return
}

func Test_download(t *testing.T) {
	for workId := 51; workId > 0; workId-- {
		//fmt.Println(fmt.Sprintf(`try work %d`, workId))
		for pageId := 1; pageId < 20; pageId++ {
			files := downList(workId, pageId)
			if files <= 0 {
				break
			}
			//fmt.Println(fmt.Sprintf(`get revew %d from work %d page %d`, files, workId, pageId))
		}
	}
	//curl 'https://user-api.foundingaz.com/api/submit-work/student-works?work_id=51&class_id=0&class_name=&group_id=0&group_name=&page=1&limit=10' \
	//-H 'Referer: https://xuexi.cyjiaomu.com/' \
	//-H 'Host: user-api.foundingaz.com' \
	//-H 'User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.1 Safari/605.1.15' \
	//-H 'Accept-Language: zh-CN,zh-Hans;q=0.9' \
	//-H 'Accept-Encoding: gzip, deflate, br' \
	//-H 'Connection: keep-alive' \
}

func Test_wallet(t *testing.T) {
	model.NewConfig()
	var key, secret string
	market := model.BinanceSpot
	switch market {
	case model.Ftx:
		key = model.AppConfig.FtxKey
		secret = model.AppConfig.FtxSecret
	case model.OKEX:
		marketInfos := api.GetMarketInfos(model.OKEX)
		model.SetMarketInfos(model.OKEX, marketInfos)
		key = model.AppConfig.OkexKey
		secret = model.AppConfig.OkexSecret
	case model.BinancePerp:
		key = model.AppConfig.BinanceKey
		secret = model.AppConfig.BinanceSecret
	case model.BybitPerp:
		key = model.AppConfig.BybitKey
		secret = model.AppConfig.BybitSecret
	case model.Gate:
		key = model.AppConfig.GateKey
		secret = model.AppConfig.GateSecret
	}
	success, price := api.GetPriceForce(key, secret, `BTC_USDT`, market)
	success, price = api.GetPriceForce(key, secret, `BTC_USDT`, market)
	fmt.Println(fmt.Sprintf(`%v %f`, success, price))
	symbol := `HNT_PERP`
	//order := api.PlaceOrder(key, secret, model.OrderSideBuy, model.OrderTypeStop,
	//	market, symbol, ``, 4444, 4444, 0.1, false, nil, nil)
	//fmt.Println(order.OrderId)
	//orders := api.QueryOpenTriggerOrders(key, secret, market, symbol)
	//for _, m := range orders {
	//	result, _, _ := api.CancelOrder(key, secret, market, symbol, m.OrderType, m.OrderId)
	//	fmt.Println(result)
	//}
	_, rate, _ := api.GetFundingRate(key, secret, model.BybitPerp, symbol)
	_, rate, _ = api.GetFundingRate(key, secret, model.BybitPerp, `LOOKS_PERP`)
	marketInfos := api.GetMarketInfos(model.BybitPerp)
	model.SetMarketInfos(model.BybitPerp, marketInfos)
	//// 1078113554871236864
	////cancelResult := api.CancelOrders(key, secret, model.BybitSpot, `ETH-USDT`)
	////fmt.Println(cancelResult)
	//orderBybit = api.QueryOrderById(key, secret, model.BybitPerp, `ETH-PERP`, `ETH-PERP`,
	//	model.OrderTypeLimit, orderBybit.OrderId)
	//fmt.Println(orderBybit.OrderId)
	api.CancelOrder(key, secret, model.BybitPerp, `ETH-PERP`, model.OrderTypeLimit,
		`d490a639-a5f7-499a-9248-142a93ddaf13`)
	orderBybit1 := api.QueryOrderById(key, secret, model.BybitPerp, `ETH-PERP`,
		model.OrderTypeLimit, `d490a639-a5f7-499a-9248-142a93ddaf13`)
	fmt.Println(orderBybit1.OrderId)
	fmt.Println(fmt.Sprintf(`%f`, rate))
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	api.InitMarketInfos()
	orderQuery := api.QueryOrderById(model.AppConfig.GateKey, model.AppConfig.GateSecret, model.Gate, `CFX_PERP`, model.OrderTypeLimit, `79852794326`)
	fmt.Println(orderQuery.OrderSide)
	order1 := api.PlaceOrder(model.AppConfig.GateKey, model.AppConfig.GateSecret, model.OrderSideBuy, model.OrderTypeLimit,
		model.Gate, `ETH_USDT`, ``,
		2000, 2000, 0.1, false, nil, nil)
	fmt.Println(order1.OrderId)
	api.CancelOrders(model.AppConfig.GateKey, model.AppConfig.GateSecret, model.Gate, `ETH_USDT`)
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	api.InitMarketInfos()
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	api.InitMarketInfos()
	fmt.Println(order1.DealAmount)
	fmt.Println(order1.DealPrice)
	fmt.Println(order1.Status)
}

func Test_accounting(t *testing.T) {
	nums := strings.Split(`230.0,663.00,0,200.0,303.0,300.00,0`, `,`)
	season := make([]float64, 4)
	item := make([]float64, 3)
	result := make([][]float64, 3)
	for i := 0; i < 3; i++ {
		item[i], _ = strconv.ParseFloat(nums[i], 64)
	}
	for i := 3; i < 7; i++ {
		season[i-3], _ = strconv.ParseFloat(nums[i], 64)
	}
	for i := 0; i < len(item); i++ {
		result[i] = make([]float64, len(season))
		itemAll := 0.0
		for j := 0; j < len(season); j++ {
			result[i][j] = math.Min(math.Min(item[i]/float64(len(season)), season[j]), item[i]-itemAll)
			season[j] -= result[i][j]
			itemAll += result[i][j]
		}
		for j := 0; j < len(season); j++ {
			if itemAll < item[i] {
				amount := math.Min(item[i]-itemAll, season[j])
				itemAll += amount
				result[i][j] += amount
				season[j] -= amount
			}
		}
	}
	for _, value := range result {
		for j := 0; j < len(value); j++ {
			fmt.Println(fmt.Sprintf(`%d季度： %.2f %.2f %.2f`, j+1, value[j]*0.2, value[j]*0.3, value[j]*0.5))
		}
		fmt.Println()
	}
}
