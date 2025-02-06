package api

import (
	"fmt"
	"github.com/antihax/optional"
	"github.com/gateio/gateapi-go/v6"
	"hello/model"
	"hello/util"
)

func getBillsGate(account *model.Account, begin, end int64) {
	client, ctx := getClientGate(account.Key, account.Secret)
	opts := &gateapi.ListFuturesAccountBookOpts{From: optional.NewInt64(begin), To: optional.NewInt64(end), Type_: optional.NewString("fund")}
	book, h, err := client.FuturesApi.ListFuturesAccountBook(ctx, `usdt`, opts)
	if err != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to get gate wallet history %s %#v`, err.Error(), h))
		return
	}
	for _, accountBook := range book {
		fmt.Println(fmt.Sprintf("%#v", accountBook))
	}
}
