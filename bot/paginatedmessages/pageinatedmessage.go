package paginatedmessages

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
	"strconv"
	"sync"
	"time"
)

var (
	logger                  = common.GetPluginLogger(&Plugin{})
	activePaginatedMessages []*PaginatedMessage
	menusLock               sync.Mutex
)

type Plugin struct{}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Paginated Messages",
		SysName:  "paginatedmessages",
		Category: common.PluginCategoryMisc,
	}
}

func RegisterPlugin() {
	common.RegisterPlugin(&Plugin{})
}

var _ bot.BotInitHandler = (*Plugin)(nil)

func (p *Plugin) BotInit() {

	eventsystem.AddHandlerAsyncLast(func(evt *eventsystem.EventData) {
		ra := evt.MessageReactionAdd()

		if ra.UserID == common.BotUser.ID {
			return
		}

		menusLock.Lock()
		for _, v := range activePaginatedMessages {
			if v.MessageID == ra.MessageID {
				menusLock.Unlock()
				v.HandleReactionAdd(ra)
				return
			}
		}
		menusLock.Unlock()

	}, eventsystem.EventMessageReactionAdd)
}

func (p *Plugin) StopBot(wg *sync.WaitGroup) {
	menusLock.Lock()
	for _, v := range activePaginatedMessages {
		go v.Stop()
	}
	menusLock.Unlock()

	wg.Done()
}

type PaginatedMessage struct {
	// immutable fields, safe to access without a lock, don't write to these, i dont see why you would need to either...
	MessageID int64
	ChannelID int64
	GuildID   int64

	// mutable fields
	CurrentPage  int
	MaxPage      int
	LastResponse *discordgo.MessageEmbed
	Navigate     func(p *PaginatedMessage, newPage int) (*discordgo.MessageEmbed, error)

	stopped        bool
	stopCh         chan bool
	lastUpdateTime time.Time
	mu             sync.Mutex
}

const (
	EmojiNext = "➡"
	EmojiPrev = "⬅"
)

func CreatePaginatedMessage(guildID, channelID int64, initPage, maxPages int, pagerFunc func(p *PaginatedMessage, page int) (*discordgo.MessageEmbed, error)) (*PaginatedMessage, error) {
	if initPage < 1 {
		initPage = 1
	}

	pm := &PaginatedMessage{
		GuildID:   guildID,
		ChannelID: channelID,

		CurrentPage:    initPage,
		MaxPage:        maxPages,
		lastUpdateTime: time.Now(),
		stopCh:         make(chan bool),
		Navigate:       pagerFunc,
	}

	embed, err := pagerFunc(pm, initPage)
	if err != nil {
		return nil, err
	}

	footer := "Page " + strconv.Itoa(initPage)
	if pm.MaxPage > 0 {
		footer += "/" + strconv.Itoa(pm.MaxPage)
	}
	embed.Footer = &discordgo.MessageEmbedFooter{
		Text: footer,
	}
	embed.Timestamp = time.Now().Format(time.RFC3339)

	msg, err := common.BotSession.ChannelMessageSendEmbed(channelID, embed)
	if err != nil {
		return nil, err
	}
	pm.MessageID = msg.ID

	err = common.BotSession.MessageReactionAdd(channelID, msg.ID, EmojiPrev)
	if err != nil {
		return nil, err
	}
	err = common.BotSession.MessageReactionAdd(channelID, msg.ID, EmojiNext)
	if err != nil {
		return nil, err
	}

	menusLock.Lock()
	activePaginatedMessages = append(activePaginatedMessages, pm)
	menusLock.Unlock()

	go pm.ticker()
	return pm, nil
}

func (p *PaginatedMessage) HandleReactionAdd(ra *discordgo.MessageReactionAdd) {

	pageMod := 0
	if ra.Emoji.Name == EmojiNext {
		pageMod = 1
	} else if ra.Emoji.Name == EmojiPrev {
		pageMod = -1
	}

	// remove the emoji to signal were handling it
	err := common.BotSession.MessageReactionRemove(ra.ChannelID, ra.MessageID, ra.Emoji.APIName(), ra.UserID)
	if err != nil {
		logger.WithError(err).WithField("guild", p.GuildID).Error("failed removing reaction")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if pageMod == 0 || (pageMod == -1 && p.CurrentPage <= 1) ||
		(p.MaxPage > 0 && pageMod == 1 && p.CurrentPage+pageMod > p.MaxPage) {
		return
	}

	newPage := p.CurrentPage + pageMod
	newMsg, err := p.Navigate(p, newPage)
	if err != nil {
		logger.WithError(err).WithField("guild", p.GuildID).Error("failed getting new page")
		return
	}

	if newMsg == nil {
		// No change...
		return
	}
	p.LastResponse = newMsg
	p.lastUpdateTime = time.Now()

	p.CurrentPage = newPage
	footer := "Page " + strconv.Itoa(newPage)
	if p.MaxPage > 0 {
		footer += "/" + strconv.Itoa(p.MaxPage)
	}

	newMsg.Footer = &discordgo.MessageEmbedFooter{
		Text: footer,
	}
	newMsg.Timestamp = time.Now().Format(time.RFC3339)

	_, err = common.BotSession.ChannelMessageEditEmbed(ra.ChannelID, ra.MessageID, newMsg)
	if err != nil {
		logger.WithError(err).WithField("guild", p.GuildID).Error("failed updating message")
	}
}

func (p *PaginatedMessage) ticker() {
	t := time.NewTicker(time.Second * 5)
	defer t.Stop()

OUTER:
	for {
		select {
		case <-t.C:
			p.mu.Lock()
			toRemove := time.Since(p.lastUpdateTime) > time.Minute
			p.mu.Unlock()
			if !toRemove {
				continue OUTER
			}

		case <-p.stopCh:
		}

		// remove the reactions
		common.BotSession.MessageReactionsRemoveAll(p.ChannelID, p.MessageID)

		// remove it
		menusLock.Lock()
		for i, v := range activePaginatedMessages {
			if v == p {
				activePaginatedMessages = append(activePaginatedMessages[:i], activePaginatedMessages[i+1:]...)
			}
		}
		menusLock.Unlock()
		return
	}

}

func (p *PaginatedMessage) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stopped {
		return
	}

	p.stopped = true
	close(p.stopCh)
}
