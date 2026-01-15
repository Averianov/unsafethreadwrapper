package service

import (
	"fmt"
	"time"

	"github.com/Averianov/unsafethreadwrapper/cmd/wrapper"
)

// Работа
func Srv(wpr *wrapper.Wrapper) {
	for i := 0; i < 10; i++ {
		select {
		case <-wpr.StopChan:
			fmt.Printf("[%s] Stopping worker from Channel\n", wpr.Name)
			return
		default:
			wpr.SendToService("logger", fmt.Sprintf("%s - %d", wpr.Name, i))
			time.Sleep(3 * time.Second)
		}
	}
	fmt.Printf("[%s] End task by timeout\n", wpr.Name)
}
