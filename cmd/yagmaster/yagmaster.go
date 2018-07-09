package main

import (
	"github.com/jonas747/yagpdb/master"
	"github.com/sirupsen/logrus"
	"os"
)

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{
		ForceColors: true,
	})

	master.Listen(os.Getenv("YAGPDB_MASTER_LISTEN_ADDR"))
}
