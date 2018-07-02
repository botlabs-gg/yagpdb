package master

import (
	"sync"
)

var idgen = make(chan int64)
var generatorOnce sync.Once

func getNewID() int64 {
	generatorOnce.Do(func() {
		go func() {
			curID := int64(0)
			for {
				idgen <- curID
				curID++
			}
		}()
	})

	return <-idgen
}
