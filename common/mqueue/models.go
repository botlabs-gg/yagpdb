package mqueue

import (
	"database/sql"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
)

type QueuedElement struct {
	Channel int64
	Guild   int64

	ID int64

	// Where this feed originated from, responsible for handling discord specific errors
	Source string
	// Could be stuff like reddit feed element id, youtube feed element id and so on
	SourceID string

	// The actual message as a simple string
	MessageStr string `json:",omitempty"`

	// The actual message as an embed
	MessageEmbed *discordgo.MessageEmbed `json:",omitempty"`
	// The channel to send the message in

	UseWebhook      bool
	WebhookUsername string
}

type Webhook struct {
	ID    int64
	Token string

	GuildID   int64
	ChannelID int64

	Plugin string
}

func FindCreateWebhook(guildID int64, channelID int64, plugin string, avatar string) (*Webhook, error) {
	const query = `
SELECT id, guild_id, channel_id, token, plugin FROM mqueue_webhooks
WHERE guild_id=$1 AND channel_id=$2 AND plugin=$3;
`

	row := common.PQ.QueryRow(query, guildID, channelID, plugin)

	var hook Webhook
	err := row.Scan(&hook.ID, &hook.GuildID, &hook.ChannelID, &hook.Token, &hook.Plugin)
	if err != nil {
		if err == sql.ErrNoRows {
			return CreateWebhook(guildID, channelID, plugin, avatar)
		}

		return nil, err
	}

	return &hook, nil
}

func CreateWebhook(guildID int64, channelID int64, plugin string, avatar string) (*Webhook, error) {
	discordHook, err := common.BotSession.WebhookCreate(channelID, plugin, avatar)
	if err != nil {
		return nil, err
	}

	const query = `
INSERT INTO mqueue_webhooks (id, guild_id, channel_id, token, plugin)
VALUES ($1, $2, $3, $4, $5);
`

	_, err = common.PQ.Exec(query, discordHook.ID, guildID, channelID, discordHook.Token, plugin)
	if err != nil {
		return nil, err
	}

	return &Webhook{
		ID:        discordHook.ID,
		Token:     discordHook.Token,
		GuildID:   guildID,
		ChannelID: channelID,
		Plugin:    plugin,
	}, nil
}
