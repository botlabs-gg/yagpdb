package main

import (
	"github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"syscall"
)

func ListenSignal() {

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill, syscall.SIGUSR1)
	for {
		sign := <-sc
		if sign == syscall.SIGUSR1 {
			LaunchNewSlave()
		} else {
			logrus.Println("Got ", sign.String())
			os.Exit(0)
		}
	}
}
