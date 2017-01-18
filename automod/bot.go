package automod

import (
	"context"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/jonas747/yagpdb/moderation"
)

func (p *Plugin) InitBot() {
	bot.AddHandler(bot.RedisWrapper(HandleMessageCreate), bot.EventMessageCreate)
	bot.AddHandler(bot.RedisWrapper(HandleMessageUpdate), bot.EventMessageUpdate)
}

func (p *Plugin) StartBot() {
	pubsub.AddHandler("update_automod_rules", HandleUpdateAutomodRules, nil)
}

// Invalidate the cache when the rules have changed
func HandleUpdateAutomodRules(event *pubsub.Event) {
	bot.Cache.Delete(KeyAllRules(event.TargetGuild))
}

func CachedGetConfig(client *redis.Client, gID string) (*Config, error) {
	if config, ok := bot.Cache.Get(KeyConfig(gID)); ok {
		return config.(*Config), nil
	}
	conf, err := GetConfig(client, gID)
	if err == nil {
		// Compile the sites and word list
		conf.Sites.GetCompiled()
		conf.Words.GetCompiled()
	}
	return conf, err
}

func HandleMessageCreate(ctx context.Context, evt interface{}) {
	CheckMessage(evt.(*discordgo.MessageCreate).Message, bot.ContextRedis(ctx))
}

func HandleMessageUpdate(ctx context.Context, evt interface{}) {
	CheckMessage(evt.(*discordgo.MessageUpdate).Message, bot.ContextRedis(ctx))
}

func CheckMessage(m *discordgo.Message, client *redis.Client) {

	if m.Author == nil || m.Author.ID == bot.State.User(true).ID {
		return // Pls no panicerinos or banerinos self
	}

	cs := bot.State.Channel(true, m.ChannelID)
	if cs == nil {
		logrus.WithField("channel", m.ChannelID).Error("Channel not found in state")
		return
	}

	if cs.IsPrivate() {
		return
	}

	config, err := CachedGetConfig(client, cs.Guild.ID())
	if err != nil {
		logrus.WithError(err).Error("Failed retrieving config")
		return
	}

	if !config.Enabled {
		return
	}

	locked := true
	cs.Owner.RLock()
	defer func() {
		if locked {
			cs.Owner.RUnlock()
		}
	}()

	ms := cs.Guild.Member(false, m.Author.ID)
	if ms == nil || ms.Member == nil {
		logrus.WithField("guild", cs.Guild.ID()).Error("Member not found in state, automod ignoring")
		return
	}

	del := false // Set if a rule triggered a message delete
	punishMsg := ""
	highestPunish := PunishNone
	muteDuration := 0

	rules := []Rule{config.Spam, config.Invite, config.Mention, config.Links, config.Words, config.Sites}

	// We gonna need to have this locked while we check
	for _, r := range rules {
		if r.ShouldIgnore(m, ms.Member) {
			continue
		}

		d, punishment, msg, err := r.Check(m, cs, client)
		if d {
			del = true
		}
		if err != nil {
			logrus.WithError(err).WithField("guild", cs.Guild.ID()).Error("Failed checking aumod rule:", err)
			continue
		}

		// If the rule did not trigger a deletion there wasnt any violation
		if !d {
			continue
		}

		punishMsg += msg + "\n"

		if punishment > highestPunish {
			highestPunish = punishment
			muteDuration = r.GetMuteDuration()
		}
	}

	if !del {
		return
	}

	if punishMsg != "" {
		// Strip last newline
		punishMsg = punishMsg[:len(punishMsg)-1]
	}
	gName := cs.Guild.Guild.Name
	member := cs.Guild.MemberCopy(false, ms.ID(), true)
	cs.Owner.RUnlock()
	locked = false

	switch highestPunish {
	case PunishNone:
		err = bot.SendDM(ms.Member.User.ID, fmt.Sprintf("**Automoderator for %s, Rule violations:**\n%s\nRepeating this offence may cause you a kick, mute or ban.", gName, punishMsg))
	case PunishMute:
		err = moderation.MuteUnmuteUser(nil, client, true, cs.Guild.ID(), cs.ID(), bot.State.User(true).User, "Automoderator: "+punishMsg, member, muteDuration)
	case PunishKick:
		err = moderation.KickUser(nil, cs.Guild.ID(), cs.ID(), bot.State.User(true).User, "Automoderator: "+punishMsg, member.User)
	case PunishBan:
		err = moderation.BanUser(client, nil, cs.Guild.ID(), cs.ID(), bot.State.User(true).User, "Automoderator: "+punishMsg, member.User)
	}

	// Execute the punishment before removing the message to make sure it's included in logs
	common.BotSession.ChannelMessageDelete(m.ChannelID, m.ID)

	if err != nil {
		logrus.WithError(err).Error("Error carrying out punishment")
	}

}
