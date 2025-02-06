package api

import (
	"fmt"
	"github.com/antihax/optional"
	"github.com/gateio/gateapi-go/v6"
	"hello/model"
	"hello/util"
	"strconv"
	"strings"
)

// getBillsGate 获取Gate交易所的账户资金费用记录 https://www.gate.io/docs/developers/apiv4/zh_CN/#%E6%9F%A5%E8%AF%A2%E5%90%88%E7%BA%A6%E8%B4%A6%E6%88%B7%E5%8F%98%E6%9B%B4%E5%8E%86%E5%8F%B2
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
	client, ctx := getClientGate(account.Key, account.Secret)
	opts := &gateapi.ListFuturesAccountBookOpts{From: optional.NewInt64(begin / 1000), To: optional.NewInt64(end / 1000), Type_: optional.NewString("fund")}
	book, _, err := client.FuturesApi.ListFuturesAccountBook(ctx, settle, opts)
	if err != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`market %s to getbills http error %v`, model.Gate, err))
		return false, nil
	}
	var fundingFees = make([]*model.FundingFee, 0)
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
		}
		fundingFees = append(fundingFees, fundingFee)
	}
	return true, fundingFees
}
