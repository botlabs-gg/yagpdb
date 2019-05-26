package logs

const DBSchema = `
CREATE TABLE IF NOT EXISTS message_logs (
	id SERIAL PRIMARY KEY,
	created_at TIMESTAMP WITH TIME ZONE,
	updated_at TIMESTAMP WITH TIME ZONE,
	deleted_at TIMESTAMP WITH TIME ZONE,

	channel_name TEXT,
	channel_id TEXT,
	guild_id TEXT,
	author TEXT,
	author_id TEXT
);

CREATE INDEX IF NOT EXISTS idx_message_logs_deleted_at ON message_logs(deleted_at);

CREATE TABLE IF NOT EXISTS messages (
	id SERIAL PRIMARY KEY,

	created_at TIMESTAMP WITH TIME ZONE,
	updated_at TIMESTAMP WITH TIME ZONE,
	-- deleted_at TIMESTAMP WITH TIME ZONE, Note: this column exists in setups from below 1.15, but during the upgrade to sqlboiler i deemed it useless and don't include it in the schema anymore

	message_log_id INT REFERENCES message_logs(id) ON DELETE CASCADE,
	message_id TEXT,

	author_username TEXT,
	author_discrim TEXT,
	author_id TEXT,
	deleted BOOLEAN,

	content TEXT,
	timestamp TEXT
);

CREATE INDEX IF NOT EXISTS idx_messages_message_id ON messages(message_id);
CREATE INDEX IF NOT EXISTS idx_messages_message_log_id ON messages(message_log_id);

CREATE TABLE IF NOT EXISTS guild_logging_configs (
	guild_id BIGINT PRIMARY KEY,

	created_at TIMESTAMP WITH TIME ZONE,
	updated_at TIMESTAMP WITH TIME ZONE,

	username_logging_enabled BOOLEAN,
	nickname_logging_enabled BOOLEAN,

	blacklisted_channels TEXT,
	manage_messages_can_view_deleted BOOLEAN,
	everyone_can_view_deleted BOOLEAN
);

ALTER TABLE guild_logging_configs ADD COLUMN IF NOT EXISTS message_logs_allowed_roles BIGINT[];

CREATE TABLE IF NOT EXISTS username_listings (
	id SERIAL PRIMARY KEY,

	created_at TIMESTAMP WITH TIME ZONE,
	updated_at TIMESTAMP WITH TIME ZONE,
	deleted_at TIMESTAMP WITH TIME ZONE,

	user_id BIGINT,
	username TEXT
);

CREATE INDEX IF NOT EXISTS idx_username_listings_user_id ON username_listings(user_id);

CREATE TABLE IF NOT EXISTS nickname_listings (
	id SERIAL PRIMARY KEY,

	created_at TIMESTAMP WITH TIME ZONE,
	updated_at TIMESTAMP WITH TIME ZONE,
	deleted_at TIMESTAMP WITH TIME ZONE,

	user_id BIGINT,
	guild_id TEXT,
	nickname TEXT
);

CREATE INDEX IF NOT EXISTS idx_nickname_listings_deleted_at ON nickname_listings(deleted_at);

-- old unused indexes, didn't sort by id, means that postgres has to sort all the dudes nicknames to find the last one, could be slow on a lot of nicknames...
-- there's also no point in having a seperate user_id index, the combined one below can be used
DROP INDEX IF EXISTS idx_nickname_listings_user_id;
DROP INDEX IF EXISTS nickname_listings_user_id_guild_idx;

-- better index that has results sorted by id
CREATE INDEX IF NOT EXISTS nickname_listings_user_id_guild_id_id_idx ON nickname_listings(user_id, guild_id, id);
`
