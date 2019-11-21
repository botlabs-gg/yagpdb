package serverstats

var DBSchemas = []string{
	`
CREATE TABLE IF NOT EXISTS server_stats_periods (
	id bigserial NOT NULL PRIMARY KEY,
	
	started  timestamptz,
	duration bigint,

	guild_id   bigint,
	user_id    bigint,
	channel_id bigint,
	count     bigint
);`,

	// This index is no longer used, should be dropped
	// `CREATE INDEX IF NOT EXISTS serverstats_periods_guild_idx on server_stats_periods(guild_id);`,
	`CREATE INDEX IF NOT EXISTS started_x on server_stats_periods(started);`,
	`CREATE INDEX IF NOT EXISTS server_stats_periods_guild_id_started_idx on server_stats_periods(guild_id, started);`,

	`
CREATE TABLE IF NOT EXISTS server_stats_configs (
    guild_id BIGINT PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE,
    public BOOLEAN,
    ignore_channels TEXT
);`,

	`
CREATE TABLE IF NOT EXISTS server_stats_member_periods (
	id BIGSERIAL PRIMARY KEY,
	guild_id BIGINT NOT NULL,

	created_at TIMESTAMP WITH TIME ZONE NOT NULL,

	num_members BIGINT NOT NULL,
	joins BIGINT NOT NULL,
	leaves BIGINT NOT NULL,
	max_online BIGINT NOT NULL,

	UNIQUE(guild_id, created_at)
);`,
	`CREATE INDEX IF NOT EXISTS server_stats_member_periods_guild_idx on server_stats_member_periods(guild_id);`,
	`CREATE INDEX IF NOT EXISTS server_stats_member_periods_created_at_idx on server_stats_member_periods(created_at);`,
}
