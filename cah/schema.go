package cah

var DBSchemas = []string{`
CREATE TABLE IF NOT EXISTS cah_guild_configs (
	guild_id BIGINT PRIMARY KEY,

	default_packs TEXT[] NOT NULL DEFAULT '{main}',
	disabled_packs TEXT[] NOT NULL DEFAULT '{}',

	vote_mode BOOLEAN NOT NULL DEFAULT false,
	player_limit INT NOT NULL DEFAULT 10,
	win_limit INT NOT NULL DEFAULT 10,
	pick_seconds INT NOT NULL DEFAULT 60,
	judge_seconds INT NOT NULL DEFAULT 90
);
`, `
CREATE TABLE IF NOT EXISTS cah_custom_packs (
	id BIGSERIAL PRIMARY KEY,
	created_at TIMESTAMP WITH TIME ZONE NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE NOT NULL,

	guild_id BIGINT NOT NULL,
	name TEXT NOT NULL,
	description TEXT NOT NULL,

	prompts TEXT[] NOT NULL,
	responses TEXT[] NOT NULL
);
`, `
CREATE INDEX IF NOT EXISTS cah_custom_packs_guild_idx ON cah_custom_packs(guild_id);
`, `
CREATE UNIQUE INDEX IF NOT EXISTS cah_custom_packs_guild_name_idx ON cah_custom_packs(guild_id, lower(name));
`}
