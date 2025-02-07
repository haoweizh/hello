package api

import (
	"fmt"
	"github.com/antihax/optional"
	"github.com/gateio/gateapi-go/v6"
	"hello/model"
	"hello/util"
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
