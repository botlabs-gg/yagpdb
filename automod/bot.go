package automod

import (
	"github.com/google/safebrowsing"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/jonas747/yagpdb/moderation"
	"github.com/karlseguin/ccache"
	"github.com/mediocregopher/radix.v2/redis"
	"github.com/sirupsen/logrus"
	"os"
	"time"
)

func (p *Plugin) InitBot() {
	eventsystem.AddHandler(bot.RedisWrapper(HandleMessageCreate), eventsystem.EventMessageCreate)
	eventsystem.AddHandler(bot.RedisWrapper(HandleMessageUpdate), eventsystem.EventMessageUpdate)
}

var _ bot.BotStarterHandler = (*Plugin)(nil)
var (
	// cache configs because they are used often
	confCache   *ccache.Cache
	safeBrowser *safebrowsing.SafeBrowser
)

func (p *Plugin) StartBot() {
	pubsub.AddHandler("update_automod_rules", HandleUpdateAutomodRules, nil)
	confCache = ccache.New(ccache.Configure().MaxSize(1000))

	safeBrosingAPIKey := os.Getenv("YAGPDB_GOOGLE_SAFEBROWSING_API_KEY")
	if safeBrosingAPIKey != "" {
		conf := safebrowsing.Config{
			APIKey: safeBrosingAPIKey,
			DBPath: "safebrowsing_db",
			Logger: logrus.StandardLogger().Writer(),
		}
		sb, err := safebrowsing.NewSafeBrowser(conf)
		if err != nil {
			logrus.WithError(err).Error("Failed initializing google safebrowsing client, integration will be disabled")
		} else {
			safeBrowser = sb
		}
	}
}

// Invalidate the cache when the rules have changed
func HandleUpdateAutomodRules(event *pubsub.Event) {
	confCache.Delete(KeyConfig(event.TargetGuildInt))
}

// CachedGetConfig either retrieves from local application cache or redis
func CachedGetConfig(client *redis.Client, gID int64) (*Config, error) {
	confItem, err := confCache.Fetch(KeyConfig(gID), time.Minute*5, func() (interface{}, error) {
		c, err := GetConfig(client, gID)
		if err != nil {
			return nil, err
		}

		// Compile sites and words
		c.Sites.GetCompiled()
		c.Words.GetCompiled()

		return c, nil
	})

	if err != nil {
		return nil, err
	}

	return confItem.Value().(*Config), nil
}

func HandleMessageCreate(evt *eventsystem.EventData) {
	CheckMessage(evt.MessageCreate().Message, bot.ContextRedis(evt.Context()))
}

func HandleMessageUpdate(evt *eventsystem.EventData) {
	CheckMessage(evt.MessageUpdate().Message, bot.ContextRedis(evt.Context()))
}

func CheckMessage(m *discordgo.Message, client *redis.Client) {

	if m.Author == nil || m.Author.ID == bot.State.User(true).ID {
		return // Pls no panicerinos or banerinos self
	}

	if m.Author.Bot {
		return
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

	member, err := bot.GetMember(cs.Guild.ID(), m.Author.ID)
	if err != nil {
		logrus.WithError(err).WithField("guild", cs.Guild.ID()).Warn("Member not found in state, automod ignoring")
		return
	}

	locked := true
	cs.Owner.RLock()
	defer func() {
		if locked {
			cs.Owner.RUnlock()
		}
	}()

	del := false // Set if a rule triggered a message delete
	punishMsg := ""
	highestPunish := PunishNone
	muteDuration := 0

	rules := []Rule{config.Spam, config.Invite, config.Mention, config.Links, config.Words, config.Sites}

	// We gonna need to have this locked while we check
	for _, r := range rules {
		if r.ShouldIgnore(m, member) {
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

		// If the rule did not trigger a deletion there wasn't any violation
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

	cs.Owner.RUnlock()
	locked = false

	switch highestPunish {
	case PunishNone:
		err = moderation.WarnUser(nil, cs.Guild.ID(), cs.ID, bot.State.User(true).User, member.DGoUser(), "Automoderator: "+punishMsg)
	case PunishMute:
		err = moderation.MuteUnmuteUser(nil, client, true, cs.Guild.ID(), cs.ID, bot.State.User(true).User, "Automoderator: "+punishMsg, member, muteDuration)
	case PunishKick:
		err = moderation.KickUser(nil, cs.Guild.ID(), cs.ID, bot.State.User(true).User, "Automoderator: "+punishMsg, member.DGoUser())
	case PunishBan:
		err = moderation.BanUser(client, nil, cs.Guild.ID(), cs.ID, bot.State.User(true).User, "Automoderator: "+punishMsg, member.DGoUser(), true)
	}

	// Execute the punishment before removing the message to make sure it's included in logs
	common.BotSession.ChannelMessageDelete(m.ChannelID, m.ID)

	if err != nil && err != moderation.ErrNoMuteRole && !common.IsDiscordErr(err, discordgo.ErrCodeMissingPermissions, discordgo.ErrCodeMissingAccess) {
		logrus.WithError(err).Error("Error carrying out punishment")
	}

}
