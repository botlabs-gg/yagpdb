package cacheset

import (
	"reflect"
	"sync"
	"testing"
	"time"
)

func expectNoErr(t *testing.T, err error) {
	if err != nil {
		t.Fatal("expected no error")
	}
}

func TestGuildCacheBasic(t *testing.T) {
	man := NewManager(time.Second)

	fetchCounter := 0
	slot := man.RegisterSlot("test", func(key interface{}) (interface{}, error) {
		fetchCounter++
		return true, nil
	}, int64(10))

	checkValue := func(v interface{}) {
		cast, ok := v.(bool)
		if !ok {
			t.Fatal("value is not bool: ", reflect.TypeOf(v))
		}

		if !cast {
			t.Fatal("value is not true")
		}
	}
	v, err := slot.Get(1)
	expectNoErr(t, err)
	checkValue(v)

	if len(slot.values) != 1 {
		t.Fatal("value was not added to cache")
	}

	v2, err := slot.Get(1)
	expectNoErr(t, err)
	checkValue(v2)

	if len(slot.values) != 1 {
		t.Fatal("slot values incorrect")
	}

	if fetchCounter != 1 {
		t.Fatal("fetch counter is not 1")
	}

	slot.gc(time.Now().Add(time.Hour))

	if len(slot.values) != 0 {
		t.Fatal("value was not remvoed from cache")
	}

	v3, err := slot.Get(1)
	expectNoErr(t, err)
	checkValue(v3)

	if fetchCounter != 2 {
		t.Fatal("fetch counter is not 2")
	}
}

type resEntry struct {
	err error
	v   interface{}
}

// this tests the waiting and broadcast of the cache
// and also tests to make sure only 1 fetch was triggered
func TestWaiting(t *testing.T) {
	man := NewManager(time.Second)

	fetchCounter := 0
	slot := man.RegisterSlot("test", func(key interface{}) (interface{}, error) {
		fetchCounter++
		time.Sleep(time.Second)
		return true, nil
	}, int64(10))

	checkValue := func(v interface{}) {
		cast, ok := v.(bool)
		if !ok {
			t.Fatal("value is not bool: ", reflect.TypeOf(v))
		}

		if !cast {
			t.Fatal("value is not true")
		}
	}

	resChan := make(chan resEntry)
	nGoroutines := 100

	for i := 0; i < nGoroutines; i++ {
		go func() {
			vg, errg := slot.Get(1)
			resChan <- resEntry{
				err: errg,
				v:   vg,
			}
		}()
	}

	v2, err := slot.Get(1)
	expectNoErr(t, err)
	checkValue(v2)

	for i := 0; i < nGoroutines; i++ {
		// check all results
		gres := <-resChan
		expectNoErr(t, gres.err)
		checkValue(gres.v)
	}

	if len(slot.values) != 1 {
		t.Fatal("slot values incorrect")
	}

	if fetchCounter != 1 {
		t.Fatal("fetch counter is not 1")
	}
}

// this tests to make sure we can fire multiple concurrent fetches
func TestConcurrentFetch(t *testing.T) {
	man := NewManager(time.Second)

	fetchCounter := 0
	var fetchmu sync.Mutex
	var otherWaiting chan bool
	slot := man.RegisterSlot("test", func(key interface{}) (interface{}, error) {
		fetchmu.Lock()
		fetchCounter++

		if otherWaiting != nil {
			otherWaiting <- true
			fetchmu.Unlock()
		} else {
			otherWaiting = make(chan bool)
			fetchmu.Unlock()
			select {
			case <-otherWaiting:
			case <-time.After(time.Second):
				panic("did not get concurrent fetch after 1 second")
			}
		}

		// time.Sleep(time.Second)
		return true, nil
	}, int64(10))

	checkValue := func(v interface{}) {
		cast, ok := v.(bool)
		if !ok {
			t.Fatal("value is not bool: ", reflect.TypeOf(v))
		}

		if !cast {
			t.Fatal("value is not true")
		}
	}

	resChan := make(chan resEntry)
	nGoroutines := 100

	for i := 0; i < nGoroutines; i++ {
		go func(id int) {
			vg, errg := slot.Get(id)
			resChan <- resEntry{
				err: errg,
				v:   vg,
			}
		}(i % 2)
	}

	v2, err := slot.Get(1)
	expectNoErr(t, err)
	checkValue(v2)

	for i := 0; i < nGoroutines; i++ {
		// check all results
		gres := <-resChan
		expectNoErr(t, gres.err)
		checkValue(gres.v)
	}

	if len(slot.values) != 2 {
		t.Fatal("slot values incorrect: ", len(slot.values))
	}

	if fetchCounter != 2 {
		t.Fatal("fetch counter is not 1: ", fetchCounter)
	}
}
