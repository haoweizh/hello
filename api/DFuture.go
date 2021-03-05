package api

import (
	"fmt"
	"github.com/bitly/go-simplejson"
	"hello/model"
	"hello/util"
	"strings"
	"time"
)

var lastDepthPingDFuture = util.GetNowUnixMillion()

var subscribeHandlerDFuture = func(subscribes []interface{}, subType string) error {
	var err error = nil
	for _, subscribe := range subscribes {
		subMsg := fmt.Sprintf(`{"id": "id1", "includeDfutureDay": "1", sub:"%s"}`, subscribe)
		if err = sendToWs(model.DFuture, []byte(subMsg)); err != nil {
			util.SocketInfo("dfuture can not subscribe fill " + err.Error())
		}
	}
	return err
}

func WsDepthServeDFuture(markets *model.Markets, errHandler ErrHandler) (chan struct{}, error) {
	wsHandler := func(event []byte) {
		responseJson, err := util.NewJSON(event)
		if err != nil {
			errHandler(err)
			return
		}
		if responseJson == nil {
			return
		}
		fmt.Println(string(event))
		if util.GetNowUnixMillion()-lastDepthPingDFuture > 15000 {
			lastDepthPingDFuture = util.GetNowUnixMillion()
			ts := time.Now().UnixNano() / int64(time.Second)
			pingMsg := fmt.Sprintf(
				`{"verify":1,"apiTime":%d,"deviceInfo":"326d7f7cfe5c0421cbfb50a1dc7e839f","token":"3f938b62ba748d891c48a6060bf8875b"}`, ts)
			if err := sendToWs(model.DFuture, []byte(pingMsg)); err != nil {
				util.SocketInfo("dfuture server ping client error " + err.Error())
			}
		}
		handleTickerDFuture(markets, responseJson)
	}
	requestUrl := model.AppConfig.WSUrls[model.DFuture]
	subType := model.SubscribeDepth + `,` + model.SubscribeTicker
	return WebSocketServe(model.DFuture, requestUrl, ``, GetWSSubscribes(model.DFuture, subType),
		subscribeHandlerDFuture, wsHandler, errHandler)
}

func handleTickerDFuture(markets *model.Markets, response *simplejson.Json) {
	if response == nil {
		return
	}
	code := response.Get("code").MustInt64()
	if code != 202 || response.GetPath(`data`, `tick`, `close`) == nil {
		util.SocketInfo(`DFuture ws msg not 202`)
		return
	}
	symbol := response.GetPath(`data`, `ch`).MustString()
	chs := strings.Split(symbol, `.`)
	if len(chs) > 2 {
		symbol = chs[2]
	}
	ts := response.GetPath(`data`, `ts`).MustInt()
	price := response.GetPath(`data`, `tick`, `close`).MustFloat64()
	bidAsk := &model.BidAsk{
		Ts:         ts * 1000,
		TsReceived: int(util.GetNowUnixMillion()),
		Bids:       []model.Tick{{Price: 0, Amount: 0}},
		Asks:       []model.Tick{{Price: price, Amount: 0}},
	}
	if markets.SetBidAsk(symbol, model.DFuture, bidAsk) {
		for function, handler := range model.GetFunctions(model.DFuture, symbol) {
			if handler != nil {
				settings := model.GetSetting(function, model.DFuture, symbol)
				for _, setting := range settings {
					//go handler(setting, bidAsk)
					fmt.Println(fmt.Sprintf(`handle %s %s %s `, setting.Market, setting.Symbol, setting.Function))
				}
			}
		}
	}
}
