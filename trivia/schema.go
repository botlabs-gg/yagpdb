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
CREATE INDEX IF NOT EXISTS trivia_users_last_played_idx ON trivia_users(last_played);
`, `
CREATE INDEX IF NOT EXISTS trivia_users_guild_idx ON trivia_users(guild_id);
`, `
CREATE INDEX IF NOT EXISTS trivia_users_score_idx ON trivia_users(score);
`, `
CREATE INDEX IF NOT EXISTS trivia_users_correct_answers_idx ON trivia_users(correct_answers);
`, `
CREATE INDEX IF NOT EXISTS trivia_users_incorrect_answers_idx ON trivia_users(incorrect_answers);
`, `
CREATE INDEX IF NOT EXISTS trivia_users_current_streak_idx ON trivia_users(current_streak);
`, `
CREATE INDEX IF NOT EXISTS trivia_users_max_streak_idx ON trivia_users(max_streak);
`}
