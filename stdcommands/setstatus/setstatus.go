package setstatus

import (
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/util"
)

var Command = &commands.YAGCommand{
	Cooldown:             2,
	CmdCategory:          commands.CategoryDebug,
	HideFromCommandsPage: true,
	Name:                 "setstatus",
	Description:          "Sets the bot's status and optional streaming url. Bot Admin Only",
	HideFromHelp:         true,
	Arguments: []*dcmd.ArgDef{
		{Name: "status", Type: dcmd.String, Default: ""},
	},
	ArgSwitches: []*dcmd.ArgDef{
		{Name: "url", Type: dcmd.String, Default: ""},
		{Name: "type", Type: dcmd.String, Help: "Game type. Allowed values are 'playing', 'streaming', 'listening', 'watching', 'custom', 'competing'.", Default: "custom"},
		{Name: "status", Type: dcmd.String, Help: "Online status. Allowed values are 'online', 'idle', 'dnd', 'offline'.", Default: "online"},
	},
	RunFunc: util.RequireBotAdmin(func(data *dcmd.Data) (interface{}, error) {
		statusText := data.Args[0].Str()
		streamingUrl := data.Switch("url").Str()
		gameType := data.Switch("type").Str()
		statusType := data.Switch("status").Str()
		switch gameType {
		case "playing":
		case "streaming":
		case "listening":
		case "watching":
		case "custom":
		case "competing":
		default:
			gameType = "custom"
		}
		switch statusType {
		case "online":
		case "idle":
		case "dnd":
		case "offline":
		default:
			statusType = "online"
		}
		bot.SetStatus(gameType, statusText, statusType, streamingUrl)
		return "Doneso", nil
	}),
}
