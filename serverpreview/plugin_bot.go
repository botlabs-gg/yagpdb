package serverpreview

import (
	"context"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/serverpreview/models"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

var _ bot.BotInitHandler = (*Plugin)(nil)

func (p *Plugin) BotInit() {
	eventsystem.AddHandlerAsyncLastLegacy(p, handleInviteDelete, eventsystem.EventInviteDelete)
	eventsystem.AddHandlerAsyncLastLegacy(p, handleChannelDelete, eventsystem.EventChannelDelete)
}

func handleInviteDelete(evt *eventsystem.EventData) {
	invite := evt.InviteDelete()
	disablePreview(invite.GuildID, qm.Where("guild_id = ? AND invite_code = ?", invite.GuildID, invite.Code))
}

func handleChannelDelete(evt *eventsystem.EventData) {
	channel := evt.ChannelDelete()
	disablePreview(channel.GuildID, qm.Where("guild_id = ? AND invite_channel_id = ?", channel.GuildID, channel.ID))
}

// disablePreview turns off a preview whose invite is gone, so its link stops leading to a dead invite.
func disablePreview(guildID int64, where qm.QueryMod) {
	updated, err := models.ServerPreviews(where, qm.Where("enabled")).UpdateAllG(context.Background(), models.M{
		"enabled":           false,
		"invite_channel_id": 0,
		"invite_code":       "",
		"updated_at":        time.Now(),
	})
	if err != nil {
		logger.WithError(err).WithField("guild", guildID).Error("failed disabling server preview after its invite was removed")
		return
	}
	if updated > 0 {
		evictSettings(guildID)
	}
}
