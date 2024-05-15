package main

import (
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"hello/api"
	"hello/model"
	"hello/util"
	"net/http"
	"testing"
	"time"
)

func Test_MapAdd(t *testing.T) {
	data := map[string]interface{}{`1`: 1, `2`: 2, `3`: 3}
	for s, i := range data {
		key := time.Now().String()
		data[key] = i.(int) * 2
		fmt.Println(s, i)
	}
}

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
		if r := recover(); r != nil {
			fmt.Println(`fail`)
		}
		fmt.Println(`finish`)
	}()
	a := 0
	a = 5 / a
}

func Test_recovery(t *testing.T) {
	doFail()
	fmt.Println(`recovery back`)
}

func Test_fullMonitors(t *testing.T) {
	model.NewConfig()
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	_ = model.AppDB.AutoMigrate(&model.SettingMonitor{})
	//account := model.AppConfig.GetAccounts(model.BinanceSpot)[0]
	api.InitMarketInfos(model.BinanceSpot)
	addresses := []string{`haoweizh@qq.com`, `57059329@qq.com`, `158553808@qq.com`, `148392942@qq.com`, `759775226@qq.com`}
	model.MarketInfos.Range(func(key, value any) bool {
		for _, address := range addresses {
			monitor := &model.SettingMonitor{
				MailAddress: address, Market: model.BinanceSpot,
				Symbol:          value.(*model.MarketInfo).Name,
				IntervalSeconds: 300,
				WarnChange:      0.02,
				WarnIncrease:    0.01,
				WarnVolume:      200000,
				Volume24:        10000}
			model.AppDB.Save(monitor)
			monitor = &model.SettingMonitor{
				MailAddress: address, Market: model.BinanceSpot,
				Symbol:          value.(*model.MarketInfo).Name,
				IntervalSeconds: 3600,
				WarnChange:      0.05,
				WarnIncrease:    0.03,
				WarnVolume:      2000000,
				Volume24:        10000}
		}
		return true
	})
}
