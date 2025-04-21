package notes

var DBSchemas = []string{`
CREATE TABLE IF NOT EXISTS notes (
	guild_id BIGINT NOT NULL,
	member_id BIGINT NOT NULL,
	notes []TEXT NOT NULL,

	PRIMARY KEY(guild_id, member_id)
);
`}
