package setstatus

import (
	"fmt"

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
	Description:          "Sets the bot's presence type, status text, online status, and optional streaming URL. Bot Admin Only",
	HideFromHelp:         true,
	Arguments: []*dcmd.ArgDef{
		{Name: "status", Type: dcmd.String, Default: ""},
	},
	ArgSwitches: []*dcmd.ArgDef{
		{Name: "url", Type: dcmd.String, Help: "The URL to the stream. Must be on twitch.tv or youtube.com. Activity type will always be streaming if this is set.", Default: ""},
		{Name: "type", Type: dcmd.String, Help: "Set activity type. Allowed values are 'playing', 'streaming', 'listening', 'watching', 'custom', 'competing'. Defaults to custom status", Default: "custom"},
		{Name: "status", Type: dcmd.String, Help: "Set online status. Allowed values are 'online', 'idle', 'dnd', 'offline'. Defaults to online", Default: "online"},
	},
	RunFunc: util.RequireBotAdmin(func(data *dcmd.Data) (interface{}, error) {
		activityType := data.Switch("type").Str()
		statusType := data.Switch("status").Str()
		statusText := data.Args[0].Str()
		streamingUrl := data.Switch("url").Str()
		switch activityType {
		case "playing", "streaming", "listening", "watching", "custom", "competing":
			// Valid activity type, do nothing
		default:
			return nil, commands.NewUserError(fmt.Sprintf("Invalid activity type %q. Allowed values are 'playing', 'streaming', 'listening', 'watching', 'custom', 'competing'", activityType))
		}
		switch statusType {
		case "online", "idle", "dnd", "offline":
			// Valid status type, do nothing
		default:
			return nil, commands.NewUserError(fmt.Sprintf("Invalid status type %q. Allowed values are 'online', 'idle', 'dnd', 'offline'", statusType))
		}
		bot.SetStatus(activityType, statusType, statusText, streamingUrl)
		return "Doneso", nil
	}),
}
