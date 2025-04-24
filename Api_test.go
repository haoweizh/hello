package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"github.com/jinzhu/configor"
	"github.com/robfig/cron/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"hello/api"
	"hello/carry/cross"
	"hello/model"
	"hello/regret"
	"hello/util"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func Test_chan(t *testing.T) {
	testChan := make(chan int, 10)
	testChan <- 1
	testChan <- 2
	for {
		select {
		case a := <-testChan:
			fmt.Println(a)
		default:
			fmt.Println(`default`)
			time.Sleep(time.Second)
		}
	}
}

func Test_Com(t *testing.T) {
	src := `0x00000000458cEec48586a85fCFEb4A179706656eE321730E`
	samples := make([]string, 10)
	for i := range samples {
		index := rand.Intn(len(src))
		samples[i] = src[index:] + src[0:index]
	}
	reg := regexp.MustCompile("^0x0{8}")
	if reg.MatchString(src) {
		fmt.Println(fmt.Sprintf(`match %s`, reg.String()))
	}
	begin := time.Now().UnixMilli()
	for i := 0; i < 10000; i++ {
		for j := 0; j < len(samples); j++ {
			if reg.MatchString(samples[j]) {

			}
		}
	}
	end := time.Now().UnixMilli()
	fmt.Println(fmt.Sprintf(`regression time %d`, end-begin))

	begin = time.Now().UnixMilli()
	for i := 0; i < 10000; i++ {
		for j := 0; j < len(samples); j++ {
			reg.MatchString(samples[j])
			if samples[j][:10] == "^0x00000000" {

			}
		}
	}
	end = time.Now().UnixMilli()
	fmt.Println(fmt.Sprintf(`str comp time %d`, end-begin))
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
	model.NewConfig()
	market := model.Bybit
	account := model.AppConfig.GetAccounts(market)[0]
	//api.QueryOrderById(account, account.Market, `COW_PERP`, model.OrderTypeLimit, `0ee81f8d-b2fb-4e88-8b45-2a3b9e25f224`)
	//api.GetPositions(account, market)
	//api.GetInterest(account)
	api.GetBalances(account, market)
	//api.InitMarketInfos(market)
	//_, _, total, _ := api.GetBalances(account, market)
	//fmt.Println(total)
	//api.InitMarketInfos(market)
	//model.MarketInfos.Range(func(key, value any) bool {
	//	if value != nil && value.(*model.MarketInfo).PriceIncrement < 0.0000001 {
	//		fmt.Println(key.(string))
	//	}
	//	return true
	//})
	//price, decimal := model.FormatPrice(model.BinancePerp, `SOL_PERP`, 19.407125)
	//priceStr := util.CutTailZero(strconv.FormatFloat(price, 'f', decimal, 64))
	//fmt.Println(priceStr)
	//order1 := api.PlaceOrder(model.AppConfig.BinanceKey, model.AppConfig.BinanceSecret, model.OrderSideBuy, model.OrderTypeLimit,
	//	model.BinancePerp, `SOL_PERP`, ``, `test`,
	//	19.407125, 19.407125, 100, false, nil)
	//fmt.Println(order1.OrderId)
	//success, pos, value, u, _ := api.GetPositions(model.AppConfig.GateKey, model.AppConfig.GateSecret, model.Gate)
	//fmt.Println(fmt.Sprintf(`%#v %#v %#v %#v`, success, pos, value, u))
}

func TestWs(t *testing.T) {
	markets := []string{`a`, `b`, `c`, `d`, `e`, `f`, `g`, `h`, `i`, `j`}
	for {
		done := true
		for _, market := range markets {
			if market == `f` {
				done = false
				break
			}
			fmt.Println(market)
		}
		if done {
			break
		}
		fmt.Println(`done`)
		time.Sleep(time.Second)
	}
	fmt.Println(markets)
	market := model.Bybit
	model.NewConfig()
	//api.MaintainConns(market)
	//model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	//api.InitMarketInfos()
	api.CreateWSTick(model.AppEnvironment, market)
	select {}
}

func Test_WsAndOrderApi(t *testing.T) {
	market := model.Gate
	coin := `ETH`
	orderType := model.OrderTypeLimit
	orderSide := model.OrderSideSell
	symbols := []string{coin + model.UniStandardTail[model.MarketTypePerp]}
	//coin + model.UniStandardTail[model.MarketTypeSpot]}
	model.NewConfig()
	api.SendMails(`test`, `code`)
	//model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	api.InitMarketInfos(market)
	account := model.AppConfig.GetAccounts(market)[0]
	api.CreateWSTick(model.AppEnvironment, market)
	for _, symbol := range symbols {
		api.CancelOrders(account, market, symbol)
		getTick := false
		var tick *model.BidAsk
		for !getTick {
			time.Sleep(time.Minute * 5)
			getTick, tick = model.AppEnvironment.GetBidAsk(market, symbol)
		}
		if tick == nil {
			continue
		}
		price := tick.Bids[len(tick.Bids)-1].Price * 1.05
		amount := 20 / price
		order := api.PlaceOrder(account, orderSide, orderType, market,
			symbol, ``, ``, `test`, ``, price, price, amount, false, nil)
		fmt.Println(fmt.Sprintf(`1. place order return %#v`, order))
		if order != nil && order.OrderId != `` {
			queryOrder := api.QueryOrderById(account, market, symbol, orderType, order.OrderId)
			fmt.Println(fmt.Sprintf(`2. query order %s return %s %s %#v`, order.OrderId, queryOrder.OrderId, queryOrder.Status, queryOrder))
		} else {
			fmt.Println(fmt.Sprintf(`1. fail to place order`))
			continue
		}
		//cancelResult, errCode, errMsg, cancelOrder := api.CancelOrder(account.Key, account.Secret, market, symbol,
		//	orderType, order.OrderId)
		//fmt.Println(fmt.Sprintf(`3. cancel %s return %#v %s %s %#v`, order.OrderId, cancelResult, errCode, errMsg, cancelOrder))
		//queryOrder := api.QueryOrderById(account.Key, account.Secret, market, symbol, orderType, order.OrderId)
		//fmt.Println(fmt.Sprintf(`4. query order %s return %s %s %#v`, order.OrderId, queryOrder.OrderId, queryOrder.Status, queryOrder))
		//order1 := api.PlaceOrder(account.Key, account.Secret, orderSide, orderType, market,
		//	symbol, ``, ``, price, price, amount, false, true, nil, nil)
		//order2 := api.PlaceOrder(account.Key, account.Secret, orderSide, orderType, market,
		//	symbol, ``, ``, price, price, amount, false, true, nil, nil)
		//fmt.Println(fmt.Sprintf(`5. place order return %#v %#v`, order1, order2))
		//api.PlacePairOKEX(account, `requestId`, symbol, symbol, model.OrderTypeLimit, price*0.9, price*1.1, amount)
		api.CancelOrders(account, market, symbol)
		//if order1 != nil {
		//	time.Sleep(time.Second)
		//	queryOrder = api.QueryOrderById(account.Key, account.Secret, market, symbol, orderType, order1.OrderId)
		//	fmt.Println(fmt.Sprintf(`6. query order %s return %s %s %#v`, order1.OrderId, queryOrder.OrderId, queryOrder.Status, queryOrder))
		//}
		//if order2 != nil {
		//	queryOrder = api.QueryOrderById(account.Key, account.Secret, market, symbol, orderType, order2.OrderId)
		//	fmt.Println(fmt.Sprintf(`6. query order %s return %s %s %#v`, order2.OrderId, queryOrder.OrderId, queryOrder.Status, queryOrder))
		//}
	}
	select {}
}

