package controller

import (
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"hello/model"
	"hello/util"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

var codeGenTime int64
var codes = sync.Map{}     // string - *time.Time
var userNames = sync.Map{} // mailAddress - code

func checkCode(code string) (valid bool) {
	value, ok := codes.Load(code)
	if !ok || value == nil {
		return false
	}
	valid = value.(*time.Time).Add(time.Minute * 5).After(time.Now())
	if !valid {
		codes.Delete(code)
	}
	return valid
}

func login(c *gin.Context) {
	session := sessions.Default(c)
	value := c.Query(`code`)
	userName := c.Query(`user`)
	code, ok := userNames.Load(userName)
	if ok && code != nil {
		if checkCode(value) && code.(string) == value {
			session.Set(`user`, userName)
			err := session.Save()
			if err == nil {
				userNames.Delete(userName)
				codes.Delete(value)
				c.String(http.StatusOK, `登录成功`)
				return
			}
		} else {
			c.String(http.StatusOK, `验证码错误或过期`)
		}
	} else {
		c.String(http.StatusForbidden, `用户不存在`)
	}
	c.String(http.StatusForbidden, `登陆失败`)
}

func GetCode(c *gin.Context) {
	userName := c.Query(`user`)
	waitTime := (util.GetNowUnixMillion() - codeGenTime) / 1000
	if waitTime < 30 {
		waitTime = 30 - waitTime
		c.String(http.StatusOK, fmt.Sprintf(`还要等待 %d 秒才能再次发送`, waitTime))
	} else {
		codeGenTime = util.GetNowUnixMillion()
		rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
		rnd = rand.New(rand.NewSource(rnd.Int63()))
		code := fmt.Sprintf("%06v", rnd.Int31n(1000000))
		codeTime := time.Now()
		codes.Store(code, &codeTime)
		userNames.Store(userName, code)
		util.Notice(fmt.Sprintf(`code is %s`, code))
		err := util.SendMail(model.AppConfig.FromMail, model.AppConfig.FromMailAuth, userName, `验证码`, `验证码是 `+code)
		if err != nil {
			msg := fmt.Sprintf(`fail to send mail to %s err %s`, userName, err.Error())
			util.Notice(msg)
			c.String(http.StatusBadRequest, msg)
		} else {
			c.String(http.StatusOK, `调用成功，请查收邮箱，如果没有，检查日志`)
		}
	}
}
