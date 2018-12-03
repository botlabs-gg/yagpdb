package bot

const DBSchema = `
CREATE TABLE IF NOT EXISTS joined_guilds (
	id BIGINT PRIMARY KEY,
	joined_at TIMESTAMP WITH TIME ZONE NOT NULL,
	left_at TIMESTAMP WITH TIME ZONE,
	member_count BIGINT NOT NULL,
	name TEXT NOT NULL,
	owner_id BIGINT NOT NULL,
	avatar TEXT NOT NULL
);
`
