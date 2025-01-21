package main

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"hello/carry"
	"hello/controller"
	"hello/model"
	"hello/service"
	"hello/util"
	"time"
)

func main() {
	model.NewConfig()
	if model.AppConfig.Mode == "agent" {
		agent()
	} else {
		server()
	}
	go util.LogChanHandler(model.AppConfig.Log, model.AppConfig.Port)
	select {}
}

func agent() {
	_, clientMarketConn, errClient := model.InitConn(model.ClientTopic, model.ChanTypeMarket)
	if errClient != nil {
		util.Log(util.LogLevelError, "ok-m-client"+errClient.Error())
		return
	}
	_, okexMarketConn, errOkex := model.InitConn(model.OkxTopic, model.ChanTypeMarket)
	if errOkex != nil {
		util.Log(util.LogLevelError, "ok-m-okex"+errOkex.Error())
		return
	}
	//_, clientOrderConn, err := model.InitConn(model.ClientTopic, model.ChanTypeOrder)
	//if err != nil {
	//	util.Log(util.LogLevelError, "ok-order-client"+err.Error())
	//	return
	//}
	//_, OkexOrderConn, err := model.InitConn(model.OkxTopic, model.ChanTypeOrder)
	//if err != nil {
	//	util.Log(util.LogLevelError, "ok-order-okex"+err.Error())
	//	return
	//}
	noaMarket := service.NewOkexAgentService(clientMarketConn, okexMarketConn)
	//noaOrder := service.NewOkexAgentService(clientOrderConn, OkexOrderConn)
	go noaMarket.HandleClientPublicMessages()
	time.Sleep(1 * time.Second)
	go noaMarket.HandleMessages()
	//go noaOrder.HandleClientPrivateMessages()
	//go noaOrder.HandleMessages()
}

func server() {
	//go func() {
	//	err := http.ListenAndServe("0.0.0.0:8081", nil)
	//	if err != nil {
	//		return
	//	}
	//}()
	var err error
	model.AppDB, err = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	if err != nil {
		util.Log(util.LogLevelError, err.Error())
		return
	}
	//model.AppRedis = redis.NewClient(&redis.Options{
	//	Addr:     model.AppConfig.RedisAddr,
	//	Password: model.AppConfig.RedisPassword,
	//	DB:       0,
	//})
	go controller.ParameterServe()
	//go model.AppEnvironment.HandleOldWSResp()
	go model.AppEnvironment.HandleWSResp()
	carry.Maintain()
}
