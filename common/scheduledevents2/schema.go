package scheduledevents2

var DBSchemas = []string{`
CREATE TABLE IF NOT EXISTS scheduled_events (
	id BIGSERIAL PRIMARY KEY,

	created_at TIMESTAMP WITH TIME ZONE NOT NULL,
	triggers_at TIMESTAMP WITH TIME ZONE NOT NULL,

	retry_on_error BOOLEAN NOT NULL,

	guild_id BIGINT NOT NULL,
	event_name TEXT NOT NULL,
	data JSONB NOT NULL,

	processed BOOL not null
);
`, `
CREATE INDEX IF NOT EXISTS scheduled_events_triggers_at_idx ON scheduled_events(triggers_at);
`, `
ALTER TABLE scheduled_events ADD COLUMN IF NOT EXISTS  error TEXT
`,
}
