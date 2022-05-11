package main

import (
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"hello/api"
	"hello/carry"
	"hello/controller"
	"hello/model"
	"hello/util"
)

func main() {
	model.NewConfig()
	order := &model.Order{}
	order = nil
	fmt.Println(order.OrderId)
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
