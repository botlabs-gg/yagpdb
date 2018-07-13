package master

import (
	"github.com/sirupsen/logrus"
	"net"
	"os"
	"os/exec"
	"sync"
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

	waitForClients(listener)
}

func StartSlave() {
	logrus.Println("Starting slave")
	cmd := exec.Command("yagpdb", "-bot")
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

type Message struct {
	EvtID EventType
	Body  []byte
}
