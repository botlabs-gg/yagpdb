package mqueue

import (
	"encoding/json"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

type DiscordProcessor struct {
}

func (d *DiscordProcessor) ProcessItem(resp chan *workResult, wi *workItem) {
	metricsProcessed.With(prometheus.Labels{"source": wi.Elem.Source}).Inc()

	retry := false
	defer func() {
		resp <- &workResult{
			item:  wi,
			retry: retry,
		}
	}()

	queueLogger := logger.WithField("mq_id", wi.Elem.ID)

	var err error
	if wi.Elem.UseWebhook {
		err = trySendWebhook(queueLogger, wi.Elem)
	} else {
		err = trySendNormal(queueLogger, wi.Elem)
	}

	if err == nil {
		return
	}

	if e, ok := errors.Cause(err).(*discordgo.RESTError); ok {
		if (e.Response != nil && e.Response.StatusCode >= 400 && e.Response.StatusCode < 500) || (e.Message != nil && e.Message.Code != 0) {
			if source, ok := sources[wi.Elem.Source]; ok {
				maybeDisableFeed(source, wi.Elem, e)
			}

			return
		}
	} else {
		if onGuild, err := common.BotIsOnGuild(wi.Elem.GuildID); !onGuild && err == nil {
			if source, ok := sources[wi.Elem.Source]; ok {
				logger.WithError(err).Warnf("disabling feed item %s from %s to nonexistant guild", wi.Elem.SourceItemID, wi.Elem.Source)
				source.DisableFeed(wi.Elem, err)
			}

			return
		} else if err != nil {
			logger.WithError(err).Error("failed checking if bot is on guild")
		}
	}

	if c, _ := common.DiscordError(err); c != 0 {
		return
	}

	retry = true
	queueLogger.Warn("Non-discord related error when sending message, retrying. ", err)
	time.Sleep(time.Second)

}

var disableOnError = []int{
	discordgo.ErrCodeUnknownChannel,
	discordgo.ErrCodeMissingAccess,
	discordgo.ErrCodeMissingPermissions,
	30007,  // max number of webhooks
	220001, // webhook points to a forum channel
}

func maybeDisableFeed(source PluginWithSourceDisabler, elem *QueuedElement, err *discordgo.RESTError) {
	// source.HandleMQueueError(elem, errors.Cause(err))
	if err.Message == nil || !common.ContainsIntSlice(disableOnError, err.Message.Code) {
		// don't disable
		l := logger.WithError(err).WithField("source", elem.Source).WithField("sourceid", elem.SourceItemID)
		if elem.MessageEmbed != nil {
			serializedEmbed, _ := json.Marshal(elem.MessageEmbed)
			l = l.WithField("embed", serializedEmbed)
		}

		l.Error("error sending mqueue message")
		return
	}

	logger.WithError(err).Warnf("disabling feed item %s from %s", elem.SourceItemID, elem.Source)
	source.DisableFeed(elem, err)
}

func trySendNormal(l *logrus.Entry, elem *QueuedElement) (err error) {
	send := func(msg *discordgo.MessageSend) error {
		m, err := common.BotSession.ChannelMessageSendComplex(elem.ChannelID, msg)
		if err != nil {
			logrus.WithError(err).Errorf("Failed sending mqueue message %#v", msg)
			return err
		}

		if elem.PublishAnnouncement {
			_, err = common.BotSession.ChannelMessageCrosspost(elem.ChannelID, m.ID)
		}
		return err
	}

	if elem.MessageStr == "" && elem.MessageEmbed == nil && elem.MessageSend == nil {
		l.Error("Empty Send Item received, skipping.")
		return
	}

	if elem.MessageSend != nil {
		return send(elem.MessageSend)
	}

	msg := &discordgo.MessageSend{}
	if elem.MessageStr != "" {
		msg.Content = elem.MessageStr
		msg.AllowedMentions = elem.AllowedMentions
	}
	if elem.MessageEmbed != nil {
		msg.Embeds = []*discordgo.MessageEmbed{elem.MessageEmbed}
	}

	return send(msg)
}

var errGuildNotFound = errors.New("Guild not found")

func trySendWebhook(l *logrus.Entry, elem *QueuedElement) (err error) {
	// Helper to fetch or create the webhook
	getWebhook := func() (*webhook, string, error) {
		avatar := ""
		if source, ok := sources[elem.Source]; ok {
			if avatarProvider, ok := source.(PluginWithWebhookAvatar); ok {
				avatar = avatarProvider.WebhookAvatar()
			}
		}
		gs := bot.State.GetGuild(elem.GuildID)
		if gs == nil {
			if onGuild, err := common.BotIsOnGuild(elem.GuildID); err == nil && !onGuild {
				return nil, "", errGuildNotFound
			} else if err != nil {
				return nil, "", err
			}
		}
		whI, err := webhookCache.GetCustomFetch(elem.ChannelID, func(key interface{}) (interface{}, error) {
			return findCreateWebhook(elem.GuildID, elem.ChannelID, elem.Source, avatar)
		})
		if err != nil {
			return nil, "", err
		}
		return whI.(*webhook), avatar, nil
	}

	if elem.MessageSend != nil {
		wh, avatar, err := getWebhook()
		if err != nil {
			return err
		}
		params := &discordgo.WebhookParams{
			Content:         elem.MessageSend.Content,
			Username:        elem.WebhookUsername,
			AvatarURL:       avatar,
			Embeds:          elem.MessageSend.Embeds,
			Components:      elem.MessageSend.Components,
			Flags:           int64(elem.MessageSend.Flags),
			AllowedMentions: &elem.MessageSend.AllowedMentions,
		}
		_, err = webhookSession.WebhookExecuteComplex(wh.ID, wh.Token, true, params)
		if err != nil {
			logrus.WithError(err).Error("Failed sending mqueue v2 message via webhook (WebhookExecuteComplex)")
			return err
		}
		return nil
	}

	if elem.MessageStr == "" && elem.MessageEmbed == nil {
		l.Error("Both MessageEmbed and MessageStr empty")
		return
	}

	wh, _, err := getWebhook()
	if err != nil {
		return err
	}

	webhookParams := &discordgo.WebhookParams{
		Username:        elem.WebhookUsername,
		Content:         elem.MessageStr,
		AllowedMentions: &discordgo.AllowedMentions{},
	}

	if elem.MessageEmbed != nil {
		webhookParams.Embeds = []*discordgo.MessageEmbed{elem.MessageEmbed}
	}

	err = webhookSession.WebhookExecute(wh.ID, wh.Token, true, webhookParams)
	if code, _ := common.DiscordError(err); code == discordgo.ErrCodeUnknownWebhook {
		// webhook got deleted, try again
		webhookCache.Delete(elem.ChannelID)
		wh, _, err = getWebhook()
		if err != nil {
			return err
		}
		err = webhookSession.WebhookExecute(wh.ID, wh.Token, true, webhookParams)
	}
	return err
}
