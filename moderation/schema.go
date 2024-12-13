package moderation

var DBSchemas = []string{`
CREATE TABLE IF NOT EXISTS moderation_configs (
	guild_id BIGINT PRIMARY KEY,
	created_at TIMESTAMP WITH TIME ZONE NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
	
	-- Many of the following columns should be non-nullable, but were originally
	-- managed by gorm (which does not add NOT NULL constraints by default) so are
	-- missing them. Unfortunately, it is unfeasible to retroactively fill missing
	-- values with defaults and add the constraints as there are simply too many
	-- rows in production.

	-- For similar legacy reasons, many fields that should have type BIGINT are TEXT.

	kick_enabled BOOLEAN,
	kick_cmd_roles BIGINT[],
	delete_messages_on_kick BOOLEAN,
	kick_reason_optional BOOLEAN,
	kick_message TEXT,

	ban_enabled BOOLEAN,
	ban_cmd_roles BIGINT[],
	ban_reason_optional BOOLEAN,
	ban_message TEXT,
	default_ban_delete_days BIGINT DEFAULT 1,

	timeout_enabled BOOLEAN,
	timeout_cmd_roles BIGINT[],
	timeout_reason_optional BOOLEAN,
	timeout_remove_reason_optional BOOLEAN,
	timeout_message TEXT,
	default_timeout_duration BIGINT DEFAULT 10,

	mute_enabled BOOLEAN,
	mute_cmd_roles BIGINT[],
	mute_role TEXT,
	mute_disallow_reaction_add BOOLEAN,
	mute_reason_optional BOOLEAN,
	unmute_reason_optional BOOLEAN,
	mute_manage_role BOOLEAN,
	mute_remove_roles BIGINT[],
	mute_ignore_channels BIGINT[],
	mute_message TEXT,
	unmute_message TEXT,
	default_mute_duration BIGINT DEFAULT 10,

	warn_commands_enabled BOOLEAN,
	warn_cmd_roles BIGINT[],
	warn_include_channel_logs BOOLEAN,
	warn_send_to_modlog BOOLEAN,
	warn_message TEXT,

	clean_enabled BOOLEAN,
	report_enabled BOOLEAN,
	action_channel TEXT,
	report_channel TEXT,
	error_channel TEXT,
	log_unbans BOOLEAN,
	log_bans BOOLEAN,
	log_kicks BOOLEAN DEFAULT TRUE,
	log_timeouts BOOLEAN,

	give_role_cmd_enabled BOOLEAN,
	give_role_cmd_modlog BOOLEAN,
	give_role_cmd_roles BIGINT[]
);
`, `
-- Tables created with gorm have missing NOT NULL constraints for created_at and
-- updated_at columns; since these columns are never null in existing rows, we can
-- retraoctively add the constraints without needing to update any data.

ALTER TABLE moderation_configs ALTER COLUMN created_at SET NOT NULL;
`, `
ALTER TABLE moderation_configs ALTER COLUMN updated_at SET NOT NULL;
`, `
ALTER TABLE moderation_configs ADD COLUMN IF NOT EXISTS delwarn_send_to_modlog BOOLEAN NOT NULL DEFAULT false;
`, `
ALTER TABLE moderation_configs ADD COLUMN IF NOT EXISTS delwarn_include_warn_reason BOOLEAN NOT NULL DEFAULT false;
`, `

CREATE TABLE IF NOT EXISTS moderation_warnings (
	id SERIAL PRIMARY KEY,
	created_at TIMESTAMP WITH TIME ZONE NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE NOT NULL,

	guild_id BIGINT NOT NULL,
	user_id TEXT NOT NULL, -- text instead of bigint for legacy compatibility
	author_id TEXT NOT NULL,

	author_username_discrim TEXT NOT NULL,

	message TEXT NOT NULL,
	logs_link TEXT
);
`, `
CREATE INDEX IF NOT EXISTS idx_moderation_warnings_guild_id ON moderation_warnings(guild_id);
`, `
-- Similar to moderation_warnings.{created_at,updated_at}, there are a number of
-- fields that are never null in existing data but do not have the proper NOT NULL
-- constraints if they were created with gorm. Add them in.

ALTER TABLE moderation_warnings ALTER COLUMN created_at SET NOT NULL;
`, `
ALTER TABLE moderation_warnings ALTER COLUMN updated_at SET NOT NULL;
`, `
ALTER TABLE moderation_warnings ALTER COLUMN guild_id SET NOT NULL;
`, `
ALTER TABLE moderation_warnings ALTER COLUMN user_id SET NOT NULL;
`, `
ALTER TABLE moderation_warnings ALTER COLUMN author_id SET NOT NULL;
`, `
ALTER TABLE moderation_warnings ALTER COLUMN author_username_discrim SET NOT NULL;
`, `
ALTER TABLE moderation_warnings ALTER COLUMN message SET NOT NULL;
`, `

CREATE TABLE IF NOT EXISTS muted_users (
	id SERIAL PRIMARY KEY,
	created_at TIMESTAMP WITH TIME ZONE NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE NOT NULL,

	expires_at TIMESTAMP WITH TIME ZONE,

	guild_id BIGINT NOT NULL,
	user_id BIGINT NOT NULL,

	author_id BIGINT NOT NULL,
	reason TEXT NOT NULL,

	removed_roles BIGINT[]
);
`, `

ALTER TABLE muted_users ALTER COLUMN created_at SET NOT NULL;
`, `
ALTER TABLE muted_users ALTER COLUMN updated_at SET NOT NULL;
`, `
ALTER TABLE muted_users ALTER COLUMN guild_id SET NOT NULL;
`, `
ALTER TABLE muted_users ALTER COLUMN user_id SET NOT NULL;
`, `
ALTER TABLE muted_users ALTER COLUMN author_id SET NOT NULL;
`, `
ALTER TABLE muted_users ALTER COLUMN reason SET NOT NULL;
`}
