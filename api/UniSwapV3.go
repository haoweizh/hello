package api

import (
	"fmt"
	"hello/cfmms/pool"
	"hello/entity"
	"hello/model"
	"hello/util"
	"time"
)

// The trading depth
var depthUSDT = []float64{100, 200, 400, 600, 900, 1300, 2000}

// return token's price in USDT, get it from CEX ticks, iterate through all CEX symbols
func getPriceInU(pool *pool.UniswapV3Pool) (err error, priceA, priceB float64) {
	if pool == nil {
		return fmt.Errorf("nil pool"), 0, 0
	}

	// always get accounts from account 0, as we only need price it does not matter which account is used.
	accounts := model.GetAccounts(0)

	for _, account := range accounts {
		if account == nil {
			continue
		}

		if ok, price := GetPriceForce(account.Key, account.Secret, pool.TokenAName+"_USDT", account.Market); ok {
			priceA = price
		}

		if ok, price := GetPriceForce(account.Key, account.Secret, pool.TokenBName+"_USDT", account.Market); ok {
			priceB = price
		}
	}

	if priceA == 0 && priceB == 0 {
		return fmt.Errorf("could not get price for pool %s", pool.GetKey()), 0, 0
	}

	price := pool.CalculatePrice(pool.TokenA)
	if priceA == 0 {
		priceA = priceB / price
	}

	if priceB == 0 {
		priceB = priceA * price
	}

	return
}

func ParsePools(msg interface{}) (pools []*pool.UniswapV3Pool) {
	panic("implement me")
}

// TickServeUniSwapV3 todo: read from MemPoolPending, invoke calcPrice, create tick to feed handler
func TickServeUniSwapV3() {
	for {
		msg := <-entity.MemPoolPending
		fmt.Print(msg)

		pools := ParsePools(msg)

		for _, pool := range pools {
			// price := pool.CalculatePrice(pool.Address)
			err, priceA, _ := getPriceInU(pool)
			symbol := pool.GetSymbol()
			if err != nil {
				util.Notice("failed to get price in U for %s", symbol)
				continue
			}

			now := int(time.Now().UnixMilli())
			bidAsk := &model.BidAsk{Ts: now, TsReceived: now}
			for _, usdtAmount := range depthUSDT {
				tokenAAmount := usdtAmount / priceA
				priceBuy := pool.PredictPrice(tokenAAmount)
				priceSell := pool.PredictPrice(-1 * tokenAAmount)
				bidAsk.Asks = append(bidAsk.Asks, model.Tick{Market: model.UniSwapV3, Symbol: symbol, Price: priceBuy, Amount: tokenAAmount})
				bidAsk.Bids = append(bidAsk.Bids, model.Tick{Market: model.UniSwapV3, Symbol: symbol, Price: priceSell, Amount: tokenAAmount})
			}

			if model.AppMarkets.SetBidAsk(symbol, model.UniSwapV3, bidAsk) {
				funcHandlers := GetFunctions(model.UniSwapV3, symbol)
				if funcHandlers != nil {
					funcHandlers.Range(func(function, value interface{}) bool {
						setting := GetSetting(function.(string), model.UniSwapV3, symbol)
						if setting != nil && value != nil {
							go value.(model.CarryHandler)(setting, bidAsk)
						}
						return true
					})
				}
			}
		}
	}
}
