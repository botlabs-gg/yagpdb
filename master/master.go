package master

import (
	"encoding/binary"
	"github.com/sirupsen/logrus"
	"net"
	"sync"
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