func Test_Redis(t *testing.T) {
	model.NewConfig()
	_, _, _, symbol := model.GetFromStandard(model.Gate, `BTC`)
	fmt.Println(symbol)
	//model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	model.AppRedis = redis.NewClient(&redis.Options{
		Addr:     model.AppConfig.RedisAddr,
		Password: model.AppConfig.RedisPassword,
		DB:       0,
	})
	//model.AppRedis.Set(context.Background(), `test234`, `string(responseBody)`, 0)
	temp, redisErr := model.AppRedis.Get(context.Background(), `okex_ARB_PERP_1H_1699027200000_1699747200000_200`).Result()
	if redisErr != nil {
		fmt.Println(redisErr.Error())
	} else {
		fmt.Println(temp)
	}
}

func Test_Sync(t *testing.T) {
	c := cron.New()
	_, err := c.AddFunc("0,30,48 * * * ?", func() {
		fmt.Println("sync success" + time.Now().String())
	})
	if err != nil {
		fmt.Println(`fail to cron clear cross ` + err.Error())
	} else {
		c.Start()
	}
	select {}
	//model.NewConfig()
	//market := model.Gate
	//account := model.AppConfig.GetAccounts(market)[0]
	//api.GetPositions(account.Key, account.Secret, market)
	//model.AppEnvironment.PriConnecting.Store(model.BitgetPerp+account.Key, false)
	//api.WsOrderServeBybit(account)
	//select {
	//case <-time.After(time.Second * 160):
	//
	//}
}

func Test_BalAndPos(t *testing.T) {
	model.NewConfig()
	market := model.Gate
	api.InitMarketInfos(market)
	account := model.AppConfig.GetAccounts(market)[0]
	api.GetPositions(account, market)
	markets := []string{`okex`, `binancespot`, `gate`, `binanceperp`, `bybit`}
	api.InitCrossMarketInfos(markets, false)
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	account = model.AppConfig.GetAccounts(model.BinancePerp)[0]
	model.AppRedis.Set(context.Background(), `test`, `11`, 0)
	temp, err := model.AppRedis.Get(context.Background(), `test`).Result()
	if err == nil {
		fmt.Println(fmt.Sprintf(`%s`, temp))
	}
	api.GetMarkPrice(account, model.BinancePerp, `ALGO_PERP`)
	//api.GetTurtleData(model.AppConfig.OkexKey, model.AppConfig.OkexSecret, model.FunctionTurtle, model.OKEX, `MATIC_PERP`)
	order := api.QueryOrderById(account, `gate`, `MGA_USDT`, model.OrderTypeLimit, `144149811503`)
	fmt.Println(order)
	//for _, market := range balMarkets {
	//	account := model.AppConfig.GetAccounts(market)[0]
	//	success, balances, total, collateral := api.GetBalances(account.Key, account.Secret, market)
	//	fmt.Println(fmt.Sprintf(`%#v %f %#v %d`, success, total, collateral, len(balances)))
	//	for _, balance := range balances {
	//		if balance.Coin == `USDT` || balance.Coin == `USD` {
	//			fmt.Println(fmt.Sprintf(`usd amount %s %f`, market, balance.Amount))
	//		}
	//	}
	//}
	posMarkets := []string{model.Bybit}
	//posMarkets := []string{model.OKEX, model.BybitPerp, model.Ftx}
	for _, market := range posMarkets {
		account = model.AppConfig.GetAccounts(market)[0]
		success, positions, total, available, _ := api.GetPositions(account, market)
		fmt.Println(fmt.Sprintf(`%#v %f %f %d`, success, total, available, len(positions)))
		for _, position := range positions {
			fmt.Println(fmt.Sprintf(`%s %f`, position.Currency, position.Holding))
			api.CancelOrders(account, market, position.Currency)
		}
		success, positions, total, available, _ = api.GetPositions(account, market)
		for _, position := range positions {
			fmt.Println(fmt.Sprintf(`%s %f`, position.Currency, position.Holding))
			api.CancelOrders(account, market, position.Currency)
		}
	}
}

