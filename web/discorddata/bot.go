package discorddata

import (
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/pubsub"
)

type EvictData struct {
	Keys []string `json:"keys"`
}

func init() {
	pubsub.AddHandler("web_discorddata_evict", func(event *pubsub.Event) {
		data := event.Data.(*EvictData)

		for _, v := range data.Keys {
			evictCached(v)
		}
	}, EvictData{})
}

var _ bot.BotInitHandler = (*Plugin)(nil)

func (p *Plugin) BotInit() {
	eventsystem.AddHandlerAsyncLast(p, p.handleInvalidateGuildCache, eventsystem.EventGuildRoleCreate,
		eventsystem.EventGuildRoleUpdate,
		eventsystem.EventGuildRoleDelete,
		eventsystem.EventChannelCreate,
		eventsystem.EventChannelUpdate,
		eventsystem.EventChannelDelete)

	eventsystem.AddHandlerAsyncLast(p, p.handleInvalidateMemberCache, eventsystem.EventGuildMemberUpdate)
}

func (p *Plugin) handleInvalidateGuildCache(evt *eventsystem.EventData) (retry bool, err error) {
	if evt.GS == nil {
		// Opening DM channels can cause this
		return
	}

	PubEvictGuild(evt.GS.ID)
	return false, nil
}

func (p *Plugin) handleInvalidateMemberCache(evt *eventsystem.EventData) (retry bool, err error) {
	if evt.GS == nil {
		return false, nil
	}

	userID := evt.GuildMemberUpdate().User.ID
	if userID == common.BotUser.ID {
		// the bot's roles decide which channels are in the guild
		pubEvictCache(keyGuildMember(evt.GS.ID, userID), keyFullGuild(evt.GS.ID))
		return false, nil
	}

	PubEvictMember(evt.GS.ID, userID)
	return false, nil
}

func pubEvictCache(keys ...string) {
	pubsub.Publish("web_discorddata_evict", -1, EvictData{
		Keys: keys,
	})
}

func PubEvictGuild(guildID int64) {
	pubEvictCache(keyFullGuild(guildID))
}

func PubEvictMember(guildID int64, userID int64) {
	pubEvictCache(keyGuildMember(guildID, userID))
}
