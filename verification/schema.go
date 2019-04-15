package verification

const DBSchema = `
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
`