func Test_DealGridSimulate(t *testing.T) {
	model.NewConfig()
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	funcNames := []string{
		`1_60_14400_2023-04-09T00:00:00+00:00_2024-04-09T00:00:00+00:00`,
		`1_20_86400_2023-04-09T00:00:00+00:00_2024-04-09T00:00:00+00:00`,
		`1.05_60_14400_2023-04-09T00:00:00+00:00_2024-04-09T00:00:00+00:00`,
		`1.05_20_86400_2023-04-09T00:00:00+00:00_2024-04-09T00:00:00+00:00`,
		`1.1_60_14400_2023-04-09T00:00:00+00:00_2024-04-09T00:00:00+00:00`,
		`1.1_20_86400_2023-04-09T00:00:00+00:00_2024-04-09T00:00:00+00:00`,
		`1.15_60_14400_2023-04-09T00:00:00+00:00_2024-04-09T00:00:00+00:00`,
		`1.15_20_86400_2023-04-09T00:00:00+00:00_2024-04-09T00:00:00+00:00`,
	}
	coins := []string{`1000LUNC`, `1000SHIB`, `ADA`, `AGIX`, `APE`, `API3`, `APT`, `ARB`, `ATOM`, `AVAX`, `BLZ`, `CFX`, `CKB`, `COMP`, `CRV`, `DOGE`, `DOT`, `DYDX`, `ENS`, `FET`, `FIL`, `FTM`, `GALA`, `GMT`, `ICP`, `ID`, `INJ`, `LEVER`, `LINA`, `LPT`, `LUNA2`, `MASK`, `MATIC`, `MKR`, `NEAR`, `OGN`, `OP`, `PEOPLE`, `RNDR`, `RUNE`, `SOL`, `STMX`, `STORJ`, `STX`, `TRB`, `UNI`, `ZRX`, `BTC`, `ETH`}
	symbols := make([]string, len(coins))
	for i, coin := range coins {
		symbols[i] = coin + `_PERP`
	}
	result := make(map[string]map[string]map[string]string)
	for _, symbol := range symbols {
		for _, funcName := range funcNames {
			orders := make([]*model.Order, 0)
			model.AppDB.Where(`market=? and refresh_type=? and function=? and symbol=? and grid_pos=?`,
				model.BinancePerp, model.FunctionSimulation, funcName, symbol, 0).Order(`order_time desc`).Limit(1).Find(&orders)
			if len(orders) > 0 {
				delNum := model.AppDB.Where(`market=? and refresh_type=? and function=? and symbol=? and order_time>?`,
					model.BinancePerp, model.FunctionSimulation, funcName, symbol, orders[0].OrderTime).Delete(&model.Order{}).RowsAffected
				if delNum > 0 {
					fmt.Println(fmt.Sprintf(`cut %s tail num %d`, symbol, delNum))
				}
			} else {
				util.Log(util.LogLevelError, fmt.Sprintf(`can not get orders from %s %s grid 0`, model.BinancePerp, symbol))
			}
			rows, _ := model.AppDB.Model(model.Order{}).Select(`symbol,order_side,sum(orders.deal_price*amount)/sum(amount),sum(amount)`).
				Where(`function=? and symbol=?`, funcName, symbol).Group(`function,symbol,order_side`).Rows()
			for rows.Next() {
				var orderSide string
				var price, amount float64
				_ = rows.Scan(&symbol, &orderSide, &price, &amount)
				if result[symbol] == nil {
					result[symbol] = make(map[string]map[string]string)
				}
				if result[symbol][funcName] == nil {
					result[symbol][funcName] = make(map[string]string)
				}
				result[symbol][funcName][orderSide] = fmt.Sprintf(`%f,%f,%f`, price, amount, amount*price)
			}
		}
	}
	for symbol, funcMap := range result {
		for _, funcName := range funcNames {
			msg := fmt.Sprintf(`,%s,%s,%s,%s`, symbol, funcName, funcMap[funcName][`buy`], funcMap[funcName][`sell`])
			util.InfoSync(msg)
		}
	}
}

func Test_CreateReport(t *testing.T) {
	model.NewConfig()
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	//begin, _ := time.Parse(time.RFC3339, `2019-01-01T00:00:00+08:00`)
	//end, _ := time.Parse(time.RFC3339, `2023-01-01T00:00:00+08:00`)
	//timeRage := `2021-07-01T00:00:00+00:00~2023-07-01T00:00:00+00:00`
	//coins := `CZCE.FG,DCE.jm,DCE.eb,CZCE.TA,SHFE.fu,DCE.p,CZCE.SF,SHFE.hc,DCE.v,DCE.y`
	//coins := `DOGE,SOL,MATIC,CHZ,LINK,ADA,BNB,FIL,SUSHI,AXS,ATOM,WAVES`
	//for _, s := range strings.Split(coins, `,`) {
	coins := []string{`1000LUNC`, `1000SHIB`, `ADA`, `AGIX`, `APE`, `API3`, `APT`, `ARB`, `ATOM`, `AVAX`, `BLZ`, `CFX`, `CKB`, `COMP`, `CRV`, `DOGE`, `DOT`, `DYDX`, `ENS`, `FET`, `FIL`, `FTM`, `GALA`, `GMT`, `ICP`, `ID`, `INJ`, `LEVER`, `LINA`, `LPT`, `LUNA2`, `MASK`, `MATIC`, `MKR`, `NEAR`, `OGN`, `OP`, `PEOPLE`, `RNDR`, `RUNE`, `SOL`, `STMX`, `STORJ`, `STX`, `TRB`, `UNI`, `ZRX`, `BTC`, `ETH`}
	funcNames := []string{
		`1_60_14400_2023-04-09T00:00:00+00:00_2024-04-09T00:00:00+00:00`,
		`1_20_86400_2023-04-09T00:00:00+00:00_2024-04-09T00:00:00+00:00`,
		`1.05_60_14400_2023-04-09T00:00:00+00:00_2024-04-09T00:00:00+00:00`,
		`1.05_20_86400_2023-04-09T00:00:00+00:00_2024-04-09T00:00:00+00:00`,
		`1.1_60_14400_2023-04-09T00:00:00+00:00_2024-04-09T00:00:00+00:00`,
		`1.1_20_86400_2023-04-09T00:00:00+00:00_2024-04-09T00:00:00+00:00`,
		`1.15_60_14400_2023-04-09T00:00:00+00:00_2024-04-09T00:00:00+00:00`,
		`1.15_20_86400_2023-04-09T00:00:00+00:00_2024-04-09T00:00:00+00:00`,
	}
	for _, function := range funcNames {
		regret.CreateReport(function, coins)
	}
	//coins = `BTC`
	//regret.CreateReport(market, coins, timeRage, `86400`)
	//regret.CreateReport(market, coins, timeRage, `14400`)
	//coins = `ETH`
	//regret.CreateReport(market, coins, timeRage, `86400`)
	//regret.CreateReport(market, coins, timeRage, `14400`)
}

