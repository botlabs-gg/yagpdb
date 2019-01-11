package customcommands

const DBSchema = `

CREATE TABLE IF NOT EXISTS custom_command_groups (
	id BIGSERIAL PRIMARY KEY,
	guild_id BIGINT NOT NULL,

	name TEXT NOT NULL,

	ignore_roles BIGINT[],
	ignore_channels BIGINT[],
	whitelist_roles BIGINT[],
	whitelist_channels BIGINT[]
);

CREATE TABLE IF NOT EXISTS custom_commands (
	local_id BIGINT NOT NULL,
	guild_id BIGINT NOT NULL,
	group_id BIGINT references custom_command_groups(id) ON DELETE SET NULL,

	trigger_type INT NOT NULL,
	text_trigger TEXT NOT NULL,
	text_trigger_case_sensitive BOOL NOT NULL,

	time_trigger_interval INT NOT NULL,
	time_trigger_excluding_days SMALLINT[] NOT NULL,
	time_trigger_excluding_hours SMALLINT[] NOT NULL,

	last_run TIMESTAMP WITH TIME ZONE,
	next_run TIMESTAMP WITH TIME ZONE,

	responses TEXT[] NOT NULL,

	channels BIGINT[],
	channels_whitelist_mode BOOL NOT NULL,

	roles BIGINT[],
	roles_whitelist_mode BOOL NOT NULL,
	
	PRIMARY KEY(guild_id, local_id)
);

CREATE INDEX IF NOT EXISTS custom_commands_guild_idx ON custom_commands(guild_id);
CREATE INDEX IF NOT EXISTS custom_commands_next_run_idx ON custom_commands(next_run);
`
