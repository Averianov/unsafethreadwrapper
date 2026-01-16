package main

import (
	"fmt"
	"log"
	"time"

	"github.com/Averianov/unsafethreadwrapper/pkg/wrapper"
)

var (
	Name = "worker2"
)

// В конце останавливает себя и логгер
func main() {
	wpr, err := wrapper.CreateWrapper(Name)
	if err != nil {
		log.Fatalf("%s", err.Error())
	}
	defer wpr.RegularStop()

	defer wpr.StopService(wpr.Name)
	// defer wpr.StopServiceRecursive("logger")
	// Работа
	for i := 0; i < 10; i++ {
		select {
		case <-wpr.StopChan:
			fmt.Printf("[%s] Stopping worker from Channel\n", wpr.Name)
			return
		default:
			wpr.SendToService("logger", fmt.Sprintf("%s - %d", wpr.Name, i))
			time.Sleep(1 * time.Second)
		}
	}

	// После завершения работы — инициировать остановку logger через master_sock
	fmt.Printf("[%s] End task by timeout\n", wpr.Name)

}