func Test_CutTail(t *testing.T) {
	model.NewConfig()
	api.SendMails(`test`, `test`)
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	coins := `1000SHIB,SOL,DOGE,MATIC,ADA,TRB,FIL,DYDX,FTM,AVAX,DOT,MASK,NEAR,BLZ,MKR,GALA,STORJ,ATOM,RUNE,CRV`
	//allLimit := 12
	market := model.BinancePerp
	strBegin := `2023-02-22T00:00:00+00:00`
	strEnd := `2024-2-22T00:00:00+00:00`
	sign := fmt.Sprintf(`market%s,coins%s,%s~%s,far%d,near%d,limit%d,allLimit%d,useNear%#v,useM%#v,seconds86400`,
		market, coins, strBegin, strEnd, 18, 9, 3, 10, true, false)
	regret.CutTail(market, coins, sign)
	sign = fmt.Sprintf(`market%s,coins%s,%s~%s,far%d,near%d,limit%d,allLimit%d,useNear%#v,useM%#v,seconds14400`,
		market, coins, strBegin, strEnd, 50, 25, 3, 10, true, false)
	regret.CutTail(market, coins, sign)
	coins = `BTC`
	sign = fmt.Sprintf(`market%s,coins%s,%s~%s,far%d,near%d,limit%d,allLimit%d,useNear%#v,useM%#v,seconds86400`,
		market, coins, strBegin, strEnd, 22, 11, 3, 3, true, false)
	regret.CutTail(market, coins, sign)
	sign = fmt.Sprintf(`market%s,coins%s,%s~%s,far%d,near%d,limit%d,allLimit%d,useNear%#v,useM%#v,seconds14400`,
		market, coins, strBegin, strEnd, 50, 25, 3, 3, true, false)
	regret.CutTail(market, coins, sign)
	coins = `ETH`
	sign = fmt.Sprintf(`market%s,coins%s,%s~%s,far%d,near%d,limit%d,allLimit%d,useNear%#v,useM%#v,seconds86400`,
		market, coins, strBegin, strEnd, 18, 9, 3, 3, true, false)
	regret.CutTail(market, coins, sign)
	sign = fmt.Sprintf(`market%s,coins%s,%s~%s,far%d,near%d,limit%d,allLimit%d,useNear%#v,useM%#v,seconds14400`,
		market, coins, strBegin, strEnd, 32, 16, 3, 3, true, false)
	regret.CutTail(market, coins, sign)
	//coins := `CZCE.FG,DCE.jm,DCE.eb,CZCE.TA,SHFE.fu,DCE.p,CZCE.SF,SHFE.hc,DCE.v,DCE.y`
	//coins := `DOGE,SOL,MATIC,CHZ,LINK,ADA,BNB,FIL,SUSHI,AXS,ATOM,WAVES`
	//coinNames := strings.Split(`CZCE.CY,CZCE.FG,CZCE.MA,CZCE.OI,CZCE.PF,CZCE.RM,CZCE.SA,CZCE.SF,CZCE.SM,CZCE.SR,CZCE.TA,CZCE.UR,CZCE.ZC,DCE.c,DCE.eb,DCE.eg,DCE.i,DCE.j,DCE.jm,DCE.l,DCE.m,DCE.p,DCE.pp,DCE.v,DCE.y,SHFE.bu,SHFE.cu,SHFE.fu,SHFE.hc,SHFE.pb,SHFE.rb,SHFE.ru`, `,`)
	//for i := 3; i <= 25; i++ {
	//	//coins = `ETH`
	//	//sign := fmt.Sprintf(`market%s,coins%s,%s~%s,far%d,near%d,limit%d,allLimit%d,useNear%#v,useM%#v,seconds86400`,
	//	//	market, coins, strBegin, strEnd, i*2, i, 3, 10, true, false)
	//	//regret.CutTail(market, coins, sign)
	//	sign := fmt.Sprintf(`market%s,coins%s,%s~%s,far%d,near%d,limit%d,allLimit%d,useNear%#v,useM%#v,seconds14400,20`,
	//		market, coins, strBegin, strEnd, i*2, i, 3, 10, false, false)
	//	regret.CutTail(market, coins, sign)
	//	//sign = fmt.Sprintf(`market%s,coins%s,%s~%s,far%d,near%d,limit%d,allLimit%d,useNear%#v,useM%#v,seconds86400`,
	//	//	market, coins, strBegin, strEnd, i*2, i, 3, 10, false, true)
	//	//regret.CutTail(market, coins, sign)
	//	//coins = `BTC`
	//	//sign = fmt.Sprintf(`market%s,coins%s,%s~%s,far%d,near%d,limit%d,allLimit%d,useNear%#v,useM%#v,seconds14400`,
	//	//	market, coins, strBegin, strEnd, i*2, i, 3, 6, true, false)
	//	//regret.CutTail(market, coins, sign)
	//	//sign = fmt.Sprintf(`market%s,coins%s,%s~%s,far%d,near%d,limit%d,allLimit%d,useNear%#v,useM%#v,seconds14400`,
	//	//	market, coins, strBegin, strEnd, i*2, i, 3, 6, false, false)
	//	//regret.CutTail(market, coins, sign)
	//	//sign = fmt.Sprintf(`market%s,coins%s,%s~%s,far%d,near%d,limit%d,allLimit%d,useNear%#v,useM%#v,seconds14400`,
	//	//	market, coins, strBegin, strEnd, i*2, i, 3, 6, false, true)
	//	//regret.CutTail(market, coins, sign)
	//}
	fmt.Println(`done`)
}

