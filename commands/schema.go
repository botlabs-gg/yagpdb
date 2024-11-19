package commands

var DBSchemas = []string{`
CREATE TABLE IF NOT EXISTS commands_channels_overrides (
	id BIGSERIAL PRIMARY KEY,
	guild_id BIGINT NOT NULL,

	channels BIGINT[],
	channel_categories BIGINT[],
	global bool NOT NULL,

	commands_enabled BOOL NOT NULL,
	always_ephemeral BOOL NOT NULL,

	autodelete_response BOOL NOT NULL,
	autodelete_trigger BOOL NOT NULL,

	autodelete_response_delay INT NOT NULL,
	autodelete_trigger_delay INT NOT NULL,

	require_roles BIGINT[] NOT NULL,
	ignore_roles BIGINT[] NOT NULL
);
`, `
CREATE INDEX IF NOT EXISTS commands_channels_overrides_guild_idx ON commands_channels_overrides(guild_id);
`, `
CREATE UNIQUE INDEX IF NOT EXISTS commands_channels_overrides_global_uniquex ON commands_channels_overrides (guild_id) WHERE global;
`, `
ALTER TABLE commands_channels_overrides ADD COLUMN IF NOT EXISTS always_ephemeral BOOLEAN NOT NULL DEFAULT false;
`, `
CREATE TABLE IF NOT EXISTS commands_command_overrides (
	id BIGSERIAL PRIMARY KEY,
	guild_id BIGINT NOT NULL,
	commands_channels_overrides_id BIGINT references commands_channels_overrides(id) ON DELETE CASCADE NOT NULL,
	
	commands TEXT[] NOT NULL,

	commands_enabled BOOL NOT NULL,
	always_ephemeral BOOL NOT NULL,

	autodelete_response BOOL NOT NULL,
	autodelete_trigger BOOL NOT NULL,

	autodelete_response_delay INT NOT NULL,
	autodelete_trigger_delay INT NOT NULL,

	require_roles BIGINT[] NOT NULL,
	ignore_roles BIGINT[] NOT NULL
);
`, `
CREATE INDEX IF NOT EXISTS commands_command_groups_channels_override_idx ON commands_command_overrides(commands_channels_overrides_id);
`, `
ALTER TABLE commands_command_overrides ADD COLUMN IF NOT EXISTS always_ephemeral BOOLEAN NOT NULL DEFAULT false;
`}
