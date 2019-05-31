package timezonecompanion

const DBSchema = `
CREATE TABLE IF NOT EXISTS timezone_guild_configs (
	guild_id BIGINT PRIMARY KEY,
	disabled_in_channels BIGINT[]
);

CREATE TABLE IF NOT EXISTS user_timezones(
	user_id BIGINT PRIMARY KEY,
	timezone_name TEXT NOT NULL
);
`
