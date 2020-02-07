package bot

import (
	"sync"
	"testing"
	"time"
)

func TestDeleteQueue(t *testing.T) {
	q := &messageDeleteQueue{
		channels: make(map[int64]*messageDeleteQueueChannel),
	}

	toDel := []int64{1, 2, 3, 4, 5}

	c := make(chan bool)
	q.customdeleteFunc = func(channel int64, ids []int64) error {
		if len(ids) != len(toDel) {
			t.Error("incorrect number being batch deleted: ", len(ids), ", ", len(toDel))
		}
		c <- true

		return nil
	}

	q.DeleteMessages(0, 1, toDel...)

	select {
	case <-time.After(time.Second):
		t.Error("never got callback")
	case <-c:
		return
	}
}

func TestDeleteQueueSplit(t *testing.T) {
	q := &messageDeleteQueue{
		channels: make(map[int64]*messageDeleteQueueChannel),
	}

	toDel := make([]int64, 0, 1000)
	for i := 0; i < 1000; i++ {
		toDel = append(toDel, int64(i))
	}

	deleted := make([]int64, 0, 1000)
	var deletedLock sync.Mutex

	c := make(chan bool)
	q.customdeleteFunc = func(channel int64, ids []int64) error {
		if len(ids) > 100 {
			t.Error("attempting to delete more than 100")
		}

		deletedLock.Lock()
		deleted = append(deleted, ids...)
		l := len(deleted)
		deletedLock.Unlock()

		if l == len(toDel) {
			c <- true // done
		}

		return nil
	}

	q.DeleteMessages(0, 1, toDel...)

	select {
	case <-time.After(time.Second):
		t.Error("never got callback")
	case <-c:

	OUTER:
		for _, td := range toDel {
			for _, d := range deleted {
				if td == d {
					continue OUTER
				}
			}

			// didn't find
			t.Error("didn't delete: ", td)
		}

		return
	}
}
