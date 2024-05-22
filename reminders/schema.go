package reminders

var DBSchemas = []string{`
CREATE TABLE IF NOT EXISTS reminders (
	id SERIAL PRIMARY KEY,
	created_at TIMESTAMP WITH TIME ZONE NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
	deleted_at TIMESTAMP WITH TIME ZONE,

	-- text instead of bigint for legacy compatibility
	user_id TEXT NOT NULL,
	channel_id TEXT NOT NULL,
	guild_id BIGINT NOT NULL,
	message TEXT NOT NULL,
	"when" BIGINT NOT NULL
);
`, `
CREATE INDEX IF NOT EXISTS idx_reminders_deleted_at ON reminders(deleted_at);
`, `
-- Previous versions of the reputation module used gorm instead of sqlboiler,
-- which does not add NOT NULL constraints by default. Therefore, ensure these constraints
-- are present in existing tables as well.

ALTER TABLE reminders ALTER COLUMN created_at SET NOT NULL;
`, `
ALTER TABLE reminders ALTER COLUMN updated_at SET NOT NULL;
`, `
ALTER TABLE reminders ALTER COLUMN user_id SET NOT NULL;
`, `
ALTER TABLE reminders ALTER COLUMN channel_id SET NOT NULL;
`, `
ALTER TABLE reminders ALTER COLUMN guild_id SET NOT NULL;
`, `
ALTER TABLE reminders ALTER COLUMN message SET NOT NULL;
`, `
ALTER TABLE reminders ALTER COLUMN "when" SET NOT NULL;
`}

/*
TBD: Should we execute the following query automatically (by including it in
DBSchemas above) before adding NOT NULL constraints to relevant columns?

Jonas includes a similar query when migrating soundboard away from gorm:
https://github.com/botlabs-gg/yagpdb/commit/628ea9a228ab11dcc327ad0d017c5654312025af
but reminders are much more widely used, and running this query on every startup (requiring
a full table scan) is potentially costly. We could instead tell self-hosters to manually
run this query before updating to the new version.

-- Delete invalid data (should not exist, but just to be sure.)
DELETE FROM reminders
WHERE created_at IS NULL
	OR updated_at IS NULL
	OR user_id IS NULL
	OR channel_id IS NULL
	OR guild_id IS NULL
	OR message IS NULL
	OR "when" IS NULL;
*/
