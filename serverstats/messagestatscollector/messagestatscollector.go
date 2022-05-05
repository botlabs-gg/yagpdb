package messagestatscollector

import (
	"strconv"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/mediocregopher/radix/v3"
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
		MsgEvtChan: make(chan *discordgo.Message, 10000),
		interval:   updateInterval,
		l:          l,
		channels:   make(map[int64]*entry),
	}

	go col.run()

	return col
}

func (c *Collector) run() {
	ticker := time.NewTicker(c.interval)
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

func KeyMessageStats(guildID int64, year, day int) string {
	return "serverstats_message_stats:" + strconv.FormatInt(guildID, 10) + ":" + strconv.Itoa(year) + ":" + strconv.Itoa(day)
}
func KeyActiveGuilds(year, day int) string {
	return "serverstats_active_guilds:" + strconv.Itoa(year) + ":" + strconv.Itoa(day)
}

func (c *Collector) flush() error {
	sleepBetweenCalls := time.Second
	if len(c.channels) > 0 {
		sleepBetweenCalls = c.interval / time.Duration(len(c.channels))
		sleepBetweenCalls /= 2
	}

	c.l.Infof("message stats collector is flushing: lc: %d, sleep: %s", len(c.channels), sleepBetweenCalls.String())
	if len(c.channels) < 1 {
		return nil
	}

	ticker := time.NewTicker(sleepBetweenCalls)
	defer ticker.Stop()

	t := time.Now().UTC()
	day := t.YearDay()
	year := t.Year()
	for k, v := range c.channels {
		err := common.RedisPool.Do(radix.FlatCmd(nil, "ZINCRBY", KeyMessageStats(v.GuildID, year, day), v.Count, v.ChannelID))
		if err != nil {
			return err
		}

		err = common.RedisPool.Do(radix.FlatCmd(nil, "SADD", KeyActiveGuilds(year, day), v.GuildID))
		if err != nil {
			return err
		}
		delete(c.channels, k)
		<-ticker.C
	}

	return nil
}

func RoundHour(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
}
