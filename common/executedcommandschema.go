package common

var ExecutedCommandDBSchemas = []string{`
CREATE TABLE IF NOT EXISTS executed_commands (
	id SERIAL PRIMARY KEY,
	created_at TIMESTAMP WITH TIME ZONE NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE NOT NULL,

	user_id TEXT NOT NULL, -- text not bigint for legacy compatibility
	channel_id TEXT NOT NULL,
	guild_id TEXT,

	command TEXT NOT NULL,
	raw_command TEXT NOT NULL,
	error TEXT,

	time_stamp TIMESTAMP WITH TIME ZONE NOT NULL,
	response_time BIGINT NOT NULL
);
`, `
-- Preexisting tables created prior to sqlboiler are missing non-null constraints,
-- so add them retraoctively.

ALTER TABLE executed_commands ALTER COLUMN created_at SET NOT NULL;
`, `
ALTER TABLE executed_commands ALTER COLUMN updated_at SET NOT NULL;
`, `
ALTER TABLE executed_commands ALTER COLUMN user_id SET NOT NULL;
`, `
ALTER TABLE executed_commands ALTER COLUMN channel_id SET NOT NULL;
`, `
ALTER TABLE executed_commands ALTER COLUMN command SET NOT NULL;
`, `
ALTER TABLE executed_commands ALTER COLUMN raw_command SET NOT NULL;
`, `
ALTER TABLE executed_commands ALTER COLUMN time_stamp SET NOT NULL;
`, `
ALTER TABLE executed_commands ALTER COLUMN response_time SET NOT NULL;
`}
