package voiceroles

var DBSchemas = []string{`
CREATE TABLE IF NOT EXISTS voice_roles (
	id bigserial NOT NULL PRIMARY KEY,
	created_at timestamptz NOT NULL,
	updated_at timestamptz NOT NULL,
	guild_id bigint NOT NULL,
	channel_id bigint NOT NULL,
	role_id bigint NOT NULL,
	enabled boolean NOT NULL DEFAULT true,
);
`, `
CREATE UNIQUE INDEX IF NOT EXISTS voice_roles_channel_idx ON voice_roles(channel_id);
`, `
CREATE INDEX IF NOT EXISTS voice_roles_guild_idx ON voice_roles(guild_id);
`}
