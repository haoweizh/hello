package Grid

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"math/rand"
	"time"
)

// 滑点
const tradeCost = 0.002
const fixAmtU = 100.0

type Data struct {
	priceLow, priceHigh, N float64
	orderBuy, orderSell    *model.Order
	begin                  time.Time
}

func createOrders(setting *model.Setting, data *Data, candle *model.Candle) {
	if (data.orderBuy == nil || data.orderBuy.Status != model.CarryStatusWorking) && setting.Chance < 3 {
		price := data.priceLow + data.N/2
		if setting.Chance > 0 {
			price = math.Min(data.priceLow, setting.PriceX-data.N/2)
		} else if setting.Chance < 0 {
			priceChange := 1.5 * data.N
			if setting.Seconds == 14400 {
				priceChange = 2 * data.N
				if !model.CommonTurtleSymbols[setting.Symbol] {
					priceChange = 2.5 * data.N
				}
			}
			price = math.Max(setting.PriceX/3+data.priceHigh*2/3-priceChange, data.priceLow)
		}
		amount := fixAmtU / price
		if setting.Chance < 0 {
			amount = setting.GridAmount
		}
		data.orderBuy = &model.Order{Amount: amount,
			Price:            price,
			UnfilledQuantity: amount,
			Function:         setting.Function,
			Market:           setting.Market,
			Fee:              data.N,
			OrderSide:        model.OrderSideBuy,
			OrderType:        model.OrderTypeLimit,
			RefreshType:      model.FunctionSimulation,
			Status:           model.CarryStatusWorking,
			Symbol:           setting.Symbol,
			OrderTime:        candle.Begin}
	}
	if (data.orderSell == nil || data.orderSell.Status != model.CarryStatusWorking) && setting.Chance > -3 {
		price := data.priceHigh - data.N/2
		if setting.Chance > 0 {
			priceChange := 1.5 * data.N
			if setting.Seconds == 14400 {
				priceChange = 2 * data.N
				if !model.CommonTurtleSymbols[setting.Symbol] {
					priceChange = 2.5 * data.N
				}
			}
			price = math.Min(setting.PriceX/3+data.priceLow*2/3+priceChange, data.priceHigh)
		} else if setting.Chance < 0 {
			price = math.Max(data.priceHigh, setting.PriceX+data.N/2)
		}
		amount := fixAmtU / price
		if setting.Chance > 0 {
			amount = setting.GridAmount
		}
		data.orderSell = &model.Order{Amount: amount,
			Price:            price,
			UnfilledQuantity: amount,
			Function:         setting.Function,
			Market:           setting.Market,
			Fee:              data.N,
			OrderSide:        model.OrderSideSell,
			OrderType:        model.OrderTypeLimit,
			RefreshType:      model.FunctionSimulation,
			Status:           model.CarryStatusWorking,
			Symbol:           setting.Symbol,
			OrderTime:        candle.Begin}
	}
	return
}

