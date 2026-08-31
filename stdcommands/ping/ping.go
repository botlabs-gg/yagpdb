package ping

import (
	"fmt"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

const placeholder = "Pinging..."

var Command = &commands.YAGCommand{
	CmdCategory:     commands.CategoryDebug,
	Name:            "Ping",
	Description:     "Shows the latency from the bot to the discord servers.",
	LongDescription: "Note that high latencies can be the fault of ratelimits and the bot itself, it's not a absolute metric.",

	DefaultEnabled:      true,
	SlashCommandEnabled: true,

	RunFunc: runPing,
}

func runPing(data *dcmd.Data) (interface{}, error) {
	gatewayPing := "Unknown"
	if roundTrip, ok := lastHeartbeatRoundTrip(data); ok {
		gatewayPing = roundTrip.String()
	}

	sentAt := time.Now()
	msg, err := sendPlaceholder(data)
	if err != nil {
		return nil, err
	}
	httpPing := time.Since(sentAt)

	result := fmt.Sprintf("HTTP API (Send Msg): %s\nGateway (last heartbeat): %s", httpPing, gatewayPing)
	if err := editWithResult(data, msg, result); err != nil {
		return nil, err
	}

	return dcmd.MarkManualResponse([]*discordgo.Message{msg}), nil
}

func isInteraction(data *dcmd.Data) bool {
	return data.TriggerType == dcmd.TriggerTypeSlashCommands
}

func sendPlaceholder(data *dcmd.Data) (*discordgo.Message, error) {
	if isInteraction(data) {
		interaction := data.SlashCommandTriggerData.Interaction
		return data.Session.CreateFollowupMessage(interaction.ApplicationID, interaction.Token, &discordgo.WebhookParams{
			Content: placeholder,
		})
	}

	return data.Session.ChannelMessageSend(data.ChannelID, placeholder)
}

func editWithResult(data *dcmd.Data, msg *discordgo.Message, content string) error {
	if isInteraction(data) {
		interaction := data.SlashCommandTriggerData.Interaction
		_, err := data.Session.EditFollowupMessage(interaction.ApplicationID, interaction.Token, msg.ID, &discordgo.WebhookParams{
			Content: content,
		})
		return err
	}

	_, err := data.Session.ChannelMessageEdit(msg.ChannelID, msg.ID, content)
	return err
}

func lastHeartbeatRoundTrip(data *dcmd.Data) (time.Duration, bool) {
	for _, session := range gatewayConnectedSessions(data) {
		if session == nil || session.GatewayManager == nil {
			continue
		}

		lastSend, lastAck := session.GatewayManager.HeartBeatStats()
		if lastSend.IsZero() || !lastAck.After(lastSend) {
			continue
		}

		return lastAck.Sub(lastSend), true
	}

	return 0, false
}

// data.Session is the shared rest session, it never connects to the gateway.
func gatewayConnectedSessions(data *dcmd.Data) []*discordgo.Session {
	if bot.ShardManager == nil {
		return nil
	}

	sessions := make([]*discordgo.Session, 0, 2)

	if data.GuildData != nil && bot.ShardManager.GetNumShards() > 0 {
		guildID := data.GuildData.GS.ID
		if bot.ReadyTracker.IsGuildOnProcess(guildID) {
			sessions = append(sessions, bot.ShardManager.SessionForGuild(guildID))
		}
	}

	for _, shardID := range bot.ReadyTracker.GetProcessShards() {
		if shardID >= 0 && shardID < len(bot.ShardManager.Sessions) {
			sessions = append(sessions, bot.ShardManager.Sessions[shardID])
		}
	}

	return sessions
}
