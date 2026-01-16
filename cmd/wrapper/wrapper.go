package wrapper

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// ### JSON Types #######################################

type Sockets struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type Processes struct {
	Name         string   `json:"name"`
	MustStart    bool     `json:"must_start"`
	StInProgress bool     `json:"st_in_progress"`
	StLaunched   bool     `json:"st_launched"`
	Required     []string `json:"required"`
}

type JSONData struct {
	Sockets   []Sockets   `json:"sockets"`
	Processes []Processes `json:"processes"`
}

// ######################################################

const (
	MASTER             string = "master"
	JSON_PATH          string = "../processes.json"
	NUMBER_OF_ATTEMPTS int    = 5
)

type Wrapper struct {
	Name         string
	Sockets      map[string]net.Conn
	Processes    map[string]Processes
	SocketsPaths map[string]string
	NativeSocks  net.Listener
	StopChan     chan struct{} // if service can used regular stop channel
}

var (
	Wpr *Wrapper
)

func CreateWrapper(name string) (wpr *Wrapper, err error) {
	Wpr = &Wrapper{
		Name:         name,
		NativeSocks:  nil,
		Sockets:      make(map[string]net.Conn),
		Processes:    make(map[string]Processes),
		SocketsPaths: make(map[string]string),
		StopChan:     make(chan struct{}),
	}

	var j JSONData = JSONData{
		Sockets:   []Sockets{},
		Processes: []Processes{},
	}
	var fileBytes []byte
	fileBytes, err = os.ReadFile(JSON_PATH)
	if err != nil {
		fmt.Println(err)
		return
	}
	err = json.Unmarshal(fileBytes, &j)
	if err != nil {
		fmt.Println(err)
		return
	}

	var masterExist, nativeExist bool = false, false
	for _, socket := range j.Sockets {
		fmt.Printf("[%s] get socket %s from json\n", Wpr.Name, socket.Name)
		Wpr.SocketsPaths[socket.Name] = socket.Path

		if socket.Name == name {
			nativeExist = true
			os.Remove(socket.Path)
			Wpr.NativeSocks, err = net.Listen("unix", socket.Path)
			if err != nil {
				err = fmt.Errorf("[%s] Cannot listen on %s: %v\n", Wpr.Name, socket.Path, err)
				return
			}
			continue
		}
		if socket.Name == MASTER {
			masterExist = true
		}
	}
	if !masterExist || !nativeExist {
		err = fmt.Errorf("[%s] Master(%v) or Native(%v) socket not founded\n", Wpr.Name, masterExist, nativeExist)
		return
	}

	for _, process := range j.Processes {
		Wpr.Processes[process.Name] = process
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	signal.Notify(sig, syscall.SIGUSR1) // for cooperative shutdown
	stop := make(chan struct{})
	go func() {
		<-sig
		fmt.Printf("[%s] Cooperative shutdown (SIGUSR1)\n", Wpr.Name)
		close(stop)
	}()

	Wpr.sendToMaster(fmt.Sprintf("launched %s", Wpr.Name), 0)
	return Wpr, nil
}

func (wpr *Wrapper) RegularStop() {
	wpr.NativeSocks.Close()
	close(wpr.StopChan)
	time.Sleep(2 * time.Second)
}

func (wpr *Wrapper) StartService(serviceName string) (err error) {
	if _, ok := wpr.SocketsPaths[serviceName]; ok {
		err = wpr.sendToMaster(fmt.Sprintf("start %s", serviceName), 0)
	} else {
		err = fmt.Errorf("[%s] Service %s not found\n", wpr.Name, serviceName)
	}
	if err != nil {
		fmt.Println(err)
	}
	return
}

func (wpr *Wrapper) StopService(serviceName string) (err error) {
	if _, ok := wpr.Processes[serviceName]; ok {
		err = wpr.sendToMaster(fmt.Sprintf("stop %s", serviceName), 0)
	} else {
		err = fmt.Errorf("[%s] Service %s not found\n", wpr.Name, serviceName)
	}
	if err != nil {
		fmt.Println(err)
	}
	return
}

// Кастыль, пока не будет реализовано в Мастере
func (wpr *Wrapper) StopServiceRecursive(serviceName string) (err error) {
	fmt.Printf("[%s] Recursive Stop %s\n", wpr.Name, serviceName)
	for _, process := range wpr.Processes {
		for _, sub := range process.Required {
			if sub == serviceName { // если у какого либо сервиса есть зависимость от текущего, то его так же останавливаем рекурсивно
				err = wpr.StopServiceRecursive(process.Name)
				if err != nil {
					fmt.Println(err)
					return
				}
				break
			}
		}
	}
	err = wpr.StopService(serviceName)
	return
}

func (wpr *Wrapper) SendToService(serviceName, msg string) (err error) {
	err = wpr.reconnectSocket(serviceName, wpr.SocketsPaths[serviceName])
	if err != nil {
		fmt.Println(err)
		return
	}

	_, err = wpr.Sockets[serviceName].Write([]byte(fmt.Sprintf("%s\n", msg)))
	if err != nil {
		err = fmt.Errorf("[%s] Failed send to %s: %v\n", wpr.Name, serviceName, err)
		fmt.Println(err)
	}

	wpr.Sockets[serviceName].Close()
	delete(wpr.Sockets, serviceName)
	return
}

func (wpr *Wrapper) reconnectSocket(socketName, socketPath string) (err error) {
	if _, ok := wpr.SocketsPaths[socketName]; ok {
		if _, ok := wpr.Sockets[socketName]; ok {
			wpr.Sockets[socketName].Close()
			delete(wpr.Sockets, socketName)
		}
		wpr.Sockets[socketName], err = net.Dial("unix", socketPath)
		if err != nil {
			err = fmt.Errorf("[%s] No connection to %s\n", wpr.Name, socketName)
			log.Fatalf("%v", err)
		}
	} else {
		err = fmt.Errorf("[%s] SocketPath of service %s not found\n", wpr.Name, socketName)
		log.Fatalf("%v", err)
	}
	return
}

func (wpr *Wrapper) sendToMaster(msg string, try int) (err error) {
	fmt.Printf("[%s] Command to Master %s, try %d\n", wpr.Name, msg, try)
	err = wpr.SendToService(MASTER, msg)
	if err != nil && try < NUMBER_OF_ATTEMPTS {
		try++
		time.Sleep(500 * time.Millisecond) // 0.5 sec
		err = wpr.sendToMaster(msg, try)
	}
	return
}