func Test_initTurtleN(t *testing.T) {
	model.NewConfig()
	api.InitMarketInfos(model.BinanceSpot)
	market := model.OKEX
	account := model.AppConfig.GetAccounts(market)[0]
	//now := time.Now()
	//nowPeriod1, _ := model.GetNowPeriod(market, 86400, now)
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	settings := map[string]*model.Setting{`SHIB_PERP`: {Market: market, Symbol: `SHIB_PERP`}}
	api.GetMultiCandle(account, model.OKEX, 3600, time.Now().Add(time.Duration(-220)*time.Hour), time.Now(), settings, false)
	//api.CalcTurtleData(account, market, `GAS_PERP`, model.FunctionCombineTurtle, 18, 9, 86400,
	//	5, 3, 0.1, nowPeriod1)
	//fmt.Println(model.AppConfig.OKPhase)
	//orders := api.QueryOpenOrders(account.Key, account.Secret, market, `ARB_PERP`)
	//fmt.Println(len(orders))
	nowPeriod, _ := model.GetMarketToday(market)
	seconds := 14400
	candles := api.CombineCandles(account, market, `BTC_PERP`, seconds,
		nowPeriod.Add(time.Second*time.Duration(seconds*-1*30)), nowPeriod)
	fmt.Println(len(candles))
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	//model.AppRedis = redis.NewClient(&redis.Options{
	//	Addr:     model.AppConfig.RedisAddr,
	//	Password: model.AppConfig.RedisPassword,
	//	DB:       0,
	//})
	api.InitMarketInfos(model.Bybit)
	suc, bals, inU, cor := api.GetBalances(account, model.Bybit)
	fmt.Println(fmt.Sprintf(`%#v %f %#v`, suc, inU, cor))
	for _, bal := range bals {
		fmt.Println(bal.Coin)
		fmt.Println(bal.Amount)
	}
	//day := today.Add(time.Hour * -24)
	//candles := api.CalcCandleN(model.AppConfig.BinanceKey, model.AppConfig.BinanceSecret, model.BinancePerp, `BNX_PERP`, 86400, day)
	//fmt.Println(candles)
	//fmt.Println(len(sortedCandles))
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	_ = model.AppDB.AutoMigrate(&model.Setting{})
	_ = model.AppDB.AutoMigrate(&model.Order{})
	_ = model.AppDB.AutoMigrate(&model.Balance{})
	_ = configor.Load(model.AppConfig, "./config.yml")
	//start, _ := time.Parse(time.RFC3339, `2022-10-01T00:00:00+00:00`)
	//end, _ := time.Parse(time.RFC3339, `2022-10-02T00:00:00+00:00`)
	//setting := &model.Setting{Market: model.Ftx, Symbol: `RVN_PERP`, AmountLimit: 3, GridAmount: 10000}
	//regret.ProcessCandles(setting.Market, setting.Symbol, start, end, setting)
	//order := api.PlaceOrder(model.AppConfig.BinanceKey, model.AppConfig.BinanceSecret, model.OrderSideSell, model.OrderTypeStop,
	//	model.BinancePerp, `ETH_PERP`, ``, `test`, 1222, 1255, 0.1, false, nil)
	//fmt.Println(order.OrderId)
	//api.CancelOrder(model.AppConfig.BinanceKey, model.AppConfig.BinanceSecret, model.BinancePerp, `ETH_PERP`, ``, order.OrderId)
	//today, _ := model.GetMarketToday(model.BinancePerp)
	//candle := api.CalcCandleN(model.AppConfig.BinanceKey, model.AppConfig.BinanceSecret, model.BinancePerp,
	//	`BTC_PERP`, 86400, day)
	_, balances, total, collateral := api.GetBalances(account, model.Gate)
	fmt.Println(collateral)
	fmt.Println(total)
	for _, balance := range balances {
		if balance.Amount > 0 {
			fmt.Println(fmt.Sprintf(`%s %f %f`, balance.Coin, balance.Amount, balance.UsdValue))
		}
	}
	accounts := make([]*model.Account, 0)
	account = model.AppConfig.GetAccounts(model.BinancePerp)[0]
	if account == nil {
		fmt.Println(`right`)
	} else {
		fmt.Println(`wrong` + account.Key)
	}
	accounts = append(accounts, account)
	accounts = append(accounts, nil)
	fmt.Println(len(accounts))
	result, _, _ := api.CancelOrder(account, `gate`, `SUN_USDT`, model.OrderTypeLimit, `86007650678`)
	fmt.Println(result)
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
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
					util.Log(util.LogLevelInfo, fmt.Sprintf(`%s`, fileValue))
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

func Test_ClearActs(t *testing.T) {
	event := `{"id":"account-balance-v2-1737821112816","status":200,"result":[{"accountAlias":"FzXqFzAusRmYSgmY","asset":"FDUSD",
"balance":"0.00000000","crossWalletBalance":"0.00000000","crossUnPnl":"0.00000000","availableBalance":"9310.89505866","maxWithdrawAmount":"0.00000000",
"marginAvailable":true,"updateTime":0},{"accountAlias":"FzXqFzAusRmYSgmY","asset":"BFUSD","balance":"0.00000000","crossWalletBalance":"0.00000000","crossUnPnl":"0.00000000","availableBalance":"9399.20898925","maxWithdrawAmount":"0.00000000","marginAvailable":true,"updateTime":0},{"accountAlias":"FzXqFzAusRmYSgmY","asset":"BNB","balance":"2.33463468","crossWalletBalance":"2.33463468","crossUnPnl":"0.00000000","availableBalance":"13.02905711","maxWithdrawAmount":"2.33463468","marginAvailable":true,"updateTime":1737821040184},{"accountAlias":"FzXqFzAusRmYSgmY","asset":"ETH","balance":"0.00000000","crossWalletBalance":"0.00000000","crossUnPnl":"0.00000000","availableBalance":"2.68397842","maxWithdrawAmount":"0.00000000","marginAvailable":true,"updateTime":0},{"accountAlias":"FzXqFzAusRmYSgmY","asset":"BTC","balance":"0.00000000","crossWalletBalance":"0.00000000","crossUnPnl":"0.00000000","availableBalance":"0.08537030","maxWithdrawAmount":"0.00000000","marginAvailable":true,"updateTime":0},{"accountAlias":"FzXqFzAusRmYSgmY","asset":"USDT","balance":"12874.29522174","crossWalletBalance":"12874.29522174","crossUnPnl":"-327.13667425","availableBalance":"9407.66742508","maxWithdrawAmount":"9407.66742508","marginAvailable":true,"updateTime":1737821040184},{"accountAlias":"FzXqFzAusRmYSgmY","asset":"USDC","balance":"0.00000000","crossWalletBalance":"0.00000000","crossUnPnl":"0.00000000","availableBalance":"9407.35389563","maxWithdrawAmount":"0.00000000","marginAvailable":true,"updateTime":0}],"rateLimits":[{"rateLimitType":"REQUEST_WEIGHT","interval":"MINUTE","intervalNum":1,"limit":2400,"count":5}]}`
	j, err := util.NewJSON([]byte(event))
	if err != nil {
		t.Error(err)
	}
	ja := j.Get(`result`).MustArray()
	fmt.Println(ja)
	for _, item := range ja {
		asset := item.(map[string]interface{})[`asset`].(string)
		if asset == `USDT` {
			balance, _ := strconv.ParseFloat(item.(map[string]interface{})[`balance`].(string), 64)
			crossUnPnl, _ := strconv.ParseFloat(item.(map[string]interface{})[`crossUnPnl`].(string), 64)
			availableBalance, _ := strconv.ParseFloat(item.(map[string]interface{})[`availableBalance`].(string), 64)
			fmt.Println(fmt.Sprintf("balance: %f %f %f", balance, crossUnPnl, availableBalance))
		}
	}
	model.NewConfig()
	market := model.OKEX
	api.InitMarketInfos(market)
	//account := model.AppConfig.GetAccounts(market)[0]
	//_, bals, _, _ := api.GetBalances(account.Key, account.Secret, market)
	//for _, bal := range bals {
	//	if strings.ToUpper(bal.Coin) == `USDT` {
	//		continue
	//	}
	//	price := api.GetPriceBitgetSpot(account, bal.Coin+`_USDT`)
	//	if price > 0 && price*bal.Amount > 0.3 {
	//		fmt.Println(fmt.Sprintf(`%s %f %f`, bal.Coin, bal.Amount, price))
	//		time.Sleep(time.Second)
	//		api.PlaceOrder(account, model.OrderSideSell, model.OrderTypeLimit, market, bal.Coin+`_USDT`,
	//			``, `test`, price, price, bal.Amount, false, nil)
	//		time.Sleep(time.Millisecond * 80)
	//	}
	//	time.Sleep(time.Millisecond * 70)
	//}
}

func Test_GetPrice(t *testing.T) {
	model.NewConfig()
	markets := []string{model.Bybit, model.Bybit, model.BinanceSpot, model.BinancePerp, model.Gate, model.Gate, model.OKEX, model.OKEX}
	symbols := []string{`BTC_USDT`, `BTC_PERP`, `BTC_USDT`, `BTC_PERP`, `BTC_PERP`, `BTC_USDT`, `BTC_PERP`, `BTC_USDT`, `BTC_PERP`, `BTC_USDT`}
	for i, market := range markets {
		result, price := api.GetPriceForce(market, symbols[i], true)
		fmt.Println(fmt.Sprintf(`%s %s %v %f`, market, symbols[i], result, price))
	}
}

func Test_Order(t *testing.T) {
	//market := model.Gate
	model.NewConfig()
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	cross.GetCoinTraded(0)
	//account := model.GetAccounts(0)[market]
	//api.InitMarketInfos(market)
	//go api.MaintainConns(market)
	//time.Sleep(5 * time.Second)
	//symbol := `SHELL_USDT`
	//order1 := api.PlaceOrder(account, model.OrderSideSell, model.OrderTypeLimit, market, symbol, ``, ``, `test`, ``,
	//	0.33, 0.33, 20, false, nil)
	//fmt.Println(fmt.Sprintf(`%#v`, order1))
	//go func() {
	//	for {
	//		for _, mk := range markets {
	//			api.RequireConnTickReset(model.AppEnvironment, mk)
	//		}
	//		time.Sleep(2 * time.Minute)
	//	}
	//}()
	//select {
	//case <-time.After(time.Second * 33):
	//
	//}
}

func Test_WSOKPair(t *testing.T) {
	model.NewConfig()
	market := model.OKEX
	//go model.AppEnvironment.HandleWSResp()
	api.MaintainConns(market)
	//symbol := `DOGE_USDT`
	//api.GetPositions(account.Key, account.Secret, market)
	//api.GetBalances(account.Key, account.Secret, market)
	//api.InitMarketInfos(market)
	//order := api.PlaceOrder(account.Key, account.Secret, model.OrderSideBuy, model.OrderTypeLimit, market, symbol, ``,
	//	`test`, 0.29, 0.29, 24.4, true, nil, nil)
	//requestId := fmt.Sprintf(`%d`, time.Now().UnixMilli())
	//api.PlacePairOKEX(model.GetAccounts(0)[market], requestId, `DOGE_USDT`, `DOGE_PERP`, model.OrderTypeLimit, 0.29, 0.55, 1000)
	//model.AppEnvironment.ReqIdOrders.Store(requestId+model.OrderSideBuy, &model.Order{OrderId: requestId + model.OrderSideBuy})
	//model.AppEnvironment.ReqIdOrders.Store(requestId+model.OrderSideSell, &model.Order{OrderId: requestId + model.OrderSideSell})
	// api.CreateWsOrderUpdate(model.AppEnvironment, market)
	// api.CreateWSTick(model.AppEnvironment, model.Gate)
	//fmt.Println(order)
	select {}
}

func Test_transferInner(t *testing.T) {
	model.NewConfig()
	market := model.OKEX
	account := model.AppConfig.GetAccounts(market)[0]
	suc, bals, total, _ := api.GetBalances(account, market)
	fmt.Println(fmt.Sprintf(`%#v total %f`, suc, total))
	for _, bal := range bals {
		api.TransferGate(model.AppConfig.GateKey, model.AppConfig.GateSecret, `MAIN_UMFUTURE`, bal.Coin, bal.Amount)
	}
	assets := api.GetCoinBalanceBybit(model.AppConfig.BybitKey, model.AppConfig.BybitSecret, `59372048`)
	for _, bal := range assets {
		if bal.Coin != `USDT` {
			amtStr := strconv.FormatFloat(bal.Amount, 'f', -1, 64)
			//suc := api.TransferInnerBybit(model.AppConfig.BybitKey, model.AppConfig.BybitSecret, bal.Coin,
			//	amtStr, `UNIFIED`, `FUND`)
			suc = api.WithdrawBybit(model.AppConfig.BybitKey, model.AppConfig.BybitSecret, bal.Coin,
				`65058810`, `65058810`, amtStr)
			if !suc {
				fmt.Println(fmt.Sprintf(`fail to transfer from FUND to UNIFIED %s %s`, bal.Coin, amtStr))
			} else {
				fmt.Println(fmt.Sprintf(`transfer %s %s success`, bal.Coin, amtStr))
			}
			time.Sleep(time.Second * 3)
		}
	}
}

func Test_LimitReport(t *testing.T) {
	_, _, _, symbol := model.GetFromStandard(model.BinanceSpot, `TKO_USDT`)
	fmt.Println(symbol)
	model.NewConfig()
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	order := &model.Order{
		DealAmount:  10,
		DealPrice:   11,
		ErrCode:     "1",
		OrderId:     "test1",
		ClientOrdId: "1730118883773671207BTC_PERP",
		Status:      "working",
		OrderTime:   time.Now(),
	}
	model.AppDB.Model(order).Where("client_ord_id = ?", order.ClientOrdId).Updates(map[string]interface{}{
		`deal_amount`: order.DealAmount, `deal_price`: order.DealPrice, `err_code`: order.ErrCode, `order_id`: order.OrderId, `status`: order.Status, `order_time`: order.OrderTime})

	market := model.OKEX
	rows, _ := model.AppDB.Model(&model.Order{}).Select(`symbol`).Where(`market=? and order_type=? and status=? and created_at>?`,
		market, model.OrderTypeLimit, model.CarryStatusSuccess, `2024-01-01`).Group(`symbol`).Rows()
	buyAmount := make(map[string]float64)
	buyU := make(map[string]float64)
	sellAmount := make(map[string]float64)
	sellU := make(map[string]float64)
	for rows.Next() {
		var symbol string
		err := rows.Scan(&symbol)
		time2024 := time.Unix(1704038400, 0)
		if err == nil {
			var order model.Order
			model.AppDB.Where(`market=? and order_type=? and status=? and created_at>? and symbol=? and function=?`,
				market, model.OrderTypeLimit, model.CarryStatusSuccess, `2024-01-01`, symbol, model.Close).
				Order(`created_at desc`).Limit(1).Find(&order)
			if order.CreatedAt.After(time2024) {
				var orders []model.Order
				model.AppDB.Where(`market=? and order_type=? and status=? and symbol=? and created_at<=?`,
					market, model.OrderTypeLimit, model.CarryStatusSuccess, symbol, order.CreatedAt).Order(`created_at desc`).Find(&orders)
				for _, value := range orders {
					if value.Function == model.Close && value.CreatedAt.Before(time2024) {
						break
					}
					if value.OrderSide == model.OrderSideBuy {
						buyAmount[symbol] += value.Amount
						buyU[symbol] += value.Amount * value.Price
					} else {
						sellAmount[symbol] += value.Amount
						sellU[symbol] += value.Amount * value.Price
					}
				}
			}
		}
	}
	for symbol := range buyAmount {
		if buyU[symbol] > 0 && sellAmount[symbol] > 0 && sellU[symbol] > 0 {
			util.InfoSync(fmt.Sprintf(`,%s,%s,%f,%f,%f,%f`, market, symbol, buyAmount[symbol], buyU[symbol], sellAmount[symbol], sellU[symbol]))
		}
	}
}

// step(n) = step(n-1) + base + 0.0001*(n-1)
func Test_CalcGridLine(t *testing.T) {
	cross.CalcGridLine(0.0014)
}

func Test_map(t *testing.T) {
	response, _ := util.HttpRequest(http.MethodGet, `https://www.binance.com/zh-CN/support/announcement/list/49`, ``, nil, 2000)
	//response, _ := util.HttpRequest(http.MethodPost, `https://www.google.com`, ``, nil, 2000)
	fmt.Println(string(response))
}

func Test_wallet(t *testing.T) {
	set := &model.Setting{
		Market: model.BinanceSpot,
	}
	set.GridAmount += 100
	fmt.Println(set.GridAmount)
	model.NewConfig()
	market := model.BinancePerp
	symbol := `BTC_PERP`
	account := model.GetAccounts(0)[market]
	_, _, rate := api.GetFundingRate(account, market, symbol, false)
	if rate != nil {
		fmt.Println(rate.ExpireTime)
		return
	}
	orderQuery0 := api.QueryOrderById(account, market, symbol, model.OrderTypeMarket, `1841956120781220352`)
	fmt.Println(orderQuery0.OrderId)
	success, price := api.GetPriceForce(market, `LDBNB_USDT`, false)
	success, price = api.GetPriceForce(market, `BTC_USDT`, false)
	fmt.Println(fmt.Sprintf(`%#v %f`, success, price))
	//order := api.PlaceOrder(key, secret, model.OrderSideBuy, model.OrderTypeStop,
	//	market, symbol, ``, 4444, 4444, 0.1, false, nil, nil)
	//fmt.Println(order.OrderId)
	//orders := api.QueryOpenTriggerOrders(key, secret, market, symbol)
	//for _, m := range orders {
	//	result, _, _ := api.CancelOrder(key, secret, market, symbol, m.OrderType, m.OrderId)
	//	fmt.Println(result)
	//}
	//// 1078113554871236864
	////cancelResult := api.CancelOrders(key, secret, model.BybitSpot, `ETH-USDT`)
	////fmt.Println(cancelResult)
	//orderBybit = api.QueryOrderById(key, secret, model.BybitPerp, `ETH-PERP`, `ETH-PERP`,
	//	model.OrderTypeLimit, orderBybit.OrderId)
	//fmt.Println(orderBybit.OrderId)
	api.CancelOrder(account, model.Bybit, `ETH-PERP`, model.OrderTypeLimit,
		`d490a639-a5f7-499a-9248-142a93ddaf13`)
	orderBybit1 := api.QueryOrderById(account, model.Bybit, `ETH-PERP`,
		model.OrderTypeLimit, `d490a639-a5f7-499a-9248-142a93ddaf13`)
	fmt.Println(orderBybit1.OrderId)
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	api.InitMarketInfos(model.Gate)
	orderQuery := api.QueryOrderById(account, model.Gate, `CFX_PERP`, model.OrderTypeLimit, `79852794326`)
	fmt.Println(orderQuery.OrderSide)
	//order1 := api.PlaceOrder(model.AppConfig.GateKey, model.AppConfig.GateSecret, model.OrderSideBuy, model.OrderTypeLimit,
	//	model.Gate, `ETH_USDT`, ``, `test`, 2000, 2000, 0.1, false, nil)
	//fmt.Println(order1.OrderId)
	api.CancelOrders(account, model.Gate, `ETH_USDT`)
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	api.InitMarketInfos(model.OKEX)
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
}

func Test_transfer(t *testing.T) {
	timeStr := "2023-04-05 15:04:05"
	// 定义一个用于接收解析结果的time.Time变量
	//t1 := time.Time{}
	// 解析字符串为time.Time
	t1, err := time.Parse("2006-01-02 15:04:05", timeStr)
	if err != nil {
		fmt.Printf("解析时间失败: %#v\n", err)
		return
	}
	fmt.Printf("解析得到的时间: %#v\n", t1)
	response, _ := util.HttpRequest(http.MethodPost, `https://user.api.it120.cc/user/apiExtUserCash/list`,
		`page=1&pageSize=50&mobile=19525266383&aggregate=`, map[string]string{`x-token`: `7404f54e-4675-48ee-94bc-113e772c96ed`,
			`Content-Type`: `application/x-www-form-urlencoded`}, 10000)
	fmt.Println(string(response))
}

func Test_GateSols(t *testing.T) {
	model.NewConfig()
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	api.PrepareSettings()
	rows, _ := model.AppDB.Model(&model.Setting{}).Select(`distinct coin`).Rows()
	var coin string
	index := 0
	coins := make([]string, 0)
	for rows.Next() {
		err := rows.Scan(&coin)
		if err != nil {
			return
		}
		res, _ := util.HttpRequest(http.MethodGet, `https://www.gate.io/json_svr/coin/tip/`+coin, ``, nil, 1000)
		fmt.Println(fmt.Sprintf(`%d %s`, index, coin))
		index++
		if strings.Contains(string(res), `explorer.solana.com`) {
			coins = append(coins, coin)
		} else if !strings.Contains(string(res), `Success`) {
			fmt.Println(coin + ` fail` + string(res))
		}
	}
	fmt.Println(coins)
}

func Test_SetLeverage(t *testing.T) {
	market := model.Gate
	model.NewConfig()
	//api.InitMarketInfos(market)
	//symbol := `DUCK_PERP`
	account := model.GetAccounts(0)[market]
	//fmt.Println(api.GetAvailAbleBybit(account.Key, account.Secret, `USDT`))
	//api.SetSymbolLeverage(account, market, symbol)
	coins := []string{`TOSHI`, `RAD`, `RATS`, `PEAQ`, `RAY`, `BONE`, `DOGEGOV`, `GPS`}
	rates := api.GetInterest(account, coins)
	for coin, f := range rates {
		_, b := api.GetBorrowGate(account.Key, account.Secret, coin)
		fmt.Println(fmt.Sprintf("%s %s %f %f", market, coin, f, b))
	}
}

func Test_C(t *testing.T) {
	// Initialize publishers and receivers
	marketPublisher, _ := model.InitMarketPublisher("BTCUSDT")
	orderPublisher, _ := model.InitOrderPublisher("BTCUSDT")
	marketReceiver, _ := model.InitMarketReceiver("BTCUSDT")
	orderReceiver, _ := model.InitOrderReceiver("BTCUSDT")

	// Publish messages
	msg := "{\"id\": \"187d3cb2-942d-484c-8271-4e2141bbadb1\",\"method\": \"time\"}"
	err := marketPublisher.PublishMarket(msg)
	if err != nil {
		return
	}
	oderMsg := "{\"id\":\"1733748523668\",\"method\":\"order.place\",\"params\":{\"apiKey\":\"xxx\",\"newClientOrderId\":\"111\",\"newOrderRespType\":\"RESULT\",\"quantity\":\"1.00\",\"selfTradePreventionMode\":\"EXPIRE_MAKER\",\"side\":\"BUY\",\"signature\":\"xxx\",\"symbol\":\"BTCUSDT\",\"timestamp\":1733748523668,\"type\":\"MARKET\"}}"
	errP := orderPublisher.PublishOrder(oderMsg)
	if errP != nil {
		return
	}
	// Receive messages
	buf := make([]byte, 4096)
	msgSize := marketReceiver.ReceiveMarket(buf)
	if msgSize > 0 {
		fmt.Printf("Received from market: %s\n", string(buf[:msgSize]))
	}

	msgSize = orderReceiver.ReceiveOrder(buf)
	if msgSize > 0 {
		fmt.Printf("Received from order: %s\n", string(buf[:msgSize]))
	}
}

func Test_Funding(t *testing.T) {
	market := model.Gate
	model.NewConfig()
	//model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	//_ = model.AppDB.AutoMigrate(&model.FundingFee{})
	account := model.GetAccounts(0)[market]
	_, fees := api.GetBills(account, time.Now().UnixMilli()-28800000, time.Now().UnixMilli())
	for i, fee := range fees {
		fmt.Println(i)
		fmt.Println(fee)
		model.AppDB.Save(&fee)
	}
}

func Test_TradingStatus(t *testing.T) {
	model.NewConfig()
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	account := model.GetAccounts(0)[model.OKEX]
	fundingRows, _ := model.AppDB.Model(model.FundingFee{}).Select(`ccy,symbol,bal_chg`).
		Where(`ts>=? and market=? and index=?`, util.GetToday().UnixMilli(), account.Market, account.Index).Rows()
	if fundingRows != nil {
		for fundingRows.Next() {
			var coin, symbol string
			balChg := 0.0
			_ = fundingRows.Scan(&coin, &symbol, &balChg)
			price := 0.0
			if coin == `USDT` {
				price = 1.0
			} else {
				_, price = api.GetPriceForce(account.Market, symbol, false)
			}
			fmt.Println(price)
		}
		err := fundingRows.Close()
		if err != nil {
			return
		}
	}
	//api.GetTradingStatusBinancePerp(account)
}
