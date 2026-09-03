package commands

import (
	"fmt"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/mediocregopher/radix/v3"
)

const (
	prefixCommandUsesPerWarning = 10
	prefixCommandUsesExpirySecs = 30 * 24 * 60 * 60
)

func prefixCommandUsesKey(guildID int64) string {
	return "prefix_command_uses:" + discordgo.StrID(guildID)
}

func warnPrefixCommandDeprecated(data *dcmd.Data) *discordgo.Message {
	if !common.ShowPrefixCommandsWarning() {
		return nil
	}

	if data.TriggerType != dcmd.TriggerTypePrefix || data.GuildData == nil || data.TraditionalTriggerData == nil {
		return nil
	}

	if isExecutedByCustomCommand(data) || !isWarnedPrefixCommandUse(data.GuildData.GS.ID) {
		return nil
	}

	name := fullCommandName(data)
	content := fmt.Sprintf(
		"**Heads up:** prefixed commands are going away. From %s, `%s%s` will stop working "+
			"— use `/%s` or mention the bot instead. Custom commands are not affected.",
		common.PrefixCommandsShutdownDate.Format("2 January 2006"),
		data.TraditionalTriggerData.PrefixUsed, name, name,
	)

	msg, err := common.BotSession.ChannelMessageSendComplex(data.ChannelID, &discordgo.MessageSend{
		Content:         content,
		AllowedMentions: discordgo.AllowedMentions{},
	})
	if err != nil {
		logger.WithError(err).WithField("guild", data.GuildData.GS.ID).Warn("failed sending prefix deprecation warning")
		return nil
	}

	return msg
}

func isExecutedByCustomCommand(data *dcmd.Data) bool {
	executed, _ := data.Context().Value(CtxKeyExecutedByCC).(bool)
	return executed
}

func isWarnedPrefixCommandUse(guildID int64) bool {
	key := prefixCommandUsesKey(guildID)

	var uses int
	err := common.RedisPool.Do(radix.Pipeline(
		radix.Cmd(&uses, "INCR", key),
		radix.FlatCmd(nil, "EXPIRE", key, prefixCommandUsesExpirySecs),
	))
	if err != nil {
		logger.WithError(err).WithField("guild", guildID).Warn("failed counting prefix command uses")
		return false
	}

	return uses%prefixCommandUsesPerWarning == 1
}

func fullCommandName(data *dcmd.Data) string {
	name := data.Cmd.FormatNames(false, " ")
	if len(data.ContainerChain) > 1 {
		container := data.ContainerChain[len(data.ContainerChain)-1]
		if len(container.Names) > 0 {
			name = container.Names[0] + " " + name
		}
	}

	return strings.ToLower(name)
}
