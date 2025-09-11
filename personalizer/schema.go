package personalizer

var DBSchemas = []string{`
CREATE TABLE IF NOT EXISTS personalized_guilds (
    guild_id BIGINT PRIMARY KEY,

    nick TEXT,
    avatar TEXT,
    banner TEXT,

    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);`}
