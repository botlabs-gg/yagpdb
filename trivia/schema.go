package trivia

var DBSchemas = []string{`
CREATE TABLE IF NOT EXISTS trivia_users (
	user_id bigint NOT NULL,
	guild_id bigint NOT NULL,
	score int NOT NULL DEFAULT 0,
	correct_answers int NOT NULL DEFAULT 0,
	incorrect_answers int NOT NULL DEFAULT 0,
	current_streak int NOT NULL DEFAULT 0,
	max_streak int NOT NULL DEFAULT 0,
	last_played TIMESTAMP WITH TIME ZONE NOT NULL,

	PRIMARY KEY(guild_id, user_id)
);
`, `
CREATE INDEX IF NOT EXISTS trivia_users_guild_idx ON trivia_users(guild_id);
`}
