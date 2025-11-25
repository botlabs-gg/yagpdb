package twitch

var DBSchemas = []string{`
CREATE TABLE IF NOT EXISTS twitch_channel_subscriptions (
	id SERIAL PRIMARY KEY,
	created_at TIMESTAMP WITH TIME ZONE NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE NOT NULL,

	guild_id TEXT NOT NULL,
	channel_id TEXT NOT NULL,
	twitch_user_id TEXT NOT NULL,
	twitch_username TEXT NOT NULL,

	mention_everyone BOOLEAN NOT NULL,
	mention_roles BIGINT[],
	
	publish_vod BOOLEAN NOT NULL DEFAULT false,
	enabled BOOLEAN NOT NULL DEFAULT TRUE
);
`, `
CREATE INDEX IF NOT EXISTS idx_twitch_user_id ON twitch_channel_subscriptions (twitch_user_id);
`, `
CREATE TABLE IF NOT EXISTS twitch_announcements (
	guild_id BIGINT PRIMARY KEY,
	message TEXT NOT NULL,
	enabled BOOLEAN NOT NULL DEFAULT FALSE
);
`}
