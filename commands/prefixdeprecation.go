package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/config"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/mediocregopher/radix/v3"
	"github.com/sirupsen/logrus"
)

const prefixAlertLifetime = 10 * time.Second

const prefixAlertCooldown = 30 * time.Second

const prefixCommandsDocsURL = "https://help.yagpdb.xyz/docs/core/command-settings/#text-commands-have-been-discontinued"

const prefixAlertColor = 0xeaed40

// /admin/config reports what the web process resolved, which on a sharded
// deployment is neither the process that reads these nor the host they are set on.
func logPrefixCommandSettings() {
	logger.WithFields(logrus.Fields{
		"prefix_triggers_disabled": CommandSystem.DisablePrefixTrigger,
		"prefix_triggers_source":   configSource(common.ConfEnablePrefixCommands),
		"alert_enabled":            common.ShowPrefixCommandsWarning(),
		"alert_source":             configSource(common.ConfEnablePrefixCommandsWarning),
	}).Info("resolved prefix command settings")
}

func configSource(opt *config.ConfigOption) string {
	if opt.ConfigSource == nil {
		return "default"
	}

	return opt.ConfigSource.Name()
}

func alertPrefixCommandDisabled(evt *eventsystem.EventData, msg *discordgo.Message, prefix string) {
	if !common.ShowPrefixCommandsWarning() || evt.GS == nil {
		return
	}

	if prefix == "" {
		logger.WithField("guild", evt.GS.ID).Warn("prefix alert skipped, no prefix resolved")
		return
	}

	data, cmd := resolvePrefixCommand(msg, prefix)
	if cmd == nil {
		return
	}

	if onPrefixAlertCooldown(evt.GS.ID, msg.Author.ID) {
		return
	}

	canExecute, _, _, err := cmd.checkCanExecuteCommand(data)
	if err != nil || !canExecute {
		return
	}

	name := canonicalCommandName(cmd)

	send := &discordgo.MessageSend{
		Reference: &discordgo.MessageReference{
			MessageID: msg.ID,
			ChannelID: msg.ChannelID,
			GuildID:   msg.GuildID,
		},
		// Zero value leaves replied_user false, so the reply does not ping.
		AllowedMentions: discordgo.AllowedMentions{},
	}

	usage := commandUsage(cmd, name)
	if hasEmbedLinks, _ := bot.BotHasPermissionGS(evt.GS, msg.ChannelID, discordgo.PermissionEmbedLinks); hasEmbedLinks {
		send.Embeds = []*discordgo.MessageEmbed{{
			Description: fmt.Sprintf("[Prefix commands have been disabled](%s). Use %s instead.",
				prefixCommandsDocsURL, usage),
			Color: prefixAlertColor,
		}}
	} else {
		send.Content = fmt.Sprintf("Prefix commands have been disabled. Use %s instead: <%s>",
			usage, prefixCommandsDocsURL)
	}

	sent, err := common.BotSession.ChannelMessageSendComplex(msg.ChannelID, send)
	if err != nil {
		logger.WithError(err).WithField("guild", evt.GS.ID).Warn("failed sending prefix deprecation alert")
		return
	}

	setPrefixAlertCooldown(evt.GS.ID, msg.Author.ID)

	time.AfterFunc(prefixAlertLifetime, func() {
		bot.MessageDeleteQueue.DeleteMessages(msg.GuildID, sent.ChannelID, sent.ID)
	})
}

// Walks the container chain the way a mention triggered command does, so the
// data carries the chain checkCanExecuteCommand needs to resolve overrides.
// Returns a nil command for plain text and for custom command triggers.
func resolvePrefixCommand(msg *discordgo.Message, prefix string) (*dcmd.Data, *YAGCommand) {
	if !strings.HasPrefix(msg.Content, prefix) {
		return nil, nil
	}

	data, err := CommandSystem.FillDataLegacyMessage(common.BotSession, msg)
	if err != nil {
		return nil, nil
	}

	data.TriggerType = dcmd.TriggerTypePrefix
	data.TraditionalTriggerData.PrefixUsed = prefix
	data.TraditionalTriggerData.MessageStrippedPrefix = strings.TrimSpace(strings.TrimPrefix(msg.Content, prefix))

	container := CommandSystem.Root
	for {
		data.ContainerChain = append(data.ContainerChain, container)

		found, rest := container.FindCommand(data.TraditionalTriggerData.MessageStrippedPrefix)
		if found == nil {
			return nil, nil
		}

		data.TraditionalTriggerData.MessageStrippedPrefix = rest

		if sub, ok := found.Command.(*dcmd.Container); ok {
			container = sub
			continue
		}

		cmd, ok := found.Command.(*YAGCommand)
		if !ok {
			return nil, nil
		}

		data.Cmd = found
		return data, cmd
	}
}

// Commands kept off the slash command list, prefix among them, have no slash
// form to point at, so the mention is the only way left to run them.
func commandUsage(cmd *YAGCommand, name string) string {
	if cmd.SlashCommandEnabled {
		return SlashCommandMention(name)
	}

	return "<@" + discordgo.StrID(common.BotUser.ID) + "> " + name
}

func canonicalCommandName(cmd *YAGCommand) string {
	if canonical := cmd.CanonicalName(); canonical != "" {
		return strings.ToLower(canonical)
	}

	return strings.ToLower(cmd.Name)
}

// Read before the override lookup, which hits the database, and only set once
// an alert has actually been sent, so a command the user cannot run does not
// cost them the alert for one they can.
func onPrefixAlertCooldown(guildID, userID int64) bool {
	var exists int
	err := common.RedisPool.Do(radix.Cmd(&exists, "EXISTS", prefixAlertCooldownKey(guildID, userID)))
	if err != nil {
		logger.WithError(err).WithField("guild", guildID).Warn("failed checking prefix alert cooldown")
		return true
	}

	return exists > 0
}

func setPrefixAlertCooldown(guildID, userID int64) {
	key := prefixAlertCooldownKey(guildID, userID)

	err := common.RedisPool.Do(radix.FlatCmd(nil, "SET", key, 1, "EX", int(prefixAlertCooldown/time.Second)))
	if err != nil {
		logger.WithError(err).WithField("guild", guildID).Warn("failed setting prefix alert cooldown")
	}
}

func prefixAlertCooldownKey(guildID, userID int64) string {
	return "prefix_alert_cooldown:" + discordgo.StrID(guildID) + ":" + discordgo.StrID(userID)
}
