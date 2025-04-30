package controller

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"hello/api"
	"hello/model"
	"hello/util"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func convertChineseToArabic(text string) string {
	numberMap := map[string]string{
		"一": "1",
		"二": "2",
		"三": "3",
		"四": "4",
		"五": "5",
		"六": "6",
		"七": "7",
		"八": "8",
		"九": "9",
		"十": "10",
	}
	result := text
	for chinese, arabic := range numberMap {
		result = strings.ReplaceAll(result, chinese, arabic)
	}
	return result
}

func SetAnnouncement(c *gin.Context) {
	market := c.Query("market")
	announcement := c.Query(`content`)
	util.Log(util.LogLevelLocal, fmt.Sprintf(`get announcement %s %s`, market, announcement))
	announcement = convertChineseToArabic(announcement)
	timeRegex := regexp.MustCompile(`将于([^0-9]*)(\d{4})年(\d+)月(\d)+日(\d{2}):(\d{2})`)
	timeMatch := timeRegex.FindStringSubmatch(announcement)
	var implementTime time.Time
	if len(timeMatch) == 7 {
		year, _ := strconv.Atoi(timeMatch[2])
		month, _ := strconv.Atoi(timeMatch[3])
		day, _ := strconv.Atoi(timeMatch[4])
		hour, _ := strconv.Atoi(timeMatch[5])
		minute, _ := strconv.Atoi(timeMatch[6])
		location, _ := time.LoadLocation("Asia/Shanghai")
		implementTime = time.Date(year, time.Month(month), day, hour, minute, 0, 0, location)
		fmt.Println(implementTime)
	}
	symbols := make([]string, 0)
	hours := make([]int, 0)
	// 解析合约信息
	contractRegex := regexp.MustCompile(`([A-Z]+)USDT(\s*)U本位永续合约(.*?)调整为每(\d+)小时`)
	contractMatches := contractRegex.FindAllStringSubmatch(announcement, -1)
	for _, match := range contractMatches {
		if len(match) == 5 {
			symbols = append(symbols, match[1])
			hour, _ := strconv.Atoi(match[4])
			hours = append(hours, hour)
		}
	}
	contractRegex = regexp.MustCompile(`([A-Z]+)和([A-Z]+)USDT(\s*)U本位永续合约(.*?)调整为每(\d+)小时`)
	contractMatches = contractRegex.FindAllStringSubmatch(announcement, -1)
	for _, match := range contractMatches {
		if len(match) == 6 {
			symbols = append(symbols, match[1])
			hour, _ := strconv.Atoi(match[4])
			hours = append(hours, hour)
		}
	}
	notice := `公告` + market
	for i, symbol := range symbols {
		notice = notice + fmt.Sprintf(`%s:%d`, symbol, hours[i])
	}
	if len(symbols) > 0 {
		api.SendMails(notice, announcement)
	} else {
		api.SendMails(`频率无关公告`, announcement)
		util.Log(util.LogLevelLocal, "can not get symbols frequency update"+announcement)
	}
	c.HTML(http.StatusOK, ``, ``)
}

func GetFrInterval(c *gin.Context) {
	rows, _ := model.AppDB.Model(&model.Setting{}).Select(`market,symbol,chance_limit`).Where(
		`function=? and chance_limit!=0 and chance_limit is not null`, model.FunctionCross).Rows()
	intervalTable := `修改周期表`
	for rows.Next() {
		var market, symbol string
		var chanceLimit int
		err := rows.Scan(&market, &symbol, &chanceLimit)
		if err == nil {
			intervalTable += fmt.Sprintf("\n%s %s %d", market, symbol, chanceLimit)
		}
	}
	util.Log(util.LogLevelLocal, intervalTable)
	c.String(http.StatusOK, intervalTable)
}

func SetFrInterval(c *gin.Context) {
	strCode := c.Query(`code`)
	if !checkCode(strCode) {
		c.String(http.StatusOK, `code error `+strCode)
		return
	}
	strHour := c.Query(`hour`)
	market := strings.ToLower(c.Query(`market`))
	symbol := strings.ToUpper(c.Query(`symbol`))
	if market == `binance` {
		market = `binanceperp`
	}
	if !strings.Contains(symbol, `_`) {
		symbol += model.UniStandardTail[model.MarketTypePerp]
	}
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
