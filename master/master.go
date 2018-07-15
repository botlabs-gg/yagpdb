package master

import (
	"github.com/sirupsen/logrus"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"
)

var (
	mainSlave *SlaveConn
	newSlave  *SlaveConn
	mu        sync.Mutex
)

func Listen(addr string) {
	logrus.Println("Starting master on ", addr)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		logrus.WithError(err).Fatal("Failed starting master")
	}
	go monitorSlaves()
	waitForClients(listener)
}

func StartSlave() {
	logrus.Println("Starting slave")
	cmd := exec.Command("./yagpdb", "-bot", "-syslog")
	cmd.Env = os.Environ()

	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	cmd.Dir = wd

	err = cmd.Start()
	if err != nil {
		logrus.Println("Error starting slave: ", err)
	}
}

func waitForClients(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			logrus.WithError(err).Error("Failed accepting slave")
			continue
		}

		logrus.Println("New client connected")
		client := NewSlaveConn(conn)
		go client.listen()
	}
}

func monitorSlaves() {
	ticker := time.NewTicker(time.Second)
	lastTimeSawSlave := time.Now()

	for {
		<-ticker.C
		mu.Lock()
		if mainSlave != nil {
			lastTimeSawSlave = time.Now()
		}
		mu.Unlock()

		if time.Since(lastTimeSawSlave) > time.Second*15 {
			logrus.Println("Haven't seen a slave in 15 seconds, starting a new one now")
			go StartSlave()
			lastTimeSawSlave = time.Now()
		}
	}
}

type Message struct {
	EvtID EventType
	Body  []byte
}
