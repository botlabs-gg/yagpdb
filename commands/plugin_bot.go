package commands

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix.v2/redis"
)

var (
	CommandSystem *commandsystem.System
)

func (p *Plugin) InitBot() {

	CommandSystem = commandsystem.NewSystem(nil, "")
	CommandSystem.SendError = false
	CommandSystem.CensorError = CensorError
	CommandSystem.State = bot.State

	CommandSystem.DefaultDMHandler = &commandsystem.Command{
		Run: func(data *commandsystem.ExecData) (interface{}, error) {
			return "Unknwon command, only a subset of commands are available in dms.", nil
		},
	}

	CommandSystem.Prefix = p
	CommandSystem.RegisterCommands(cmdHelp)

	eventsystem.AddHandler(bot.RedisWrapper(HandleGuildCreate), eventsystem.EventGuildCreate)
	eventsystem.AddHandler(handleMsgCreate, eventsystem.EventMessageCreate)
}

func handleMsgCreate(evt *eventsystem.EventData) {
	CommandSystem.HandleMessageCreate(common.BotSession, evt.MessageCreate)
}

func (p *Plugin) GetPrefix(s *discordgo.Session, m *discordgo.MessageCreate) string {
	client, err := common.RedisPool.Get()
	if err != nil {
		log.WithError(err).Error("Failed retrieving redis connection from pool")
		return ""
	}
	defer common.RedisPool.Put(client)

	channel := bot.State.Channel(true, m.ChannelID)
	if channel == nil {
		log.Error("Failed retrieving channels from state")
		return ""
	}

	prefix, err := GetCommandPrefix(client, channel.Guild.ID())
	if err != nil {
		log.WithError(err).Error("Failed retrieving commands prefix")
	}

	return prefix
}

func GenerateHelp(target string) string {
	if target != "" {
		return CommandSystem.GenerateHelp(target, 100)
	}

	categories := make(map[CommandCategory][]*CustomCommand)

	for _, v := range CommandSystem.Commands {
		cast := v.(*CustomCommand)
		categories[cast.Category] = append(categories[cast.Category], cast)
	}

	out := "```ini\n"

	out += `[Legend]
#
#Command   = {alias1, alias2...} <required arg> (optional arg) : Description
#
#Example:
Help        = {hlp}   (command)       : blablabla
# |             |          |                |
#Comand name, Aliases,  optional arg,    Description

`

	// Do it manually to preserve order
	out += "[General] # General YAGPDB commands"
	out += generateComandsHelp(categories[CategoryGeneral]) + "\n"

	out += "\n[Tools]"
	out += generateComandsHelp(categories[CategoryTool]) + "\n"

	out += "\n[Moderation] # These are off by default"
	out += generateComandsHelp(categories[CategoryModeration]) + "\n"

	out += "\n[Misc/Fun] # Fun commands for family and friends!"
	out += generateComandsHelp(categories[CategoryFun]) + "\n"

	out += "\n[Debug/Maintenance] # Commands for maintenance and debug mainly."
	out += generateComandsHelp(categories[CategoryDebug]) + "\n"

	unknown, ok := categories[CommandCategory("")]
	if ok && len(unknown) > 1 {
		out += "\n[Unknown] # ??"
		out += generateComandsHelp(unknown) + "\n"
	}

	out += "```"
	return out
}

func generateComandsHelp(cmds []*CustomCommand) string {
	out := ""
	for _, v := range cmds {
		if !v.HideFromHelp {
			out += "\n" + v.GenerateHelp("", 100, 0)
		}
	}
	return out
}

var cmdHelp = &CustomCommand{
	Cooldown: 10,
	Category: CategoryGeneral,
	Command: &commandsystem.Command{
		Name:        "Help",
		Description: "Shows help abut all or one specific command",
		RunInDm:     true,
		Arguments: []*commandsystem.ArgDef{
			&commandsystem.ArgDef{Name: "command", Type: commandsystem.ArgumentString},
		},

		Run: cmdFuncHelp,
	},
}

func cmdFuncHelp(data *commandsystem.ExecData) (interface{}, error) {
	target := ""
	if data.Args[0] != nil {
		target = data.Args[0].Str()
	}

	// Fetch the prefix if ther command was not run in a dm
	footer := ""
	if data.Source != commandsystem.SourceDM && target == "" {
		prefix, err := GetCommandPrefix(data.Context().Value(CtxKeyRedisClient).(*redis.Client), data.Guild.ID())
		if err != nil {
			return "Error communicating with redis", err
		}

		footer = "**No command prefix set, you can still use commands through mentioning the bot\n**"
		if prefix != "" {
			footer = fmt.Sprintf("**Command prefix: %q**\n", prefix)
		}
	}

	if target == "" {
		footer += "**Support server:** https://discord.gg/0vYlUK2XBKldPSMY\n**Control Panel:** https://yagpdb.xyz/manage\n"
	}

	channelId := data.Message.ChannelID

	help := GenerateHelp(target)
	if target == "" && data.Source != commandsystem.SourceDM {
		privateChannel, err := bot.GetCreatePrivateChannel(data.Message.Author.ID)
		if err != nil {
			return "", err
		}
		channelId = privateChannel.ID
	}

	if help == "" {
		help = "Command not found"
	}

	dutil.SplitSendMessagePS(common.BotSession, channelId, help+"\n"+footer, "```ini\n", "```", false, false)
	if data.Source != commandsystem.SourceDM && target == "" {
		return "You've Got Mail!", nil
	} else {
		return "", nil
	}
}

func HandleGuildCreate(evt *eventsystem.EventData) {
	client := bot.ContextRedis(evt.Context())
	g := evt.GuildCreate
	prefixExists, err := common.RedisBool(client.Cmd("EXISTS", "command_prefix:"+g.ID))
	if err != nil {
		log.WithError(err).Error("Failed checking if prefix exists")
		return
	}

	if !prefixExists {
		client.Cmd("SET", "command_prefix:"+g.ID, "-")
		log.WithField("guild", g.ID).WithField("g_name", g.Name).Info("Set command prefix to default (-)")
	}
}
