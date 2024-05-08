package monitor

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"net/http"
	"testing"
)

func Test_MapDel(t *testing.T) {
	data := map[string]interface{}{`1`: 1, `2`: 2, `3`: 3}
	for s := range data {
		if s == `2` {
			delete(data, s)
		}
		data[`4`] = 4
	}
	fmt.Println(data)
}

func Test_initMonitors(t *testing.T) {
	model.NewConfig()
	intervals := []int{300, 900, 3600, 14400, 86400}
	marketInfos := api.GetMarketsBinance(model.GetAccounts(0)[model.BinanceSpot], model.BinanceSpot, model.MarketTypeSpot)
	for symbol := range marketInfos {
		for _, interval := range intervals {
			settingMonitor := model.SettingMonitor{
				MailAddress:     `158553808@QQ.COM`,
				Market:          model.BinanceSpot,
				Symbol:          symbol,
				IntervalSeconds: interval,
				WarnChange:      0.015,
				WarnIncrease:    0.01,
				WarnVolume:      10000,
			}
			model.AppDB.Save(&settingMonitor)
		}
	}
	response, _ := util.HttpRequest(http.MethodPost, `https://user.api.it120.cc/user/apiExtUserCash/list`,
		`page=1&pageSize=50&mobile=19525266383&aggregate=`, map[string]string{`x-token`: `7404f54e-4675-48ee-94bc-113e772c96ed`,
			`Content-Type`: `application/x-www-form-urlencoded`}, 10000)
	fmt.Println(string(response))
}

func doFail() {
	defer func() {
		if recover() != nil {
			fmt.Println(`fail`)
		}
	}()
	a := 0
	a = 5 / a
}
func Test_recovery(t *testing.T) {
	doFail()
	fmt.Println(`recovery`)
}
