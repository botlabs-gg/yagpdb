package youtube

var DBSchemas = []string{`
CREATE TABLE IF NOT EXISTS youtube_channel_subscriptions (
	id SERIAL PRIMARY KEY,
	created_at TIMESTAMP WITH TIME ZONE NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE NOT NULL,

	guild_id TEXT NOT NULL,
	channel_id TEXT NOT NULL,
	youtube_channel_id TEXT NOT NULL,
	youtube_channel_name TEXT NOT NULL,

	mention_everyone BOOLEAN NOT NULL,
	mention_roles BIGINT[],
	publish_livestream BOOLEAN NOT NULL DEFAULT TRUE,
	publish_shorts BOOLEAN NOT NULL DEFAULT TRUE,
	enabled BOOLEAN NOT NULL DEFAULT TRUE
);
`, `

-- Old tables managed with gorm are missing NOT NULL constraints on some columns
-- that are never null in existing records; add them as needed.

ALTER TABLE youtube_channel_subscriptions ALTER COLUMN created_at SET NOT NULL;
`, `
ALTER TABLE youtube_channel_subscriptions ALTER COLUMN updated_at SET NOT NULL;
`, `
ALTER TABLE youtube_channel_subscriptions ALTER COLUMN guild_id SET NOT NULL;
`, `
ALTER TABLE youtube_channel_subscriptions ALTER COLUMN channel_id SET NOT NULL;
`, `
ALTER TABLE youtube_channel_subscriptions ALTER COLUMN youtube_channel_id SET NOT NULL;
`, `
ALTER TABLE youtube_channel_subscriptions ALTER COLUMN youtube_channel_name SET NOT NULL;
`, `
ALTER TABLE youtube_channel_subscriptions ALTER COLUMN mention_everyone SET NOT NULL;

-- Can't add a NOT NULL constraint to mention_roles because too many records in
-- production have it set to null.
`, `

-- The migration for the publish_livestream, publish_shorts, and enabled columns is
-- more involved. These columns were added later and so it is possible that they are
-- null in some older records. Therefore, we first replace these missing values with
-- defaults before adding the NOT NULL constraint.

DO $$
BEGIN

-- only run if we haven't added the NOT NULL constraint yet
IF EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='youtube_channel_subscriptions' AND column_name='publish_livestream' AND is_nullable='yes') THEN
	UPDATE youtube_channel_subscriptions SET publish_livestream = TRUE WHERE publish_livestream IS NULL;
	UPDATE youtube_channel_subscriptions SET publish_shorts = TRUE WHERE publish_shorts IS NULL;
	UPDATE youtube_channel_subscriptions SET enabled = TRUE WHERE enabled IS NULL;

	ALTER TABLE youtube_channel_subscriptions ALTER COLUMN publish_livestream SET NOT NULL;
	ALTER TABLE youtube_channel_subscriptions ALTER COLUMN publish_shorts SET NOT NULL;
	ALTER TABLE youtube_channel_subscriptions ALTER COLUMN enabled SET NOT NULL;
END IF;
END $$;
`, `

CREATE TABLE IF NOT EXISTS youtube_announcements (
	guild_id BIGINT PRIMARY KEY,
	message TEXT NOT NULL,
	enabled BOOLEAN NOT NULL DEFAULT FALSE
);
`, `

ALTER TABLE youtube_announcements ALTER COLUMN guild_id SET NOT NULL;
`, `
ALTER TABLE youtube_announcements ALTER COLUMN message SET NOT NULL;
`, `
ALTER TABLE youtube_announcements ALTER COLUMN enabled SET NOT NULL;
`}
