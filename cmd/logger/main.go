package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/Averianov/unsafethreadwrapper/cmd/wrapper"
)

var (
	Name = "logger"
)

func main() {
	wpr, err := wrapper.CreateWrapper(Name)
	if err != nil {
		log.Fatalf("%s", err.Error())
	}
	defer wpr.RegularStop()

	//### Work #################################################################
	fmt.Printf("[%s] Listening on %s\n", Name, wpr.NativeSocks.Addr())
	for {
		select {
		case <-wpr.StopChan:
			fmt.Printf("[%s] Stopping worker from Channel\n", wpr.Name)
			return
		default:
			// Используем неблокирующий accept с таймаутом
			wpr.NativeSocks.(*net.UnixListener).SetDeadline(time.Now().Add(1 * time.Second))
			conn, err := wpr.NativeSocks.Accept()
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				break
			}
			handleConn(Name, conn)
		}
	}
}

func handleConn(name string, conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			fmt.Printf("[%s] %s", name, string(buf[:n]))
		}
		if err != nil {
			return
		}
	}
}
