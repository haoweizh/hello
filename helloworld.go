package main

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"hello/carry"
	"hello/controller"
	"hello/model"
	"hello/util"
	//_ "net/http/pprof"
)

func main() {
	//go func() {
	//	err := http.ListenAndServe("0.0.0.0:8081", nil)
	//	if err != nil {
	//		return
	//	}
	//}()
	model.NewConfig()
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
	go util.LogChanHandler(model.AppConfig.Log, model.AppConfig.Port)
	carry.Maintain()
	select {}
}
