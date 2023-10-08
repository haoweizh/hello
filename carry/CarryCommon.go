package carry

import (
	"hello/api"
	"hello/model"
	"strings"
)

func GetHolding(account *model.Account, market, symbol string) (success bool, holding float64) {
	_, marketType, coin, _ := model.GetFromStandard(market, symbol)
	if marketType == model.MarketTypePerp {
		_, positions, _, _ := api.GetPositions(account.Key, account.Secret, market)
		for _, position := range positions {
			if strings.EqualFold(symbol, position.Currency) {
				return true, position.Holding
			}
		}
	} else if marketType == model.MarketTypeSpot {
		_, balances, _, _ := api.GetBalances(account.Key, account.Secret, market)
		for _, balance := range balances {
			if strings.EqualFold(balance.Coin, coin) {
				return true, balance.Amount
			}
		}
	}
	return false, 0
}
