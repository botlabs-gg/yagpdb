package rsvp

var DBSchemas = []string{`
CREATE TABLE IF NOT EXISTS rsvp_sessions (
	message_id BIGINT PRIMARY KEY,

	guild_id BIGINT NOT NULL,
	channel_id BIGINT NOT NULL,
	local_id BIGINT NOT NULL,
	author_id BIGINT NOT NULL,

	created_at TIMESTAMP WITH TIME ZONE NOT NULL,
	starts_at TIMESTAMP WITH TIME ZONE NOT NULL,

	title TEXT NOT NULL,
	description TEXT NOT NULL,
	max_participants INT NOT NULL,

	send_reminders BOOLEAN NOT NULL,
	sent_reminders BOOLEAN NOT NULL
);
`, `
CREATE TABLE IF NOT EXISTS rsvp_participants (
	user_id BIGINT NOT NULL,
	rsvp_sessions_message_id BIGINT NOT NULL REFERENCES rsvp_sessions(message_id) ON DELETE CASCADE,

	guild_id BIGINT NOT NULL,

	join_state SMALLINT NOT NULL,
	reminder_enabled BOOLEAN NOT NULL,
	marked_as_participating_at TIMESTAMP WITH TIME ZONE NOT NULL,

	PRIMARY KEY(rsvp_sessions_message_id, user_id)
);
`}
