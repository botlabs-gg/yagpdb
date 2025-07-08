package methodutil

import (
	"time"
)

// PollMethod is a utility function to help repeatidly call a method until a desired condition is met.
//
// The method will be called at a specified interval. The data returned should be a pointer.
// The handler will return a function method to be called when the desired condition is met.
//
// If the interval is 0, it will default to 5 seconds.
func PollMethod(method func(func()), interval time.Duration) {
	if interval == 0 {
		interval = 5 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	finished := make(chan bool)
	done := func() {
		close(finished)
	}

	for {
		select {
		case <-ticker.C:
			method(done)
		case <-finished:
			return
		}
	}
}
