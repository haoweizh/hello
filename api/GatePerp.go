package api

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/antihax/optional"
	"github.com/gateio/gateapi-go/v6"
	"hello/model"
	"hello/util"
	"math"
	"strconv"
	"strings"
	"time"
)

// getBillsGate 获取Gate交易所的账户资金费用记录 https://www.gate.io/docs/developers/apiv4/en/#query-futures-account
// 参数:
//
//	account: 包含账户信息的指针，包括API密钥和密钥
//	begin: unix second 开始时间戳，用于筛选记录
//	end: unix second 结束时间戳，用于筛选记录
//
// 返回值:
//
//	bool: 请求是否成功
//	[]*model.FundingFee: 资金费用记录的切片
func getBillsGate(account *model.Account, begin, end int64) (bool, []*model.FundingFee) {
	settle := `usdt`
	limit := 100
	offset := 0
	client, ctx := getClientGate(account.Key, account.Secret)
	opts := &gateapi.ListFuturesAccountBookOpts{From: optional.NewInt64(begin / 1000), To: optional.NewInt64(end / 1000),
		Type_: optional.NewString("fund"), Limit: optional.NewInt32(int32(limit)), Offset: optional.NewInt32(int32(offset))}
	var fundingFees = make([]*model.FundingFee, 0)
	book, _, err := client.FuturesApi.ListFuturesAccountBook(ctx, settle, opts)
	for {
		if err != nil {
			util.Log(util.LogLevelError, fmt.Sprintf(`market %s to getbills http error %v`, model.Gate, err))
			break
		}
		for _, data := range book {
			ts := int64(data.Time)
			balChg, _ := strconv.ParseFloat(data.Change, 64)
			success, _, symbol := model.GetFromDialect(model.Gate, model.MarketTypePerp, data.Contract)
			if !success {
				util.Log(util.LogLevelError, fmt.Sprintf(`market %s to getbills instId %s can not get standardSymbol`, model.Gate, data.Contract))
				continue
			}
			fundingFee := &model.FundingFee{
				Market: model.Gate,
				Ccy:    strings.ToUpper(settle),
				Ts:     ts * 1000,
				BalChg: balChg,
				Symbol: symbol,
				Index:  account.Index,
			}
			fundingFees = append(fundingFees, fundingFee)
		}
		if len(book) < limit {
			break
		}
		offset += len(book)
		opts.Offset = optional.NewInt32(int32(offset))
		opts.Limit = optional.NewInt32(int32(limit))
		book, _, err = client.FuturesApi.ListFuturesAccountBook(ctx, settle, opts)
		time.Sleep(time.Second)
	}
	return true, fundingFees
}

