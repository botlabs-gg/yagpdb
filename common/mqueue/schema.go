package mqueue

const DBSchema = `
CREATE TABLE IF NOT EXISTS mqueue_webhooks (
	id BIGINT PRIMARY KEY,

	guild_id BIGINT NOT NULL,
	channel_id BIGINT NOT NULL,
	token TEXT NOT NULL,

	plugin TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS mqueue_webhooks_channel_id_idx ON mqueue_webhooks(channel_id);
`
