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
	Name                = "logger"
	MASTER_SOCKET_PATH  = "/tmp/master.sock"
	SERVICE_SOCKET_PATH = fmt.Sprintf("/tmp/%s.sock", Name)
)

func main() {
	os.Remove(SERVICE_SOCKET_PATH)
	ln, err := net.Listen("unix", SERVICE_SOCKET_PATH)
	if err != nil {
		fmt.Printf("[%s] Cannot listen on %s: %v\n", Name, SERVICE_SOCKET_PATH, err)
		return
	}
	defer ln.Close()
	conn, err := net.Dial("unix", MASTER_SOCKET_PATH)
	if err != nil {
		fmt.Printf("[%s] Cannot connect to master_sock: %v\n", Name, err)
		return
	}
	cmd := fmt.Sprintf("launched %s\n", Name)
	conn.Write([]byte(cmd))
	if err != nil {
		fmt.Printf("[%s] Failed to send status launched: %v\n", Name, err)
		return
	}
	fmt.Printf("[%s] Sent command to master: %s", Name, cmd)

	defer func() {
		cmd = fmt.Sprintf("stop %s\n", Name)
		conn.Write([]byte(cmd))
		if err != nil {
			fmt.Printf("[%s] Failed to send status stop: %v\n", Name, err)
			return
		}
		fmt.Printf("[%s] Sent command to master: %s", Name, cmd)
		cmd = fmt.Sprintf("stopped %s\n", Name)
		conn.Write([]byte(cmd))
		if err != nil {
			fmt.Printf("[%s] Failed to send status stopped: %v\n", Name, err)
			return
		}
		fmt.Printf("[%s] Sent command to master: %s", Name, cmd)
		conn.Close()
	}()

	// #########################################################

	// Shutdown by signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	stop := make(chan struct{})
	go func() {
		<-sig
		fmt.Printf("[%s] Received shutdown signal\n", Name)
		close(stop)
	}()

	fmt.Printf("[%s] Listening on %s\n", Name, SERVICE_SOCKET_PATH)
	for {
		select {
		case <-stop:
			fmt.Printf("[%s] Shutting down...\n", Name)
			return
		default:
			// Используем неблокирующий accept с таймаутом
			ln.(*net.UnixListener).SetDeadline(time.Now().Add(1 * time.Second))
			conn, err := ln.Accept()
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
			fmt.Printf("[%s] %s\n", name, string(buf[:n]))
		}
		if err != nil {
			return
		}
	}
}
