package paginatedmessages

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

const (
	EmojiNext = "▶"
	EmojiPrev = "◀"
)

var (
	activePaginatedMessagesMap = make(map[int64]*PaginatedMessage)
	paginationNext             = "pagination_next"
	paginationPrev             = "pagination_prev"
)

func handleInteractionCreate(evt *eventsystem.EventData) {
	ic := evt.InteractionCreate()
	if ic.Type != discordgo.InteractionMessageComponent {
		return
	}

	if ic.GuildID == 0 {
		//DM interactions are handled via pubsub
		return
	}

	switch ic.MessageComponentData().CustomID {
	case paginationNext:
		handlePageChange(ic, 1)
	case paginationPrev:
		handlePageChange(ic, -1)
	}
}

func handlePageChange(ic *discordgo.InteractionCreate, pageMod int) {
	if ic.Member != nil && ic.Member.User.ID == common.BotUser.ID {
		return
	}

	if ic.User != nil && ic.User.ID == common.BotUser.ID {
		return
	}

	menusLock.Lock()
	if paginatedMessage, ok := activePaginatedMessagesMap[ic.Message.ID]; ok {
		menusLock.Unlock()
		paginatedMessage.HandlePageButtonClick(ic, pageMod)
		return
	}
	menusLock.Unlock()
}

func (p *Plugin) StopBot(wg *sync.WaitGroup) {
	menusLock.Lock()
	for _, v := range activePaginatedMessagesMap {
		go v.Stop()
	}

	for _, v := range activePaginatedMessages {
		go v.Stop()
	}
	menusLock.Unlock()

	wg.Done()
}

func createNavigationButtons(prevDisabled bool, nextDisabled bool) []discordgo.MessageComponent {
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    EmojiPrev,
					Style:    discordgo.PrimaryButton,
					Disabled: prevDisabled,
					CustomID: paginationPrev,
				},
				discordgo.Button{
					Label:    EmojiNext,
					Style:    discordgo.PrimaryButton,
					Disabled: nextDisabled,
					CustomID: paginationNext,
				},
			},
		},
	}
}

func CreatePaginatedMessage(guildID, channelID int64, initPage, maxPages int, pagerFunc PagerFunc) (*PaginatedMessage, error) {
	if initPage < 1 {
		initPage = 1
	}

	pm := &PaginatedMessage{
		GuildID:        guildID,
		ChannelID:      channelID,
		CurrentPage:    initPage,
		MaxPage:        maxPages,
		lastUpdateTime: time.Now(),
		stopCh:         make(chan bool),
		Navigate:       pagerFunc,
	}

	m, err := pagerFunc(pm, initPage)
	if err != nil {
		return nil, err
	}
	
	footer := "Page " + strconv.Itoa(initPage)
	nextButtonDisabled := false
	if pm.MaxPage > 0 {
		footer += "/" + strconv.Itoa(pm.MaxPage)
		nextButtonDisabled = initPage >= pm.MaxPage
	}

	m.Components = createNavigationButtons(true, nextButtonDisabled)
	if len(m.Embeds) > 0 {
		m.Embeds[len(m.Embeds)-1].Footer = &discordgo.MessageEmbedFooter{
			Text: footer,
		}
		m.Embeds[len(m.Embeds)-1].Timestamp = time.Now().Format(time.RFC3339)
	} else {
		m.Content = fmt.Sprintf("%s\n`%s`", m.Content, footer)
	}

	msg, err := common.BotSession.ChannelMessageSendComplex(channelID, m)
	if err != nil {
		return nil, err
	}

	pm.MessageID = msg.ID
	pm.LastResponse = m

	menusLock.Lock()
	activePaginatedMessagesMap[pm.MessageID] = pm
	menusLock.Unlock()

	go pm.paginationTicker()
	return pm, nil
}

