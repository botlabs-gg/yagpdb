CREATE TABLE IF NOT EXISTS serverstats_periods (
	id bigserial NOT NULL PRIMARY KEY,
	created_at timestamptz NOT NULL,
	updated_at timestamptz NOT NULL,
	
	started  timestamptz,
	duration bigint,

	guild_id   bigint,
	user_id    bigint,
	channel_id bigint,
	count     bigint
);

CREATE INDEX IF NOT EXISTS serverstats_periods_guild_idx on serverstats_periods(guild_id);
CREATE INDEX IF NOT EXISTS started_x on serverstats_periods(started);

