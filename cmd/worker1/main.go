package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	Name                = "worker1"
	MASTER_SOCKET_PATH  = "/tmp/master.sock"
	SERVICE_SOCKET_PATH = fmt.Sprintf("/tmp/%s.sock", Name)
	LOGGER_SOCKET_PATH  = "/tmp/logger.sock"
)

func main() {
	var err error
	var service_sock net.Listener
	var master_sock net.Conn
	var logger_sock net.Conn

	os.Remove(SERVICE_SOCKET_PATH)
	service_sock, err = net.Listen("unix", SERVICE_SOCKET_PATH)
	if err != nil {
		fmt.Printf("[%s] Cannot listen on %s: %v\n", Name, SERVICE_SOCKET_PATH, err)
		return
	}
	defer service_sock.Close()

	master_sock, err = net.Dial("unix", MASTER_SOCKET_PATH)
	if err != nil {
		fmt.Printf("[%s] Cannot connect to master_sock: %v\n", Name, err)
		return
	}
	cmd := fmt.Sprintf("launched %s\n", Name)
	master_sock.Write([]byte(cmd))
	if err != nil {
		fmt.Printf("[%s] Failed to send status launched: %v\n", Name, err)
		return
	}
	fmt.Printf("[%s] Sent command to master: %s", Name, cmd)

	defer func() {
		cmd = fmt.Sprintf("stop %s\n", Name)
		_, err = master_sock.Write([]byte(cmd))
		if err != nil {
			fmt.Printf("[%s] Failed to send status stop: %v\n", Name, err)
			return
		}
		fmt.Printf("[%s] Sent command to master: %s", Name, cmd)
		cmd = fmt.Sprintf("stopped %s\n", Name)
		_, err = master_sock.Write([]byte(cmd))
		if err != nil {
			fmt.Printf("[%s] Failed to send status stopped: %v\n", Name, err)
			return
		}
		fmt.Printf("[%s] Sent command to master: %s", Name, cmd)
		master_sock.Close()
	}()

	// #########################################################
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	signal.Notify(sig, syscall.SIGUSR1) // for cooperative shutdown
	stop := make(chan struct{})
	go func() {
		<-sig
		fmt.Printf("[%s] Cooperative shutdown (SIGUSR1)\n", Name)
		close(stop)
	}()

	for {
		logger_sock, err = net.Dial("unix", LOGGER_SOCKET_PATH)
		if err != nil {
			fmt.Printf("[%s] No connection to logger, waiting...\n", Name)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		defer logger_sock.Close()

		fmt.Printf("[%s] Connection to logger established\n", Name)
		for i := 0; i < 10; i++ {
			select {
			case <-stop:
				fmt.Printf("[WORKER %s] Stopping worker\n", Name)
				// После завершения работы — инициировать запуск worker2 через master_sock
				cmd = "start worker2\n"
				_, err = master_sock.Write([]byte(cmd))
				if err != nil {
					fmt.Printf("[%s] Failed to send start worker2: %v\n", Name, err)
					return
				}
				fmt.Printf("[%s] Sent command to master: %s", Name, cmd)
			default:
				msg := fmt.Sprintf("%s - %d", Name, i)
				logger_sock.Write([]byte(msg))
				time.Sleep(2 * time.Second)
			}
		}
		fmt.Printf("[%s] End task by timeout\n", Name)

		// После завершения работы — инициировать запуск worker2 через master_sock
		cmd = "start worker2\n"
		_, err = master_sock.Write([]byte(cmd))
		if err != nil {
			fmt.Printf("[%s] Failed to send start worker2: %v\n", Name, err)
			return
		}
		fmt.Printf("[%s] Sent command to master: %s", Name, cmd)
		return
	}
}
