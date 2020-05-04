package messagestatscollector

import (
	"context"
	"time"

	"emperror.dev/errors"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/sirupsen/logrus"
)

// Collector is a message stats collector which will preiodically update the serberstats messages table with stats
type Collector struct {
	MsgEvtChan chan *discordgo.Message

	interval time.Duration

	channels map[int64]*entry
	// buf      []*discordgo.Message
	// channels []int64
	l *logrus.Entry
}

type entry struct {
	GuildID   int64
	ChannelID int64
	Count     int64
}

// NewCollector creates a new Collector
func NewCollector(l *logrus.Entry, updateInterval time.Duration) *Collector {
	col := &Collector{
		MsgEvtChan: make(chan *discordgo.Message, 1000),
		interval:   updateInterval,
		l:          l,
		channels:   make(map[int64]*entry),
	}

	go col.run()

	return col
}

func (c *Collector) run() {
	ticker := time.NewTicker(time.Minute * 5)
	defer ticker.Stop()

	for {
		select {
		case msg := <-c.MsgEvtChan:
			c.handleIncMessage(msg)
		case <-ticker.C:
			err := c.flush()
			if err != nil {
				c.l.Errorf("failed updating temp serverstats: %+v", err)
			}
		}
	}
}

func (c *Collector) handleIncMessage(msg *discordgo.Message) {
	if c, ok := c.channels[msg.ChannelID]; ok {
		c.Count++
		return
	}

	c.channels[msg.ChannelID] = &entry{
		GuildID:   msg.GuildID,
		ChannelID: msg.ChannelID,
		Count:     1,
	}
}

func (c *Collector) flush() error {
	c.l.Infof("message stats collector is flushing: lc: %d", len(c.channels))
	if len(c.channels) < 1 {
		return nil
	}

	const updateQuery = `
	INSERT INTO server_stats_hourly_periods_messages (guild_id, t, channel_id, count) 
	VALUES ($1, $2, $3, $4)
	ON CONFLICT (guild_id, channel_id, t) DO UPDATE
	SET count = server_stats_hourly_periods_messages.count + $4`

	tx, err := common.PQ.BeginTx(context.Background(), nil)
	if err != nil {
		return errors.WithStackIf(err)
	}

	for _, v := range c.channels {
		_, err := tx.Exec(updateQuery, v.GuildID, RoundHour(time.Now()), v.ChannelID, v.Count)
		if err != nil {
			tx.Rollback()
			return errors.WithStackIf(err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return errors.WithStackIf(err)
	}

	// reset buffers
	c.channels = make(map[int64]*entry)

	return nil
}

func RoundHour(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
}
