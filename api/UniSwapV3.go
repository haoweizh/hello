package api

import (
	"fmt"
	"hello/entity"
)

// TickServeUniSwapV3 todo: read from ChanBlock, invoke calcPrice, create tick to feed handler
func TickServeUniSwapV3() {
	for {
		msg := <-entity.ChanBlock
		fmt.Print(msg)
		//if model.AppMarkets.SetBidAsk(standardSymbol, model.UniSwapV3, bidAsk) {
		//	funcHandlers := GetFunctions(model.UniSwapV3, standardSymbol)
		//	if funcHandlers != nil {
		//		funcHandlers.Range(func(function, value interface{}) bool {
		//			setting := GetSetting(function.(string), model.Ftx, standardSymbol)
		//			if setting != nil && value != nil {
		//				go value.(model.CarryHandler)(setting, bidAsk)
		//			}
		//			return true
		//		})
		//	}
		//}
	}
}
