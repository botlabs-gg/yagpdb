package timezonecompanion

var DBSchemas = []string{`
CREATE TABLE IF NOT EXISTS timezone_guild_configs (
	guild_id BIGINT PRIMARY KEY,

	disabled_in_channels BIGINT[],
	enabled_in_channels BIGINT[],

	new_channels_disabled BOOLEAN NOT NULL
);
`, `
ALTER TABLE timezone_guild_configs ADD COLUMN IF NOT EXISTS enabled_in_channels BIGINT[];
`, `
ALTER TABLE timezone_guild_configs ADD COLUMN IF NOT EXISTS new_channels_disabled BOOLEAN NOT NULL DEFAULT FALSE;
`, `
CREATE TABLE IF NOT EXISTS user_timezones(
	user_id BIGINT PRIMARY KEY,
	timezone_name TEXT NOT NULL
);`}