var ProcessGrid = func(start, end time.Time, setting *model.Setting) {
	util.StoreSyncMap(&model.CarryInfo, nil, `gridInfo`)
	account := model.AppConfig.GetAccounts(setting.Market)[0]
	candles := api.GetMultiCandle(account, setting.Market, 60, start, end,
		map[string]*model.Setting{setting.Symbol: setting}, false)
	util.StoreSyncMap(&model.CarryInfo, fmt.Sprintf(`get market candle %s %s from %s len %d`,
		setting.Market, setting.Symbol, start.String(), len(candles)), `gridInfo`)
	gridCandles := api.CombineCandles(account, setting.Market, setting.Symbol, int(setting.Seconds), start.Add(time.Hour*-1200), end)
	util.StoreSyncMap(&model.CarryInfo, fmt.Sprintf(`get grid candle %s %s from %s len %d`,
		setting.Market, setting.Symbol, start.Add(time.Hour*-1200).String(), len(gridCandles)), `gridInfo`)
	if gridCandles != nil && gridCandles.Len() > 0 && gridCandles[0] != nil && gridCandles[len(gridCandles)-1] != nil {
		util.Info(fmt.Sprintf(`get sorted gridCandles from %s %s to %s %s`,
			gridCandles[0].Begin.String(), gridCandles[0].Symbol,
			gridCandles[gridCandles.Len()-1].Begin.String(), gridCandles[gridCandles.Len()-1].Symbol))
	}
	beginPrice := 0.0
	calcLenN := 10
	if setting.Seconds == 14400 {
		calcLenN = 15
	}
	startPoint := int(math.Max(float64(calcLenN), float64(setting.Far)))
	for i := 0; i < startPoint; i++ {
		beginPrice += gridCandles[i].PriceHigh - gridCandles[i].PriceLow
	}
	gridMap := make(map[int64]*Data)
	gridCandles[calcLenN-1].N = beginPrice / float64(calcLenN)
	for i := startPoint; i < len(gridCandles); i++ {
		data := &Data{begin: gridCandles[i].Begin,
			N: (gridCandles[i-1].N*(float64(calcLenN)-1) + gridCandles[i].PriceHigh - gridCandles[i].PriceLow) / float64(calcLenN)}
		for j := i - int(setting.Far); j < i; j++ {
			if data.priceLow == 0 || data.priceLow > gridCandles[j].PriceLow {
				data.priceLow = gridCandles[j].PriceLow
			}
			if data.priceHigh < gridCandles[j].PriceHigh {
				data.priceHigh = gridCandles[j].PriceHigh
			}
		}
		beginUnix := candles[i].Begin.Unix() - candles[i].Begin.Unix()%setting.Seconds
		gridMap[beginUnix] = data
	}
	for i := 0; i < len(candles); i++ {
		beginUnix := candles[i].Begin.Unix() - candles[i].Begin.Unix()%setting.Seconds
		if gridMap[beginUnix] == nil {
			util.Notice(fmt.Sprintf(`fail to get combine candle %s %s %d`, candles[i].Market, candles[i].Symbol, beginUnix))
			continue
		}
		createOrders(setting, gridMap[beginUnix], candles[i])
		handleGrid(setting, gridMap[beginUnix], candles[i])
		//handleOld(setting, currentGrid.oldOrders, candles[i], currentGrid.candle.N)
		util.StoreSyncMap(&model.CarryInfo, fmt.Sprintf(`deal grid candle %s %s for grid %s`,
			setting.Market, setting.Symbol, candles[i].Begin.String()), `gridInfo`)
	}
}

func dealGridSuccess(setting *model.Setting, order *model.Order, candle *model.Candle) {
	order.Status = model.CarryStatusSuccess
	order.DealAmount = order.Amount
	order.UnfilledQuantity = 0
	order.DealPrice = order.Price
	order.LineBuy = setting.OpenShortMargin
	order.LineSell = setting.CloseShortMargin
	order.OrderUpdateTime = candle.Begin
	order.OrderId = fmt.Sprintf(`%d_%d`, candle.Begin.Unix(), rand.Int())
	if order.OrderType != model.OrderTypeLimit {
		if order.OrderSide == model.OrderSideSell {
			order.DealPrice = order.Price * (1 - tradeCost)
		} else if order.OrderSide == model.OrderSideBuy {
			order.DealPrice = order.Price * (1 + tradeCost)
		}
	}
	model.AppDB.Save(order)
}

func handleGrid(setting *model.Setting, data *Data, candle *model.Candle) {
	if data.orderBuy != nil && data.orderBuy.Status == model.CarryStatusWorking && candle.PriceLow < data.orderBuy.Price {
		dealGridSuccess(setting, data.orderBuy, candle)
		setting.PriceX = data.orderBuy.Price
		if setting.Chance >= 0 {
			setting.Chance += 1
			setting.GridAmount += data.orderBuy.Amount
		} else {
			setting.Chance = 0
			setting.GridAmount = 0
		}
		data.orderBuy = nil
	}
	if data.orderSell != nil && data.orderSell.Status == model.CarryStatusWorking && candle.PriceHigh > data.orderSell.Price {
		dealGridSuccess(setting, data.orderSell, candle)
		setting.PriceX = data.orderSell.Price
		if setting.Chance <= 0 {
			setting.Chance -= 1
			setting.GridAmount += data.orderSell.Amount
		} else {
			setting.Chance = 0
			setting.GridAmount = 0
		}
		data.orderSell = nil
	}
}
