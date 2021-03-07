package main

import (
	"hello/api"
	"hello/carry"
	"hello/controller"
	"hello/model"
)

func main() {
	model.NewConfig()
	go controller.ParameterServe()
	go api.AppWSManager.Start()
	carry.Maintain()
	select {}
}
