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
	value, getCode := c.GetPostForm(`code`)
	userName, getUser := c.GetPostForm(`user`)
	if !getCode || !getUser {
		c.JSON(http.StatusBadRequest, map[string]interface{}{
			`status`: `fail`, `msg`: `code and user is required`, `data`: map[string]interface{}{}})
		return
	}
	code, ok := userNames.Load(userName)
	if ok && code != nil {
		if checkCode(value) && code.(string) == value {
			session.Set(`user`, userName)
			err := session.Save()
			if err == nil {
				userNames.Delete(userName)
				codes.Delete(value)
				c.JSON(http.StatusOK, map[string]interface{}{`status`: `ok`, `msg`: `success`, `data`: map[string]interface{}{}})
				return
			}
		} else {
			c.JSON(http.StatusOK, map[string]interface{}{`status`: `fail`, `msg`: `验证码错误或过期`, `data`: map[string]interface{}{}})
			return
		}
	} else {
		c.JSON(http.StatusOK, map[string]interface{}{`status`: `fail`, `msg`: `用户不存在`, `data`: map[string]interface{}{}})
		return
	}
	c.JSON(http.StatusOK, map[string]interface{}{`status`: `fail`, `msg`: `登陆失败`, `data`: map[string]interface{}{}})
}

func GetCode(c *gin.Context) {
	userName, get := c.GetPostForm(`user`)
	if !get {
		c.JSON(http.StatusBadRequest, map[string]interface{}{`status`: `fail`, `msg`: `user is required`, `data`: map[string]interface{}{}})
		return
	}
	waitTime := (util.GetNowUnixMillion() - codeGenTime) / 1000
	if waitTime < 30 {
		waitTime = 30 - waitTime
		c.JSON(http.StatusOK, map[string]interface{}{`status`: `fail`, `msg`: fmt.Sprintf(`还要等待 %d 秒才能再次发送`, waitTime), `data`: map[string]interface{}{}})
		return
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
			c.JSON(http.StatusBadRequest, map[string]interface{}{`status`: `fail`, `msg`: msg, `data`: map[string]interface{}{}})
			return
		} else {
			c.JSON(http.StatusOK, map[string]interface{}{`status`: `ok`, `msg`: `success`, `data`: map[string]interface{}{}})
			return
		}
	}
}
