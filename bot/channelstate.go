package bot

import (
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/karlseguin/ccache"
)

var (
	addChannelAttempts   = ccache.New(ccache.Configure())
	addChannelAttemptsMu sync.Mutex
)

const addChannelCooldown = 10 * time.Minute

// AddChannelToState fetches a channel or thread that a message or interaction came from but is not in state, and adds it to state.
// Interactions can also come from channels the bot cannot view, so every channel is only tried once per cooldown.
func AddChannelToState(guildID, channelID int64) {
	if !claimAddChannelAttempt(channelID) {
		return
	}

	l := logger.WithField("guild", guildID).WithField("channel", channelID)

	session := ShardManager.SessionForGuild(guildID)
	if session == nil {
		return
	}

	c, err := common.BotSession.Channel(channelID)
	if err != nil {
		l.WithError(err).Debug("Failed fetching channel to add to state")
		return
	}

	if c.GuildID != guildID {
		return
	}

	// the state is updated right away so the caller can use the channel, the event then lets the rest of the bot know about it
	var evt any = &discordgo.ChannelUpdate{Channel: c}
	if c.Type.IsThread() {
		evt = &discordgo.ThreadUpdate{Channel: *c}
	}

	stateTracker.HandleEvent(session, evt)
	eventsystem.HandleEvent(session, evt)

	l.Warn("Added channel to state")
}

// InteractionChannel returns the channel the interaction was used in, along with the guild containing it.
// Channels the bot can view are added to state when missing, others are built from the partial channel in the interaction,
// so plugins can still respond through the interaction.
func InteractionChannel(gs *dstate.GuildSet, interaction *discordgo.Interaction) (*dstate.GuildSet, *dstate.ChannelState) {
	if gs.GetChannelOrThread(interaction.ChannelID) == nil && dstate.BotCanViewInteractionChannel(interaction) {
		AddChannelToState(gs.ID, interaction.ChannelID)
		if updated := State.GetGuild(gs.ID); updated != nil {
			gs = updated
		}
	}

	return gs.WithInteractionChannel(interaction)
}

func claimAddChannelAttempt(channelID int64) bool {
	key := discordgo.StrID(channelID)

	addChannelAttemptsMu.Lock()
	defer addChannelAttemptsMu.Unlock()

	if item := addChannelAttempts.Get(key); item != nil && !item.Expired() {
		return false
	}

	addChannelAttempts.Set(key, true, addChannelCooldown)
	return true
}
