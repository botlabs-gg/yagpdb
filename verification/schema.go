package verification

var DBSchemas = []string{`
CREATE TABLE IF NOT EXISTS verification_configs  (
	guild_id BIGINT PRIMARY KEY,

	enabled BOOLEAN NOT NULL,

	verified_role BIGINT NOT NULL,

	page_content TEXT NOT NULL,

	kick_unverified_after INT NOT NULL,
	warn_unverified_after INT NOT NULL,
	warn_message TEXT NOT NULL,

	log_channel BIGINT NOT NULL
);
`, `
ALTER TABLE verification_configs ADD COLUMN IF NOT EXISTS dm_message TEXT NOT NULL DEFAULT '';
`, `
CREATE TABLE IF NOT EXISTS verification_sessions  (
	token TEXT PRIMARY KEY,
	user_id BIGINT NOT NULL,
	guild_id BIGINT NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE NOT NULL,

	solved_at TIMESTAMP WITH TIME ZONE,
	expired_at TIMESTAMP WITH TIME ZONE
);
`, `
CREATE TABLE IF NOT EXISTS verified_users (
	guild_id BIGINT NOT NULL,
	user_id BIGINT NOT NULL,

	verified_at TIMESTAMP WITH TIME ZONE NOT NULL,
	ip TEXT NOT NULL,

	PRIMARY KEY(guild_id, user_id)
);
`}
