package Grid

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"math/rand"
	"strings"
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
	if (data.orderBuy == nil || data.orderBuy.Status != model.CarryStatusWorking) && setting.Chance < 3 &&
		(strings.Contains(setting.Function, `both`) || strings.Contains(setting.Function, `buy`) || setting.Chance < 0) {
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
		util.Notice(fmt.Sprintf(`create order %s %s %s amt %f at %f %s`,
			data.orderBuy.Market, data.orderBuy.Symbol, data.orderBuy.OrderSide, data.orderBuy.Amount, data.orderBuy.Price, data.orderBuy.OrderTime.String()))
	}
	if (data.orderSell == nil || data.orderSell.Status != model.CarryStatusWorking) && setting.Chance > -3 &&
		(strings.Contains(setting.Function, `both`) || strings.Contains(setting.Function, `sell`) || setting.Chance > 0) {
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
		util.Notice(fmt.Sprintf(`create order %s %s %s amt %f at %f %s`,
			data.orderSell.Market, data.orderSell.Symbol, data.orderSell.OrderSide, data.orderSell.Amount, data.orderSell.Price, data.orderSell.OrderTime.String()))
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
	gridMap := make(map[string]*Data)
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
		sign := fmt.Sprintf(`%s%s%d`, setting.Market, setting.Symbol, gridCandles[i].Begin.Unix()-gridCandles[i].Begin.Unix()%setting.Seconds)
		gridMap[sign] = data
	}
	msgMiss := ``
	msgHandle := ``
	for i := 0; i < len(candles); i++ {
		if candles[i] == nil {
			util.Notice(fmt.Sprintf(`fail to get market candle %s %s %d`, setting.Market, setting.Symbol, i))
			continue
		}
		sign := fmt.Sprintf(`%s%s%d`, setting.Market, setting.Symbol, candles[i].Begin.Unix()-candles[i].Begin.Unix()%setting.Seconds)
		if gridMap[sign] == nil {
			if msgMiss != fmt.Sprintf(`fail to get combine candle %s`, sign) {
				msgMiss = fmt.Sprintf(`fail to get combine candle %s`, sign)
				util.Notice(msgMiss)
			}
			continue
		}
		if msgHandle != fmt.Sprintf(`deal grid candle %s`, sign) {
			msgHandle = fmt.Sprintf(`deal grid candle %s`, sign)
			util.Notice(msgHandle)
		}
		createOrders(setting, gridMap[sign], candles[i])
		handleGrid(setting, gridMap[sign], candles[i])
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
	order.OrderUpdateTime = candle.Begin
	order.OrderId = fmt.Sprintf(`%d_%d`, candle.Begin.Unix(), rand.Int())
	setting.PriceX = order.Price
	if order.OrderType != model.OrderTypeLimit {
		if order.OrderSide == model.OrderSideSell {
			order.DealPrice = order.Price * (1 - tradeCost)
		} else if order.OrderSide == model.OrderSideBuy {
			order.DealPrice = order.Price * (1 + tradeCost)
		}
	}
	util.Notice(fmt.Sprintf(`success deal %s %s %s amt %f at %f %s candle %f - %f %s`,
		order.Market, order.Symbol, order.OrderSide, order.Amount, order.Price, order.OrderTime.String(),
		candle.PriceLow, candle.PriceHigh, candle.Begin.String()))
	model.AppDB.Save(order)
}

func handleGrid(setting *model.Setting, data *Data, candle *model.Candle) {
	if data.orderBuy != nil && data.orderBuy.Status == model.CarryStatusWorking && candle.PriceLow < data.orderBuy.Price {
		if setting.Chance >= 0 {
			setting.Chance += 1
			setting.GridAmount += data.orderBuy.Amount
		} else {
			setting.Chance = 0
			setting.GridAmount = 0
		}
		data.orderBuy.GridPos = setting.Chance
		dealGridSuccess(setting, data.orderBuy, candle)
		data.orderBuy = nil
		util.Notice(fmt.Sprintf(`set setting %s %s chance %d amt %f time %s`,
			setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, candle.Begin.String()))
	}
	if data.orderSell != nil && data.orderSell.Status == model.CarryStatusWorking && candle.PriceHigh > data.orderSell.Price {
		if setting.Chance <= 0 {
			setting.Chance -= 1
			setting.GridAmount += data.orderSell.Amount
		} else {
			setting.Chance = 0
			setting.GridAmount = 0
		}
		data.orderSell.GridPos = setting.Chance
		dealGridSuccess(setting, data.orderSell, candle)
		data.orderSell = nil
		util.Notice(fmt.Sprintf(`set setting %s %s chance %d amt %f time %s`,
			setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, candle.Begin.String()))
	}
}
