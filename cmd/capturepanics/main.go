package main

import (
	"bufio"
	"container/ring"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

var idgen = make(chan int64)

func idFiller() {
	var i int64
	for {
		idgen <- i
		i++
	}
}

func main() {
	if len(os.Args) < 2 {
		os.Exit(1)
	}

	err := os.Mkdir("shutdown_logs", 0755)
	if err != nil && !os.IsExist(err) {
		fmt.Println("Failed creating panic log folder: ", err)
		os.Exit(1)
	}

	go idFiller()

	cmdName := os.Args[1]
	args := os.Args[2:]

	cmd := exec.Command(cmdName, args...)
	cmd.Env = os.Environ()

	stdErr, err := cmd.StderrPipe()
	stdOut, err2 := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("failed creating stderr pipe: ", err)
		os.Exit(1)
	}
	if err2 != nil {
		fmt.Println("failed creating stdout pipe: ", err2)
		os.Exit(1)
	}

	go reader(stdErr, "stderr")
	go reader(stdOut, "stdout")

	err = cmd.Run()
	if err != nil {
		fmt.Println("returned error: ", err)
		time.Sleep(time.Second * 10) // make sure all output is processed before exiting
		os.Exit(1)
	}

	fmt.Println("Exited")
	time.Sleep(time.Second * 10) // make sure all output is processed before exiting
}

func reader(reader io.ReadCloser, logPrefix string) {
	r := ring.New(50000)

	bufReader := bufio.NewReader(reader)

	running := true
	for running {
		line, err := bufReader.ReadBytes('\n')
		if err != nil {
			running = false
			fmt.Println("Read Err: ", err, len(line))
		}

		if len(line) < 1 {
			continue
		}

		r.Value = line
		r = r.Prev()
	}

	writeLogs(logPrefix, r)
}

func writeLogs(prefix string, r *ring.Ring) {
	compiled := make([]byte, 0, 10000000)

	r.Do(func(p interface{}) {
		if p != nil {
			v := p.([]byte)
			compiled = append(compiled, v...)
		}
	})

	id := <-idgen
	now := time.Now().Format("2006-01-02T15-04-05")
	fName := fmt.Sprintf("%s_shutdown_%s_%d.log", prefix, now, id)

	f, err := os.Create("shutdown_logs/" + fName)
	if err != nil {
		fmt.Println("Failed writing panic logs: ", err)
	} else {
		defer f.Close()
		f.Write(compiled)
	}
}
