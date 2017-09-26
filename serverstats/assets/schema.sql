CREATE TABLE IF NOT EXISTS server_stats_periods (
	id bigserial NOT NULL PRIMARY KEY,
	
	started  timestamptz,
	duration bigint,

	guild_id   bigint,
	user_id    bigint,
	channel_id bigint,
	count     bigint
);

CREATE INDEX IF NOT EXISTS serverstats_periods_guild_idx on server_stats_periods(guild_id);
CREATE INDEX IF NOT EXISTS started_x on server_stats_periods(started);

