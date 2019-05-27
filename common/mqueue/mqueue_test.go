package mqueue

import (
	"io"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
)

var ()

type RoundTripper struct {
	T         *testing.T
	Expecting string
	Timeout   time.Duration

	ResChan chan bool
}

type FakeReadCloser struct {
}

func (f *FakeReadCloser) Read(b []byte) (n int, err error) {
	b[0] = '{'
	b[1] = '}'
	return 2, io.EOF
}

func (f *FakeReadCloser) Close() error {
	return nil
}

func (r *RoundTripper) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	defer func() {
		r.ResChan <- true
	}()

	resp = &http.Response{
		StatusCode: 200,
		Body:       &FakeReadCloser{},
	}

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		r.T.Error("Error reading body", err)
		return
	}

	if string(body) != r.Expecting {
		r.T.Error("Unexpected output, got ", string(body), ", expected ", r.Expecting)
		return
	}

	return
}

func init() {
	common.InitTest()
	common.BotSession, _ = discordgo.New("")

	common.PQ.Exec(`DROP TABLE IF EXISTS mqueue;
CREATE TABLE mqueue (
	id serial NOT NULL PRIMARY KEY,
	source text NOT NULL,
	source_id text NOT NULL,
	message_str text NOT NULL,
	message_embed text NOT NULL,
	channel text NOT NULL,
	processed boolean NOT NULL
);`)

	InitStores()
}

// Full integration test
func TestQueue(t *testing.T) {
	transport := &RoundTripper{
		T:         t,
		Expecting: `{"content":"hello world","tts":false}`,
		ResChan:   make(chan bool),
	}
	client := &http.Client{
		Transport: transport,
	}
	common.BotSession.Client = client

	go StartPolling()

	QueueMessageString("0", "0", "123", "hello world")

	select {
	case <-time.After(time.Second * 5):
		t.Error("Timed out")
		return
	case <-transport.ResChan:
		if t.Failed() {
			return
		}
	}

	transport.Expecting = `{"embed":{"type":"rich","title":"hello title"},"tts":false}`
	QueueMessageEmbed("0", "0", "123", &discordgo.MessageEmbed{Title: "hello title"})

	select {
	case <-time.After(time.Second * 5):
		t.Error("Timed out")
		return
	case <-transport.ResChan:
	}
}
