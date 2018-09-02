package premium

const DBSchema = `
CREATE TABLE IF NOT EXISTS premium_codes (
	id BIGSERIAL PRIMARY KEY,

	code TEXT UNIQUE NOT NULL,
	message TEXT NOT NULL,

	created_at TIMESTAMP WITH TIME ZONE NOT NULL,
	used_at TIMESTAMP WITH TIME ZONE,
	attached_at TIMESTAMP WITH TIME ZONE,

	user_id BIGINT,
	guild_id BIGINT,

	permanent BOOLEAN NOT NULL,
	full_duration BIGINT NOT NULL,
	duration_used BIGINT NOT NULL -- duration_used is updated after its detached, so to get the full duration used also include the time passed since attached_at (if its attached to a guild)
);

CREATE INDEX IF NOT EXISTS premium_codes_code_idx ON premium_codes(code); 
`
