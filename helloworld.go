package main

import (
	"hello/carry"
	"hello/controller"
	"hello/model"
)

func main() {
	model.NewConfig()
	go controller.ParameterServe()
	go model.AppWSManager.Start()
	carry.Maintain()
	select {}
}
