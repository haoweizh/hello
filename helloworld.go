package main

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"hello/carry"
	"hello/controller"
	"hello/model"
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
		model.Log(``, model.LogLevelError, ``, model.SystemOther, err.Error())
		return
	}
	//model.AppRedis = redis.NewClient(&redis.Options{
	//	Addr:     model.AppConfig.RedisAddr,
	//	Password: model.AppConfig.RedisPassword,
	//	DB:       0,
	//})
	go controller.ParameterServe()
	go model.AppEnvironment.HandleWSResp()
	carry.Maintain()
	select {}
}
