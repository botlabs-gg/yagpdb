package serverpreview

var DBSchemas = []string{`
CREATE TABLE IF NOT EXISTS server_previews (
	guild_id BIGINT PRIMARY KEY,
	enabled BOOLEAN NOT NULL DEFAULT FALSE,
	suspended BOOLEAN NOT NULL DEFAULT FALSE,

	invite_channel_id BIGINT NOT NULL DEFAULT 0,
	invite_code TEXT NOT NULL DEFAULT '',

	slug TEXT,
	accent_color INT NOT NULL DEFAULT 0,
	hide_footer BOOLEAN NOT NULL DEFAULT FALSE,
	custom_text TEXT NOT NULL DEFAULT '',
	buttons JSONB NOT NULL DEFAULT '[]',

	updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
`, `
CREATE UNIQUE INDEX IF NOT EXISTS server_previews_slug_idx ON server_previews (slug) WHERE slug IS NOT NULL;
`, `
ALTER TABLE server_previews DROP COLUMN IF EXISTS show_banner;
`, `
ALTER TABLE server_previews ADD COLUMN IF NOT EXISTS hide_member_count BOOLEAN NOT NULL DEFAULT FALSE;
`, `
ALTER TABLE server_previews ADD COLUMN IF NOT EXISTS hide_description BOOLEAN NOT NULL DEFAULT FALSE;
`, `
ALTER TABLE server_previews ADD COLUMN IF NOT EXISTS hide_banner BOOLEAN NOT NULL DEFAULT FALSE;
`}
