package customcommands

var DBSchemas = []string{`

CREATE TABLE IF NOT EXISTS custom_command_groups (
	id BIGSERIAL PRIMARY KEY,
	guild_id BIGINT NOT NULL,

	name TEXT NOT NULL,

	ignore_roles BIGINT[],
	ignore_channels BIGINT[],
	whitelist_roles BIGINT[],
	whitelist_channels BIGINT[]
);
`, `
ALTER TABLE custom_command_groups ADD COLUMN IF NOT EXISTS disabled BOOLEAN NOT NULL DEFAULT false;
`, `
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
`, `
CREATE INDEX IF NOT EXISTS custom_commands_guild_idx ON custom_commands(guild_id);
`, `
CREATE INDEX IF NOT EXISTS custom_commands_next_run_idx ON custom_commands(next_run);
`, ` 
ALTER TABLE custom_commands ADD COLUMN IF NOT EXISTS context_channel BIGINT NOT NULL DEFAULT 0;
`, ` 
ALTER TABLE custom_commands ADD COLUMN IF NOT EXISTS redirect_errors_channel BIGINT NOT NULL DEFAULT 0;
`, `
ALTER TABLE custom_commands ADD COLUMN IF NOT EXISTS reaction_trigger_mode SMALLINT NOT NULL DEFAULT 0;
`, `
ALTER TABLE custom_commands ADD COLUMN IF NOT EXISTS last_error TEXT NOT NULL DEFAULT '';
`, `
ALTER TABLE custom_commands ADD COLUMN IF NOT EXISTS last_error_time TIMESTAMP WITH TIME ZONE;
`, `
ALTER TABLE custom_commands ADD COLUMN IF NOT EXISTS run_count INT NOT NULL DEFAULT 0;
`, `
ALTER TABLE custom_commands ADD COLUMN IF NOT EXISTS show_errors BOOLEAN NOT NULL DEFAULT true;
`, `
ALTER TABLE custom_commands ADD COLUMN IF NOT EXISTS disabled BOOLEAN NOT NULL DEFAULT false;
`, `
ALTER TABLE custom_commands ADD COLUMN IF NOT EXISTS trigger_on_edit BOOLEAN NOT NULL DEFAULT false;
`, `
ALTER TABLE custom_commands ADD COLUMN IF NOT EXISTS public_id TEXT NOT NULL DEFAULT '';
`, `
ALTER TABLE custom_commands ADD COLUMN IF NOT EXISTS name TEXT;
`, `
ALTER TABLE custom_commands ADD COLUMN IF NOT EXISTS public BOOLEAN NOT NULL DEFAULT false;
`, `
CREATE INDEX IF NOT EXISTS custom_commands_public_id_idx ON custom_commands(public_id);
`, `
ALTER TABLE custom_commands ADD COLUMN IF NOT EXISTS import_count INT NOT NULL DEFAULT 0;
`, `
ALTER TABLE custom_commands ADD COLUMN IF NOT EXISTS interaction_defer_mode SMALLINT NOT NULL DEFAULT 0;
`, `
CREATE TABLE IF NOT EXISTS templates_user_database (
	id BIGSERIAL PRIMARY KEY,

	created_at TIMESTAMP WITH TIME ZONE NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
	expires_at TIMESTAMP WITH TIME ZONE,

	guild_id BIGINT NOT NULL,
	user_id BIGINT NOT NULL,

	key TEXT NOT NULL,
	value_num DOUBLE PRECISION NOT NULL,
	value_raw BYTEA NOT NULL,

	UNIQUE(guild_id, user_id, key)
);

`, `
CREATE INDEX IF NOT EXISTS templates_user_database_combined_idx ON templates_user_database (guild_id, user_id, key, value_num);
`, `
CREATE INDEX IF NOT EXISTS templates_user_database_expires_idx ON templates_user_database (expires_at);
`}
