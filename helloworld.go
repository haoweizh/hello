package main

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"hello/api"
	"hello/carry"
	"hello/controller"
	"hello/model"
	"hello/util"
	"log"
	"net/http"
	_ "net/http/pprof"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:8081", nil))
	}()
	model.NewConfig()
	var err error
	model.AppDB, err = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	if err != nil {
		util.Notice(err.Error())
		return
	}
	go controller.ParameterServe()
	go api.AppWSManager.Start()
	carry.Maintain()
	select {}
}
