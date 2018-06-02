package discordlogger

import (
	"fmt"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
	"github.com/sirupsen/logrus"
	"os"
	"strconv"
	"time"
)

var (
	// Send errors to this discord channel
	ErrorChannel int64
	// Send bot leaves joins to this disocrd channel
	BotLeavesJoins int64
)

func init() {
	ErrorChannel, _ = strconv.ParseInt(os.Getenv("YAGPDB_ERRORCHANNEL"), 10, 64)
	BotLeavesJoins, _ = strconv.ParseInt(os.Getenv("YAGPDB_BOTLEAVESJOINS"), 10, 64)
}

func Register() {
	// if ErrorChannel != "" {
	// 	logrus.Info("Adding logrus hook")
	// 	// logrus.AddHook(&Plugin{})
	// 	eventsystem.AddHandler(OnReady, eventsystem.EventReady)
	// }

	if BotLeavesJoins != 0 {
		logrus.Info("Listening for bot leaves and join")
		common.RegisterPlugin(&Plugin{})
	}
}

type Plugin struct{}

func (p *Plugin) InitBot() {
	eventsystem.AddHandler(EventHandler, eventsystem.EventNewGuild, eventsystem.EventGuildDelete)
}

func EventHandler(evt *eventsystem.EventData) {
	bot.State.RLock()
	count := len(bot.State.Guilds)
	bot.State.RUnlock()

	msg := ""
	switch evt.Type {
	case eventsystem.EventGuildDelete:
		if evt.GuildDelete().Unavailable {
			// Just a guild outage
			return
		}
		msg = fmt.Sprintf(":x: Left guild **%s** :(", evt.GuildDelete().Guild.Name)
	case eventsystem.EventNewGuild:
		msg = fmt.Sprintf(":white_check_mark: Joined guild **%s** :D", evt.GuildCreate().Guild.Name)
	}

	msg += fmt.Sprintf(" (now connected to %d servers)", count)
	common.BotSession.ChannelMessageSend(BotLeavesJoins, common.EscapeSpecialMentions(msg))
}

func (p *Plugin) Levels() []logrus.Level {
	return []logrus.Level{logrus.ErrorLevel, logrus.PanicLevel, logrus.FatalLevel}
}
func (p *Plugin) Name() string {
	return "DiscordLogger"
}

var levelColors = map[logrus.Level]int{
	logrus.ErrorLevel: 0xf44242,
	logrus.FatalLevel: 0xd442f4,
	logrus.PanicLevel: 0x42f4e5,
}

func (p *Plugin) Fire(entry *logrus.Entry) error {
	embed := &discordgo.MessageEmbed{
		Title:       entry.Level.String(),
		Description: entry.Message,
		Footer: &discordgo.MessageEmbedFooter{
			Text: entry.Time.UTC().Format(time.RFC3339),
		},
		Color: levelColors[entry.Level],
	}

	for k, v := range entry.Data {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   k,
			Value:  fmt.Sprint(v),
			Inline: true,
		})
	}

	_, err := common.BotSession.ChannelMessageSendEmbed(ErrorChannel, embed)
	if err != nil {
		logrus.WithError(err).Warn("Failed sending error logging message to discord channel")
	}
	return nil
}

type Stringer interface {
	String() string
}

func OnReady(evt *eventsystem.EventData) {
	// common.BotSession.ChannelMessageSend(ErrorChannel, "<@"+common.Conf.Owner+"> Ready!")
}
