package discordlogger

import (
	"context"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"os"
	"time"
)

var (
	// Send errors to this discord channel
	ErrorChannel string
	// Send bot leaves joins to this disocrd channel
	BotLeavesJoins string
)

func init() {
	ErrorChannel = os.Getenv("YAGPDB_ERRORCHANNEL")
	BotLeavesJoins = os.Getenv("YAGPDB_BOTLEAVESJOINS")
}

func Register() {
	if ErrorChannel != "" {
		logrus.Info("Adding logrus hook")
		logrus.AddHook(&Plugin{})
	}

	if BotLeavesJoins != "" {
		logrus.Info("Listening for bot leaves and join")
		bot.RegisterPlugin(&Plugin{})
	}
}

type Plugin struct{}

func (p *Plugin) InitBot() {
	bot.AddHandler(EventHandler, bot.EventNewGuild)
	bot.AddHandler(EventHandler, bot.EventGuildDelete)
}

func EventHandler(ctx context.Context, evt interface{}) {
	bot.State.RLock()
	count := len(bot.State.Guilds)
	bot.State.RUnlock()

	msg := ""
	switch t := evt.(type) {
	case *discordgo.GuildDelete:
		msg = fmt.Sprintf(":x: Left guild **%s** :(", t.Guild.Name)
	case *discordgo.GuildCreate:
		msg = fmt.Sprintf(":white_check_mark: Joined guild **%s** :D", t.Guild.Name)
	}

	msg += fmt.Sprintf(" (now connected to %d servers)", count)
	common.BotSession.ChannelMessageSend(BotLeavesJoins, msg)
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
