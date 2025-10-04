package premium

var DBSchemas = []string{`
CREATE TABLE IF NOT EXISTS premium_slots (
	id BIGSERIAL PRIMARY KEY,

	created_at TIMESTAMP WITH TIME ZONE NOT NULL,
	attached_at TIMESTAMP WITH TIME ZONE,

	user_id BIGINT NOT NULL,
	guild_id BIGINT,

	title TEXT NOT NULL,
	message TEXT NOT NULL,
	source TEXT NOT NULL,
	source_id BIGINT NOT NULL,

	full_duration BIGINT NOT NULL,
	permanent BOOLEAN NOT NULL,
	duration_remaining BIGINT NOT NULL
); 
`, `
ALTER TABLE premium_slots ADD COLUMN IF NOT EXISTS tier INT NOT NULL DEFAULT 1;
`, `
ALTER TABLE premium_slots ADD COLUMN IF NOT EXISTS deletes_at TIMESTAMP WITH TIME ZONE;
`, `
CREATE UNIQUE INDEX IF NOT EXISTS premium_slots_guild_unique
ON premium_slots(guild_id)
WHERE guild_id IS NOT NULL;
`, `
CREATE TABLE IF NOT EXISTS premium_codes (
	id BIGSERIAL PRIMARY KEY,

	code TEXT UNIQUE NOT NULL,
	message TEXT NOT NULL,

	created_at TIMESTAMP WITH TIME ZONE NOT NULL,
	used_at TIMESTAMP WITH TIME ZONE,
	slot_id BIGINT references premium_slots(id),

	user_id BIGINT,
	guild_id BIGINT,

	permanent BOOLEAN NOT NULL,
	duration BIGINT NOT NULL
);
`, `
CREATE INDEX IF NOT EXISTS premium_codes_code_idx ON premium_codes(code);
`, `
CREATE TABLE IF NOT EXISTS discord_skus ( sku_id BIGINT PRIMARY KEY, slots INTEGER NOT NULL);
`,
	`
CREATE TABLE IF NOT EXISTS patreon_tiers ( tier_id BIGINT PRIMARY KEY, slots INTEGER NOT NULL);
`,
}