var wsPriHandlerGatePerp = func(market, key string, msg []byte) {
	responseJson, err := util.NewJSON(msg)
	if err != nil || responseJson == nil {
		return
	}
	channel := responseJson.Get(`channel`).MustString()
	ts := responseJson.Get(`time_ms`).MustInt64()
	result := responseJson.GetPath(`header`, `status`).MustString()
	connKey := getPrivateConnKey(model.Gate, key, model.MarketTypePerp)
	valueFuture, _ := model.AppEnvironment.ConnOrder.Load(connKey)
	if channel == `futures.ping` {
		if valueFuture == nil {
			return
		}
		err := valueFuture.(*model.WSConn).WriteMsg([]byte(fmt.Sprintf(
			`{"time" : %d, "channel" : "futures.pong"}`, time.Now().Unix())))
		if err != nil {
			model.AppEnvironment.ConnOrder.Delete(connKey)
			return
		}
		util.Log(util.LogLevelError, fmt.Sprintf(`gate futures order pong from %s`, string(msg)))
	} else if channel == `futures.orders` {
		data := responseJson.Get(`result`).MustArray()
		for _, datum := range data {
			value := datum.(map[string]interface{})
			size, _ := value[`size`].(json.Number).Float64()
			left, _ := value[`left`].(json.Number).Float64()
			dialectSymbol := value[`contract`].(string)
			// 此处不同于gate标准的合约格式以_USDT结尾，而是以_USD结尾
			coin := strings.Split(dialectSymbol, "_")[0]
			symbol := coin + model.UniStandardTail[model.MarketTypePerp]
			_, size = model.ParseRealAmount(model.Gate, symbol, size)
			orderSide := model.OrderSideBuy
			if size < 0 {
				orderSide = model.OrderSideSell
			}
			_, left = model.ParseRealAmount(model.Gate, symbol, left)
			dealAmount := math.Abs(size) - math.Abs(left)
			status := model.CarryStatusWorking
			if value[`status`] == `finished` {
				status = model.CarryStatusSuccess
			}
			orderId := value["id"].(json.Number).String()
			// //判定为自动减仓，停止开单 value[`tif`].(string) == `ioc`
			if value[`text`].(string) == `auto_deleveraging` || strings.Contains(strings.ToLower(value[`text`].(string)), `auto`) {
				util.Log(util.LogLevelInfo, fmt.Sprintf(`auto_deleveraging %v %s %s %s %s %s %s`,
					value[`text`], coin, market, symbol, key, orderSide, string(msg)))
				if orderSide == model.OrderSideSell {
					util.StoreSyncMap(&model.AppEnvironment.PauseTrade, true, coin, market, symbol, key, model.OrderSideBuy)
				} else {
					util.StoreSyncMap(&model.AppEnvironment.PauseTrade, true, coin, market, symbol, key, model.OrderSideSell)
				}
			}
			UpdateOrderDeal(market, orderId, ``, status, string(msg), dealAmount)
		}
	} else if channel == `futures.positions` {
		//https: //www.gate.io/docs/developers/futures/ws/zh_CN/#%E4%BB%93%E4%BD%8D%E8%AE%A2%E9%98%85
		util.LogLess(util.LogLevelInfo, "risk check ws update positions gate "+string(msg))
	} else {
		channel = responseJson.GetPath(`header`, `channel`).MustString()
		if channel == `futures.order_place` {
			if responseJson.GetPath(`header`, `status`).MustString() == `400` { //AUTHENTICATION_FAILED Not login
				requestId := responseJson.Get(`request_id`).MustString()
				wsResp := model.WSResp{RequestId: requestId, Success: false, Msg: responseJson.GetPath(`data`, `errs`, `message`).MustString()}
				model.AppEnvironment.WSRespChan <- wsResp
				if responseJson.GetPath(`data`, `errs`, `label`).MustString() == `AUTHENTICATION_FAILED` {
					account := model.AppConfig.GetAccountFromKeyIndex(model.Gate, key, -1)
					if valueFuture == nil {
						return
					}
					wsLoginGateOrder(account, valueFuture.(*model.WSConn), model.MarketTypePerp)
				}
			} else if !responseJson.Get(`ack`).MustBool() {
				requestId := responseJson.Get(`request_id`).MustString()
				idJson := responseJson.GetPath(`data`, `result`, `id`).MustInt()
				wsResp := model.WSResp{RequestId: requestId, OrderId: strconv.Itoa(idJson)}
				if result == `200` {
					wsResp.Success = true
				} else {
					wsResp.Success = false
					wsResp.Msg = responseJson.GetPath(`data`, `errs`, `message`).MustString()
				}
				model.AppEnvironment.WSRespChan <- wsResp
			}
		} else if channel == `futures.login` {
			if result == `200` {
				//connKey := getPrivateConnKey(model.Gate, key, model.MarketTypePerp)
				//未用到ts
				_ = ts
				subscribePrivateGatePerp(valueFuture.(*model.WSConn), connKey, key)
			}
		}
	}
}

func subscribePrivateGatePerp(conn *model.WSConn, connKey, key string) {
	gateAccounts := model.AppConfig.GetAccounts(model.Gate)
	secret := ``
	for _, account := range gateAccounts {
		if account.Key == key {
			secret = account.Secret
		}
	}
	if conn == nil {
		return
	}
	err := conn.WriteMsg([]byte(getSignMsgSend(secret, `futures.orders`, key)))
	if err != nil {
		model.AppEnvironment.ConnOrder.Delete(connKey)
		return
	}
	err = conn.WriteMsg([]byte(getSignMsgSend(secret, `futures.positions`, key)))
	if err != nil {
		model.AppEnvironment.ConnOrder.Delete(connKey)
		return
	}
}

func getSignMsgSend(secret, channel, key string) string {
	ts := time.Now().Unix()
	hashFuture := hmac.New(sha512.New, []byte(secret))
	hashFuture.Write([]byte(fmt.Sprintf("channel=%s&event=subscribe&time=%d", channel, ts)))
	sign := hex.EncodeToString(hashFuture.Sum(nil))
	return fmt.Sprintf(`{"time":%d,"channel":"%s","event":"subscribe","payload":["!all"],
					"auth":{"method":"api_key","KEY":"%s","SIGN":"%s"}}`, ts, channel, key, sign)
}
