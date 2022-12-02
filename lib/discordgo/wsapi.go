package discordgo

import (
	"encoding/json"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

type wsWriter struct {
	session *Session

	conn               net.Conn
	closer             chan interface{}
	incoming           chan outgoingEvent
	sendCloseQueue     chan []byte
	readyRecv          chan bool
	waitingForIdentify bool
	queuedEvents       []*outgoingEvent

	writer *wsutil.Writer

	sendRatelimiter chan bool
}

func newWSWriter(conn net.Conn, session *Session, closer chan interface{}) *wsWriter {
	return &wsWriter{
		conn:           conn,
		session:        session,
		closer:         closer,
		incoming:       make(chan outgoingEvent, 10),
		sendCloseQueue: make(chan []byte),
		readyRecv:      make(chan bool),
	}
}

func (w *wsWriter) Run() {
	w.sendRatelimiter = make(chan bool)
	go w.runSendRatelimiter()

	w.writer = wsutil.NewWriter(w.conn, ws.StateClientSide, ws.OpText)

	for {
		select {
		case <-w.closer:
			select {
			// Ensure we send the close frame
			case body := <-w.sendCloseQueue:
				w.sendClose(body)
			default:
			}
			return
		case msg := <-w.incoming:
			if w.waitingForIdentify {
				w.queuedEvents = append(w.queuedEvents, &msg)
				continue
			}

			if msg.Operation == GatewayOPIdentify {
				w.waitingForIdentify = true
			}

			err := w.writeJson(msg)
			if err != nil {
				w.session.log(LogError, "Error writing to gateway: %s", err.Error())
				return
			}
		case body := <-w.sendCloseQueue:
			w.sendClose(body)

		case <-w.readyRecv:
			w.waitingForIdentify = false
			for _, msg := range w.queuedEvents {
				err := w.writeJson(msg)
				if err != nil {
					w.session.log(LogError, "Error writing to gateway: %s", err.Error())
					return
				}
			}

			w.queuedEvents = nil
		}
	}
}

func (w *wsWriter) runSendRatelimiter() {
	tc := time.NewTicker(time.Millisecond * 500)
	defer tc.Stop()
	defer func() {
		close(w.sendRatelimiter)
	}()

	for {
		select {
		case <-tc.C:
			select {
			case w.sendRatelimiter <- true:
			case <-w.closer:
				return
			}
		case <-w.closer:
			return
		}
	}
}

func (w *wsWriter) writeJson(data interface{}) error {
	serialized, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return w.writeRaw(serialized)
}

func (w *wsWriter) writeRaw(data []byte) error {
	<-w.sendRatelimiter

	_, err := w.writer.WriteThrough(data)
	if err != nil {
		return err
	}

	return w.writer.Flush()
}

func (w *wsWriter) sendClose(body []byte) error {
	_, err := w.conn.Write(body)
	return err

}

func (w *wsWriter) Queue(data outgoingEvent) {
	select {
	case <-time.After(time.Second * 10):
	case <-w.closer:
	case w.incoming <- data:
	}
}

func (w *wsWriter) QueueClose(body []byte) {
	select {
	case <-time.After(time.Second * 10):
	case <-w.closer:
	case w.sendCloseQueue <- body:
	}
}

type wsHeartBeater struct {
	sync.Mutex

	writer      *wsWriter
	sequence    *int64
	receivedAck bool
	missedAcks  int
	stop        chan interface{}

	// Called when we received no Ack from last heartbeat
	onNoAck func()

	lastAck  time.Time
	lastSend time.Time
}

func (wh *wsHeartBeater) ReceivedAck() {
	wh.Lock()
	wh.receivedAck = true
	wh.missedAcks = 0
	wh.lastAck = time.Now()
	wh.Unlock()
}

func (wh *wsHeartBeater) UpdateSequence(seq int64) {
	atomic.StoreInt64(wh.sequence, seq)
}

func (wh *wsHeartBeater) Run(interval time.Duration) {
	ticker := time.NewTicker(interval)

	for {
		select {
		case <-ticker.C:

			wh.Lock()
			hasReceivedAck := wh.receivedAck
			wh.receivedAck = false
			wh.Unlock()

			if !hasReceivedAck {
				wh.onNoAck()
			} else {
				wh.SendBeat()
			}
		case <-wh.stop:
			return
		}
	}
}

func (wh *wsHeartBeater) SendBeat() {
	wh.Lock()
	wh.lastSend = time.Now()
	wh.Unlock()

	seq := atomic.LoadInt64(wh.sequence)

	wh.writer.Queue(outgoingEvent{
		Operation: GatewayOPHeartbeat,
		Data:      seq,
	})
}

func (wh *wsHeartBeater) Times() (send time.Time, ack time.Time) {
	wh.Lock()
	send = wh.lastSend
	ack = wh.lastAck
	wh.Unlock()
	return
}
