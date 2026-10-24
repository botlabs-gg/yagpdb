package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

const prefixAlertLifetime = 10 * time.Second

const prefixCommandsDocsURL = "https://help.yagpdb.xyz/docs/core/command-settings/#text-commands-have-been-discontinued"

const prefixAlertColor = 0xeaed40

// Set by custom commands, which can be named after a built in command and would
// otherwise be alerted about despite having run.
var PrefixMessageClaimedBy func(evt *eventsystem.EventData, msg *discordgo.Message) bool

func alertPrefixCommandDisabled(evt *eventsystem.EventData, msg *discordgo.Message, prefix string) {
	if !common.ShowPrefixCommandsWarning() || prefix == "" || evt.GS == nil {
		return
	}

	name := prefixedCommandName(msg.Content, prefix)
	if name == "" {
		return
	}

	if PrefixMessageClaimedBy != nil && PrefixMessageClaimedBy(evt, msg) {
		return
	}

	sent, err := common.BotSession.ChannelMessageSendComplex(msg.ChannelID, &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{{
			Description: fmt.Sprintf("[Prefix commands have been disabled](%s). Use %s instead.",
				prefixCommandsDocsURL, SlashCommandMention(name)),
			Color: prefixAlertColor,
		}},
		Reference: &discordgo.MessageReference{
			MessageID: msg.ID,
			ChannelID: msg.ChannelID,
			GuildID:   msg.GuildID,
		},
		// Zero value leaves replied_user false, so the reply does not ping.
		AllowedMentions: discordgo.AllowedMentions{},
	})
	if err != nil {
		logger.WithError(err).WithField("guild", evt.GS.ID).Warn("failed sending prefix deprecation alert")
		return
	}

	time.AfterFunc(prefixAlertLifetime, func() {
		bot.MessageDeleteQueue.DeleteMessages(msg.GuildID, sent.ChannelID, sent.ID)
	})
}

// Returns "" for anything that is not a built in command, which is what keeps
// plain text and custom command triggers from being alerted about.
func prefixedCommandName(content, prefix string) string {
	if !strings.HasPrefix(content, prefix) {
		return ""
	}

	stripped := strings.TrimSpace(strings.TrimPrefix(content, prefix))
	if stripped == "" {
		return ""
	}

	found, _ := CommandSystem.Root.AbsFindCommand(stripped)
	if found == nil {
		return ""
	}

	cmd, ok := found.Command.(*YAGCommand)
	if !ok {
		return ""
	}

	if canonical := cmd.CanonicalName(); canonical != "" {
		return strings.ToLower(canonical)
	}

	return strings.ToLower(cmd.Name)
}
