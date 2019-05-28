package twitter

const DBSchema = `
CREATE TABLE IF NOT EXISTS twitter_feeds (
	id BIGSERIAL PRIMARY KEY,
	guild_id BIGINT NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE NOT NULL,

	twitter_username TEXT NOT NULL,
	twitter_user_id BIGINT NOT NULL,
	channel_id BIGINT NOT NULL,
	enabled BOOLEAN NOT NULL
);

CREATE INDEX IF NOT EXISTS twitter_user_id_idx ON twitter_feeds(twitter_user_id);
`
