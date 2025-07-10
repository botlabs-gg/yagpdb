package logs

var DBSchemas = []string{`
CREATE TABLE IF NOT EXISTS message_logs2 (
	id INT NOT NULL,
	guild_id BIGINT NOT NULL,
	legacy_id INT NOT NULL,

	created_at TIMESTAMP WITH TIME ZONE NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE NOT NULL,

	channel_name TEXT NOT NULL,
	channel_id BIGINT NOT NULL,
	author_id BIGINT NOT NULL,
	author_username TEXT NOT NULL,

	messages BIGINT[],

	PRIMARY KEY(guild_id, id)
);
`,
	//CREATE INDEX IF NOT EXISTS message_logs2_guild_id_idx ON message_logs2(guild_id);
	`
CREATE TABLE IF NOT EXISTS messages2 (
	id BIGINT PRIMARY KEY,
	guild_id BIGINT NOT NULL,

	created_at TIMESTAMP WITH TIME ZONE NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
	deleted BOOLEAN NOT NULL,

	author_username TEXT NOT NULL,
	author_id BIGINT NOT NULL,

	content TEXT NOT NULL
);
`, `

CREATE TABLE IF NOT EXISTS guild_logging_configs (
	guild_id BIGINT PRIMARY KEY,

	created_at TIMESTAMP WITH TIME ZONE,
	updated_at TIMESTAMP WITH TIME ZONE,

	username_logging_enabled BOOLEAN,
	nickname_logging_enabled BOOLEAN,

	blacklisted_channels TEXT,
	manage_messages_can_view_deleted BOOLEAN,
	everyone_can_view_deleted BOOLEAN
);`,

	`ALTER TABLE guild_logging_configs ADD COLUMN IF NOT EXISTS message_logs_allowed_roles BIGINT[];`,
	`ALTER TABLE guild_logging_configs ADD COLUMN IF NOT EXISTS access_mode SMALLINT NOT NULL DEFAULT 0;`,
	`ALTER TABLE guild_logging_configs ADD COLUMN IF NOT EXISTS channels_whitelist_mode BOOLEAN NOT NULL DEFAULT FALSE;`,

	`CREATE TABLE IF NOT EXISTS username_listings (
	id SERIAL PRIMARY KEY,

	created_at TIMESTAMP WITH TIME ZONE,
	updated_at TIMESTAMP WITH TIME ZONE,
	deleted_at TIMESTAMP WITH TIME ZONE,

	user_id BIGINT,
	username TEXT
);`,

	`CREATE INDEX IF NOT EXISTS idx_username_listings_user_id ON username_listings(user_id);`,

	`CREATE TABLE IF NOT EXISTS nickname_listings (
	id SERIAL PRIMARY KEY,

	created_at TIMESTAMP WITH TIME ZONE,
	updated_at TIMESTAMP WITH TIME ZONE,
	deleted_at TIMESTAMP WITH TIME ZONE,

	user_id BIGINT,
	guild_id TEXT,
	nickname TEXT
);`,

	`CREATE INDEX IF NOT EXISTS idx_nickname_listings_deleted_at ON nickname_listings(deleted_at);`,

	// old unused indexes, didn't sort by id, means that postgres has to sort all the dudes nicknames to find the last one, could be slow on a lot of nicknames...
	// there's also no point in having a seperate user_id index, the combined one below can be used
	`DROP INDEX IF EXISTS idx_nickname_listings_user_id;`,
	`DROP INDEX IF EXISTS nickname_listings_user_id_guild_idx;`,

	// better indexes that has results sorted by id
	`CREATE INDEX IF NOT EXISTS nickname_listings_user_id_guild_id_id_idx ON nickname_listings(user_id, guild_id, id);`,
	`CREATE INDEX IF NOT EXISTS username_listings_user_id_id_idx ON username_listings(user_id, id);`,
}
