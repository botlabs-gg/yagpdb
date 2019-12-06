package serverstats

var legacyDBSchemas = []string{
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

var dbSchemas = []string{
	`
	CREATE TABLE IF NOT EXISTS server_stats_configs (
		guild_id BIGINT PRIMARY KEY,
		created_at TIMESTAMP WITH TIME ZONE,
		updated_at TIMESTAMP WITH TIME ZONE,
		public BOOLEAN,
		ignore_channels TEXT
	);
	`, `
	CREATE TABLE IF NOT EXISTS server_stats_hourly_periods_messages (
		guild_id BIGINT NOT NULL,
		t TIMESTAMP WITH TIME ZONE NOT NULL,
		compressed BOOLEAN NOT NULL DEFAULT FALSE,
	
		channel_id bigint,
		count     bigint,
	
		PRIMARY KEY(guild_id, channel_id, t)
	);
	`, `
	CREATE TABLE IF NOT EXISTS server_stats_hourly_periods_misc (
		guild_id BIGINT NOT NULL,
		t TIMESTAMP WITH TIME ZONE NOT NULL,
		compressed BOOLEAN NOT NULL DEFAULT FALSE,
	
		num_members BIGINT NOT NULL,
		max_online BIGINT NOT NULL,
		joins INT NOT NULL,
		leaves INT NOT NULL,
		max_voice INT NOT NULL,
	
		PRIMARY KEY(guild_id, t)
	);
	`, `
	CREATE TABLE IF NOT EXISTS server_stats_periods_compressed (
		guild_id BIGINT NOT NULL,
		t DATE NOT NULL,
		premium BOOLEAN NOT NULL,
	
		num_messages INT NOT NULL,
		num_members BIGINT NOT NULL,
		max_online BIGINT NOT NULL,
		joins INT NOT NULL,
		leaves INT NOT NULL,
		max_voice INT NOT NULL,
	
		PRIMARY KEY(guild_id, t)
	);
	`,
	// we don't care about indexing t of non-premium rows, this means we can also use it in the cleanup
	// without needing to filter out premium rows, since they're not included in the index at all
	`CREATE INDEX IF NOT EXISTS server_stats_periods_compressed_t_nonpremium_idx ON server_stats_periods_compressed(t) WHERE premium=false;`,
}
