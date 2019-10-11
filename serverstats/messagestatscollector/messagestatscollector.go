package messagestatscollector

import (
	"context"
	"time"

	"emperror.dev/errors"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/serverstats/models"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
)

// Collector is a message stats collector which will preiodically update the serberstats messages table with stats
type Collector struct {
	MsgEvtChan chan *discordgo.Message

	interval time.Duration

	buf      []*discordgo.Message
	channels []int64
	l        *logrus.Logger
}

// NewCollector creates a new Collector
func NewCollector(l *logrus.Logger, updateInterval time.Duration) *Collector {
	col := &Collector{
		MsgEvtChan: make(chan *discordgo.Message, 1000),
		interval:   updateInterval,
		l:          l,
	}

	go col.run()

	return col
}

func (c *Collector) run() {
	ticker := time.NewTicker(time.Minute)
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
	c.buf = append(c.buf, msg)
	for _, v := range c.channels {
		if v == msg.ChannelID {
			return
		}
	}

	c.channels = append(c.channels, msg.ChannelID)
}

func (c *Collector) flush() error {
	c.l.Debugf("message stats collector is flushing: l:%d, lc: %d", len(c.buf), len(c.channels))
	if len(c.buf) < 1 {
		return nil
	}

	channelStats := make([]*models.ServerStatsPeriod, 0, len(c.channels))

OUTER:
	for _, v := range c.buf {

		for _, cm := range channelStats {
			if cm.ChannelID.Int64 == v.ChannelID {
				cm.Count.Int64++
				continue OUTER
			}
		}

		created, err := v.Timestamp.Parse()
		if err != nil {
			c.l.WithError(err).Errorf("Message has invalid timestamp: %s (%d/%d/%d)", v.Timestamp, v.GuildID, v.ChannelID, v.ID)
			created = time.Now()
		}

		channelModel := &models.ServerStatsPeriod{
			GuildID:   null.Int64From(v.GuildID),
			ChannelID: null.Int64From(v.ChannelID),
			Started:   null.TimeFrom(created), // TODO: we should calculate these from the min max snowflake ids
			Duration:  null.Int64From(int64(time.Minute)),
			Count:     null.Int64From(1),
		}
		channelStats = append(channelStats, channelModel)
	}

	tx, err := common.PQ.BeginTx(context.Background(), nil)
	if err != nil {
		return errors.WithStackIf(err)
	}

	for _, model := range channelStats {
		err = model.Insert(context.Background(), tx, boil.Infer())
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
	if len(c.buf) > 100000 {
		// don't hog memory, above 100k is spikes, normal operation is around 2-8k/minute
		c.buf = nil
		c.channels = nil
	} else {
		c.buf = c.buf[:0]
		c.channels = c.channels[:0]
	}

	return nil
}
