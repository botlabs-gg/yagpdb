package tickets

var DBSchemas = []string{`
CREATE TABLE IF NOT EXISTS ticket_configs  (
	guild_id BIGINT PRIMARY KEY,

	enabled BOOLEAN NOT NULL,

	ticket_open_msg TEXT NOT NULL,

	tickets_channel_category BIGINT NOT NULL,
	status_channel BIGINT NOT NULL,
	tickets_transcripts_channel BIGINT NOT NULL,
	download_attachments BOOLEAN NOT NULL,
	tickets_use_txt_transcripts BOOLEAN NOT NULL,

	mod_roles BIGINT[],
	admin_roles BIGINT[]
);
`, `
ALTER TABLE ticket_configs ADD COLUMN IF NOT EXISTS tickets_transcripts_channel_admin_only BIGINT NOT NULL DEFAULT 0;
`, `
ALTER TABLE ticket_configs ADD COLUMN IF NOT EXISTS append_buttons BIGINT NOT NULL DEFAULT 0;
`, `
CREATE TABLE IF NOT EXISTS tickets (
	guild_id BIGINT NOT NULL,
	local_id BIGINT NOT NULL,

	channel_id BIGINT NOT NULL,

	title TEXT NOT NULL,

	created_at TIMESTAMP WITH TIME ZONE NOT NULL,
	closed_at TIMESTAMP WITH TIME ZONE,

	logs_id BIGINT NOT NULL,

	author_id BIGINT NOT NULL,
	author_username_discrim TEXT NOT NULL,

	PRIMARY KEY(guild_id, local_id)
);

`, `
CREATE INDEX IF NOT EXISTS tickets_guild_id_channel_id_idx ON tickets(guild_id, channel_id);

`, `
CREATE TABLE IF NOT EXISTS ticket_participants (
	ticket_guild_id BIGINT NOT NULL,
	ticket_local_id BIGINT NOT NULL,

	user_id BIGINT NOT NULL,
	username TEXT NOT NULL,
	discrim TEXT NOT NULL,

	is_staff BOOLEAN NOT NULL,

	-- This is bugged in sqlboiler, sooooo don't use it for now i guess
	-- FOREIGN KEY (ticket_guild_id, ticket_local_id) REFERENCES tickets(guild_id, local_id) ON DELETE CASCADE,

	PRIMARY KEY(ticket_guild_id, ticket_local_id, user_id)


);
`, `

CREATE INDEX IF NOT EXISTS ticket_participants_ticket_local_id_idx ON ticket_participants(ticket_guild_id, ticket_local_id);
`}
