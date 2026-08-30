package ping

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

var Command = &commands.YAGCommand{
	CmdCategory:     commands.CategoryDebug,
	Name:            "Ping",
	Description:     "Shows the latency from the bot to the discord servers.",
	LongDescription: "Note that high latencies can be the fault of ratelimits and the bot itself, it's not a absolute metric.",

	DefaultEnabled:      true,
	SlashCommandEnabled: true,

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		if data.TriggerType == dcmd.TriggerTypeSlashCommands {
			return runSlashCommand(data)
		}

		return fmt.Sprintf(":PONG;%d", time.Now().UnixNano()), nil
	},
}

// Interaction responses are webhook messages, which we can't edit through the
// normal message endpoint the ping-pong trick below relies on, so measure the
// latencies inline instead.
func runSlashCommand(data *dcmd.Data) (interface{}, error) {
	interaction := data.SlashCommandTriggerData.Interaction

	gatewayPing := "Unknown"
	if lastSend, lastAck := data.Session.GatewayManager.HeartBeatStats(); !lastSend.IsZero() && lastAck.After(lastSend) {
		gatewayPing = lastAck.Sub(lastSend).String()
	}

	started := time.Now()
	m, err := data.Session.CreateFollowupMessage(interaction.ApplicationID, interaction.Token, &discordgo.WebhookParams{
		Content: "Pinging...",
	})
	if err != nil {
		return nil, err
	}
	httpPing := time.Since(started)

	_, err = data.Session.EditFollowupMessage(interaction.ApplicationID, interaction.Token, m.ID, &discordgo.WebhookParams{
		Content: "HTTP API (Send Msg): " + httpPing.String() + "\nGateway (last heartbeat): " + gatewayPing,
	})
	if err != nil {
		return nil, err
	}

	return dcmd.MarkManualResponse([]*discordgo.Message{m}), nil
}

func HandleMessageCreate(evt *eventsystem.EventData) {
	m := evt.MessageCreate()

	bUser := common.BotUser
	if bUser == nil {
		return
	}

	if bUser.ID != m.Author.ID {
		return
	}

	// ping pong
	split := strings.Split(m.Content, ";")
	if split[0] != ":PONG" || len(split) < 2 {
		return
	}

	parsed, err := strconv.ParseInt(split[1], 10, 64)
	if err != nil {
		return
	}

	taken := time.Duration(time.Now().UnixNano() - parsed)

	started := time.Now()
	common.BotSession.ChannelMessageEdit(m.ChannelID, m.ID, "Gateway (http send -> gateway receive time): "+taken.String())
	httpPing := time.Since(started)

	common.BotSession.ChannelMessageEdit(m.ChannelID, m.ID, "HTTP API (Edit Msg): "+httpPing.String()+"\nGateway: "+taken.String())
}
