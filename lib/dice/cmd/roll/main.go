package main

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/justinian/dice"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	rollDesc := strings.Join(os.Args[1:], " ")
	res, reason, err := dice.Roll(rollDesc)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v", err)
	} else {
		if reason != "" {
			fmt.Printf("%s: ", reason)
		}
		fmt.Println(res)
		fmt.Println(res.Description())
	}
}