func (p *PaginatedMessage) HandlePageButtonClick(ic *discordgo.InteractionCreate, pageMod int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Pong the interaction
	err := common.BotSession.CreateInteractionResponse(ic.ID, ic.Token, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
	if err != nil {
		return
	}

	if pageMod == 0 || (pageMod == -1 && p.CurrentPage <= 1) ||
		(p.MaxPage > 0 && pageMod == 1 && p.CurrentPage+pageMod > p.MaxPage) {
		return
	}

	newPage := p.CurrentPage + pageMod
	newMsg, err := p.Navigate(p, newPage)
	if err != nil {
		if err == ErrNoResults {
			if pageMod == 1 {
				newPage--
			}
			if newPage < 1 {
				newPage = 1
			}

			p.MaxPage = newPage
			newMsg = p.LastResponse
			logger.Println("Max page set to ", newPage)
		} else {
			logger.WithError(err).WithField("guild", p.GuildID).Error("failed getting new page")
			return
		}
	}

	if newMsg == nil {
		// No change...
		return
	}
	p.LastResponse = newMsg
	p.lastUpdateTime = time.Now()

	p.CurrentPage = newPage
	footer := "Page " + strconv.Itoa(newPage)
	nextButtonDisabled := false
	if p.MaxPage > 0 {
		footer += "/" + strconv.Itoa(p.MaxPage)
		nextButtonDisabled = newPage >= p.MaxPage
	}

	if len(newMsg.Embeds) > 0 {
		newMsg.Embeds[len(newMsg.Embeds)-1].Footer = &discordgo.MessageEmbedFooter{
			Text: footer,
		}
		newMsg.Embeds[len(newMsg.Embeds)-1].Timestamp = time.Now().Format(time.RFC3339)
	} else {
		newMsg.Content = fmt.Sprintf("%s\n`%s`", newMsg.Content, footer)
	}

	_, err = common.BotSession.ChannelMessageEditComplex(&discordgo.MessageEdit{
		Content:         &newMsg.Content,
		Embeds:          newMsg.Embeds,
		Components:	     createNavigationButtons(newPage <= 1, nextButtonDisabled),
		AllowedMentions: newMsg.AllowedMentions,
		Channel:         ic.ChannelID,
		ID:              ic.Message.ID,
	})

	if err != nil {
		switch code, _ := common.DiscordError(err); code {
		case discordgo.ErrCodeUnknownChannel, discordgo.ErrCodeUnknownMessage, discordgo.ErrCodeMissingAccess, discordgo.ErrCodeMissingPermissions:
			p.Broken = true
		default:
			logger.WithError(err).WithField("guild", p.GuildID).Error("failed updating paginated message")
		}
	}
}

func (p *PaginatedMessage) paginationTicker() {
	t := time.NewTicker(time.Second * 5)
	defer t.Stop()

OUTER:
	for {
		select {
		case <-t.C:
			p.mu.Lock()
			toRemove := time.Since(p.lastUpdateTime) > time.Minute*10 || p.Broken
			p.mu.Unlock()
			if !toRemove {
				continue OUTER
			}

		case <-p.stopCh:
		}

		// remove the navigation buttons
		lastMessage := p.LastResponse
		footer := "Page " + strconv.Itoa(p.CurrentPage)
		if p.MaxPage > 0 {
			footer += "/" + strconv.Itoa(p.MaxPage)
		}
		if len(lastMessage.Embeds) > 0 {
			lastMessage.Embeds[len(lastMessage.Embeds)-1].Footer = &discordgo.MessageEmbedFooter{
				Text: footer,
			}
			lastMessage.Embeds[len(lastMessage.Embeds)-1].Timestamp = time.Now().Format(time.RFC3339)
		}

		_, err := common.BotSession.ChannelMessageEditComplex(&discordgo.MessageEdit{
			Content:         &lastMessage.Content,
			Embeds:          lastMessage.Embeds,
			Components:	     []discordgo.MessageComponent{},
			AllowedMentions: lastMessage.AllowedMentions,
			Channel:         p.ChannelID,
			ID:              p.MessageID,
		})

		if err != nil {
			switch code, _ := common.DiscordError(err); code {
			case discordgo.ErrCodeUnknownChannel, discordgo.ErrCodeUnknownMessage, discordgo.ErrCodeMissingAccess, discordgo.ErrCodeMissingPermissions:
				p.Broken = true
			default:
				logger.WithError(err).WithField("guild", p.GuildID).Error("failed updating paginated message")
			}
		}

		// remove the object from map
		menusLock.Lock()
		delete(activePaginatedMessagesMap, p.MessageID)
		menusLock.Unlock()
		return
	}
}
