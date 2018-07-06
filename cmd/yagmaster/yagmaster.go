package main

import (
	"github.com/jonas747/yagpdb/master"
	"time"
)

func main() {
	go master.StartSlave()

	time.Sleep(d)
}
