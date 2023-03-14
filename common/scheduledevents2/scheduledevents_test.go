package scheduledevents2

import (
	"sync"
	"testing"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2/models"
)

var SchemaInit = false

func init() {
	common.InitTest()

	// err := InitSchema()
	// if err != nil {
	// 	log.Println("Unable to initialize schema: ", err)
	// } else {
	// 	SchemaInit = true
	// }

	// bot.TotalShardCount = 1
	// bot.ProcessShardCount = 1
}

func testStopPlugin(p *ScheduledEvents) {
	var wg sync.WaitGroup
	wg.Add(1)
	p.StopBot(&wg)
	wg.Wait()

	registeredHandlers = make(map[string]*RegisteredHandler)
}

// func TestSchema(t *testing.T) {
// 	_, err := common.PQ.Exec("DROP TABLE IF EXISTS scheduled_events;")
// 	if err != nil {
// 		t.Error("failed dropping table: ", err)
// 		return
// 	}

// 	err = InitSchema()
// 	if err != nil {
// 		t.Error(err)
// 	}
// }

func TestScheduleHandle(t *testing.T) {
	if !SchemaInit {
		t.Skip("schema was not initilized, skipping.")
		return
	}

	doneChan := make(chan bool)
	RegisterHandler("test", nil, func(evt *models.ScheduledEvent, data interface{}) (retry bool, err error) {
		doneChan <- true
		return false, nil
	})

	p := newScheduledEventsPlugin()
	go p.runCheckLoop()
	defer testStopPlugin(p)

	err := ScheduleEvent("test", 0, time.Now().Add(time.Second), nil)
	if err != nil {
		t.Error("failed scheduling event: ", err)
		return
	}

	select {
	case <-time.After(time.Second * 5):
		t.Error("never handled event")
		return
	case <-doneChan:
		return // Success
	}

}

func TestScheduleHandleSlow(t *testing.T) {
	if !SchemaInit {
		t.Skip("schema was not initilized, skipping.")
		return
	}

	doneChan := make(chan bool)
	RegisterHandler("test", nil, func(evt *models.ScheduledEvent, data interface{}) (retry bool, err error) {
		doneChan <- true
		return false, nil
	})

	p := newScheduledEventsPlugin()
	go p.runCheckLoop()
	defer testStopPlugin(p)

	sent := time.Now()

	err := ScheduleEvent("test", 0, sent.Add(time.Second*3), nil)
	if err != nil {
		t.Error("failed scheduling event: ", err)
		return
	}

	select {
	case <-time.After(time.Second * 10):
		t.Error("never handled event")
		return
	case <-doneChan:
		since := time.Since(sent)
		if since < time.Second*2 {
			t.Error("too fast: ", since)
		} else if since > time.Second*5 {
			t.Error("too slow: ", since)
		}

		return // Success
	}
}

type TestData struct {
	A bool
	B string
}

func TestScheduleHandleWithData(t *testing.T) {
	if !SchemaInit {
		t.Skip("schema was not initilized, skipping.")
		return
	}

	input := TestData{
		A: true,
		B: "hello",
	}

	doneChan := make(chan bool)
	RegisterHandler("test", TestData{}, func(evt *models.ScheduledEvent, data interface{}) (retry bool, err error) {
		cast, ok := data.(*TestData)
		if !ok {
			t.Error("failed casting data")
		} else {
			if cast.A != input.A {
				t.Error("cast.A != ", input.A)
			} else if cast.B != input.B {
				t.Error("cast.B (", cast.B, ") != ", input.B)
			}
		}

		doneChan <- true
		return false, nil
	})

	p := newScheduledEventsPlugin()
	go p.runCheckLoop()
	defer testStopPlugin(p)

	err := ScheduleEvent("test", 0, time.Now(), input)
	if err != nil {
		t.Error("failed scheduling event: ", err)
		return
	}

	select {
	case <-time.After(time.Second * 5):
		t.Error("never handled event")
		return
	case <-doneChan:
		return // Success
	}
}
