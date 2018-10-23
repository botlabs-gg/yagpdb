package scheduledevents2

const DBSchema = `
CREATE TABLE IF NOT EXISTS scheduled_events (
	id BIGINT PRIMARY KEY,

	created_at TIMESTAMP WITH TIME ZONE NOT NULL,
	triggers_at TIMESTAMP WITH TIME ZONE NOT NULL,

	retry_on_error BOOLEAN NOT NULL,

	guild_id BIGINT NOT NULL,
	event_name TEXT NOT NULL,
	data JSONB NOT NULL
);

CREATE INDEX IF NOT EXISTS scheduled_events_triggers_at_idx ON scheduled_events(triggers_at);
`
