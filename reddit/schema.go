package reddit

var DBSchemas = []string{`
CREATE TABLE IF NOT EXISTS reddit_feeds (
	id BIGSERIAL PRIMARY KEY,
	guild_id BIGINT NOT NULL,
	channel_id BIGINT NOT NULL,
	subreddit TEXT NOT NULL,

	-- 0 = none, 1 = ignore, 2 - whitelist
	filter_nsfw INT NOT NULL,
	min_upvotes INT NOT NULL,

	use_embeds BOOLEAN NOT NULL,
	slow BOOLEAN NOT NULL
);
`, `
CREATE INDEX IF NOT EXISTS redidt_feeds_guild_idx ON reddit_feeds(guild_id);
`, `
CREATE INDEX IF NOT EXISTS redidt_feeds_subreddit_idx ON reddit_feeds(subreddit);

`, `
ALTER TABLE reddit_feeds ADD COLUMN IF NOT EXISTS disabled BOOLEAN NOT NULL DEFAULT FALSE;
`}
