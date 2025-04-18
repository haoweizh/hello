package controller

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"hello/model"
	"hello/util"
	"net/http"
	"strconv"
)

func SetFrInterval(c *gin.Context) {
	strCode := c.Query(`code`)
	if !checkCode(strCode) {
		c.String(http.StatusOK, `code error `+strCode)
		return
	}
	strHour := c.Query(`hour`)
	market := c.Query(`market`)
	symbol := c.Query(`symbol`)
	marketInfo := model.GetMarketInfo(market, symbol)
	if marketInfo == nil {
		c.String(http.StatusOK, fmt.Sprintf(`no market info %s %s`, market, symbol))
		return
	}
	hour, err := strconv.ParseInt(strHour, 10, 64)
	if err != nil {
		c.String(http.StatusOK, fmt.Sprintf(`interval error %s %s %s %s`, market, symbol, strHour, err.Error()))
		return
	}
	if hour >= 0 {
		model.AppDB.Model(&model.Setting{}).Where(`function=? and market=? and symbol=?`,
			model.FunctionCross, market, symbol).Updates(map[string]interface{}{`chance_limit`: hour})
		if hour > 0 {
			marketInfo.FundingRateInterval = int(hour * 3600000)
			msg := fmt.Sprintf(`set fr interval %s %s %s %d`, market, symbol, strHour, marketInfo.FundingRateInterval)
			util.Log(util.LogLevelLocal, msg)
			c.String(http.StatusOK, msg)
		} else {
			c.String(http.StatusOK, fmt.Sprintf(`clear manual %s %s %s %d`, market, symbol, strHour, marketInfo.FundingRateInterval))
		}
	} else {
		c.String(http.StatusOK, fmt.Sprintf(`wrong hour no set %s %s %s`, market, symbol, strHour))
	}
}
