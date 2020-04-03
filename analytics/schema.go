package analytics

var dbSchemas = []string{`CREATE TABLE IF NOT EXISTS analytics (
	id BIGSERIAL PRIMARY KEY,
	guild_id BIGINT NOT NULL,

	created_at TIMESTAMP WITH TIME ZONE NOT NULL,

	plugin TEXT NOT NULL,
	name TEXT NOT NULL,
	count INT NOT NULL
)`,
	`
CREATE INDEX IF NOT EXISTS idx_analytics_created_at ON analytics(created_at);
`,
}
