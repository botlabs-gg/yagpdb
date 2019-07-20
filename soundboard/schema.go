package soundboard

var DBSchemas = []string{`
CREATE TABLE IF NOT EXISTS soundboard_sounds(
	id SERIAL PRIMARY KEY,
	created_at TIMESTAMP WITH TIME ZONE NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
	
	guild_id BIGINT NOT NULL,
	required_role TEXT NOT NULL,
	name TEXT NOT NULL,
	status INT NOT NULL,

	required_roles BIGINT[],
	blacklisted_roles BIGINT[]
);
`, `
CREATE INDEX IF NOT EXISTS soundboard_sounds_guild_idx ON soundboard_sounds(guild_id);
`, `

-- i was using gorm way back, and that apperently didn't add not null constraints
-- so for existing tables make sure they're present

-- this SHOULD be impossible, but just to sure... (was 0 rows on prod db)
DELETE FROM soundboard_sounds WHERE guild_id IS NULL or required_role IS NULL or name IS NULL or status IS NULL or created_at IS NULL or updated_at IS NULL;
`, `

ALTER TABLE soundboard_sounds ALTER COLUMN guild_id SET NOT NULL;
`, `
ALTER TABLE soundboard_sounds ALTER COLUMN required_role SET NOT NULL;
`, `
ALTER TABLE soundboard_sounds ALTER COLUMN name SET NOT NULL;
`, `
ALTER TABLE soundboard_sounds ALTER COLUMN status SET NOT NULL;
`, `
ALTER TABLE soundboard_sounds ALTER COLUMN created_at SET NOT NULL;
`, `
ALTER TABLE soundboard_sounds ALTER COLUMN updated_at SET NOT NULL;

`, `
-- we migrate the data from the old system so that peoples settings dont dissapear
DO $$
BEGIN
	IF (SELECT COUNT(*) FROM information_schema.columns WHERE table_name='soundboard_sounds' and column_name='required_roles') < 1 THEN
		ALTER TABLE soundboard_sounds ADD COLUMN required_roles BIGINT[];
		ALTER TABLE soundboard_sounds ADD COLUMN blacklisted_roles BIGINT[];

		-- migrate
		UPDATE soundboard_sounds SET required_roles=ARRAY[required_role]::BIGINT[] WHERE required_role IS NOT NULL AND required_role != '';
	END IF;
END $$;
`}
