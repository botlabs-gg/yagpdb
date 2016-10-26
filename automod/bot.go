package automod

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/moderation"
)

func (p *Plugin) InitBot() {
	common.BotSession.AddHandler(bot.CustomMessageCreate(HandleMessageCreate))
	bot.AddEventHandler("update_automod_rules", HandleUpdateAutomodRules, nil)
}

// Invalidate the cache when the rules have changed
func HandleUpdateAutomodRules(event *bot.Event) {
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

func HandleMessageCreate(s *discordgo.Session, evt *discordgo.MessageCreate, client *redis.Client) {

	channel := common.LogGetChannel(evt.ChannelID)
	if channel == nil {
		return
	}

	if channel.IsPrivate {
		return
	}

	guild := common.LogGetGuild(channel.GuildID)
	if guild == nil {
		return
	}

	config, err := CachedGetConfig(client, guild.ID)
	if err != nil {
		logrus.WithError(err).Error("Failed retrieving config")
		return
	}

	if !config.Enabled {
		logrus.Info("Automoderator is disabled")
		return
	}

	member, err := s.State.Member(guild.ID, evt.Author.ID)
	if err != nil {
		logrus.WithError(err).Error("Failed finding guild member")
		return
	}

	del := false
	var punishMsg string
	highestPunish := PunishNone

	rules := []Rule{config.Spam, config.Invite, config.Mention, config.Links, config.Words, config.Sites}

	// We gonna need to have this locked while we check
	s.State.RLock()
	for _, r := range rules {
		if r.ShouldIgnore(evt, member) {
			continue
		}

		d, punishment, msg, err := r.Check(evt, channel, client)
		if d {
			del = true
		}
		if err != nil {
			logrus.WithError(err).Error("Failed checking aumod rule:", err)
			continue
		}

		// If the rule did not trigger a deletion there wasnt any violation
		if !d {
			continue
		}

		punishMsg += msg + "\n"

		if punishment > highestPunish {
			highestPunish = punishment
		}
	}
	s.State.RUnlock()

	if del {
		s.ChannelMessageDelete(evt.ChannelID, evt.ID)
	} else {
		return // Nothing to do
	}

	switch highestPunish {
	case PunishNone:
		err = bot.SendDM(s, member.User.ID, fmt.Sprintf("**Automoderator for %s, Rule violations:**\n%s\nRepeating this offence may cause you a kick, mute or ban.", guild.Name, punishMsg))
	case PunishMute:
		// TODO
	case PunishKick:
		err = moderation.KickUser(client, channel.GuildID, channel.ID, "Automod", punishMsg, member.User)
	case PunishBan:
		err = moderation.BanUser(client, channel.GuildID, channel.ID, "Automod", punishMsg, member.User)
	}

	if err != nil {
		logrus.WithError(err).Error("Error carrying out punishment")
	}
}
