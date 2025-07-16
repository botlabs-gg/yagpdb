package rss

var DBSchemas = []string{`
CREATE TABLE IF NOT EXISTS rss_feed_subscriptions (
	id SERIAL PRIMARY KEY,
	created_at TIMESTAMP WITH TIME ZONE NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE NOT NULL,

	guild_id BIGINT NOT NULL,
	channel_id BIGINT NOT NULL,
	feed_url TEXT NOT NULL,

	mention_everyone BOOLEAN NOT NULL,
	mention_roles BIGINT[],
	enabled BOOLEAN NOT NULL DEFAULT TRUE
);
`}
