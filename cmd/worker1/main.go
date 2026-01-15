package main

import (
	"log"

	"github.com/Averianov/unsafethreadwrapper/cmd/worker1/service"
	"github.com/Averianov/unsafethreadwrapper/cmd/wrapper"
)

const (
	Name = "worker1"
)

func main() {
	wpr, err := wrapper.CreateWrapper(Name)
	if err != nil {
		log.Fatalf("%s", err.Error())
	}
	defer wpr.RegularStop()

	service.Srv(wpr)

	wpr.StartService("worker2") // инициировать запуск worker2 через master_sock
	//wpr.StopService(wpr.Name)   // "должен быть остановлен" - чтобы повторно не запускался
}
