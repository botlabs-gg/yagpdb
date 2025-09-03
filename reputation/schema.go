package reputation

var DBSchemas = []string{`
CREATE TABLE IF NOT EXISTS reputation_configs (
	guild_id        bigint PRIMARY KEY,
	points_name     varchar(50) NOT NULL,
	enabled         bool NOT NULL,	
	cooldown        int NOT NULL,
	max_give_amount bigint NOT NULL,

	required_give_role       varchar(30),
	required_receive_role    varchar(30),
	blacklisted_give_role    varchar(30),
	blacklisted_receive_role varchar(30),
	admin_role               varchar(30)
);
`, `
ALTER TABLE reputation_configs ADD COLUMN IF NOT EXISTS disable_thanks_detection BOOLEAN NOT NULL DEFAULT false;
`, `
ALTER TABLE reputation_configs ADD COLUMN IF NOT EXISTS whitelisted_thanks_channels BIGINT[];
`, `
ALTER TABLE reputation_configs ADD COLUMN IF NOT EXISTS blacklisted_thanks_channels BIGINT[];
`, `
ALTER TABLE reputation_configs ADD COLUMN IF NOT EXISTS thanks_regex TEXT;
`, `
DO $$
BEGIN

-- add the 'max_remove_amount' column, which was added way after when the reputation system was made
-- to preserve backwards compatibility the initial value is the same as max_give_amount, so that requires some special code

IF (SELECT COUNT(*) FROM information_schema.columns WHERE table_name='reputation_configs' and column_name='max_remove_amount') < 1 THEN
    ALTER TABLE reputation_configs ADD COLUMN max_remove_amount BIGINT NOT NULL DEFAULT 0;
	UPDATE reputation_configs SET max_remove_amount=max_give_amount;
END IF;

-- from after 196658c24fc23770d8664d468a3d6e5733669279
-- we have all the role restrictions as BIGINT[]
-- the below converts from the old system to the new one

IF (SELECT COUNT(*) FROM information_schema.columns WHERE table_name='reputation_configs' and column_name='required_give_roles') < 1 THEN
    -- req give roles
    ALTER TABLE reputation_configs ADD COLUMN admin_roles BIGINT[];
	UPDATE reputation_configs SET admin_roles=ARRAY[admin_role]::BIGINT[] WHERE admin_role IS NOT NULL AND admin_role != '';

    -- req give roles
    ALTER TABLE reputation_configs ADD COLUMN required_give_roles BIGINT[];
	UPDATE reputation_configs SET required_give_roles=ARRAY[required_give_role]::BIGINT[] WHERE required_give_role IS NOT NULL AND required_give_role != '';

	-- req rec roles
	ALTER TABLE reputation_configs ADD COLUMN required_receive_roles BIGINT[];
	UPDATE reputation_configs SET required_receive_roles=ARRAY[required_receive_role]::BIGINT[] WHERE required_receive_role IS NOT NULL AND required_receive_role != '';

	-- blacklisted give roles
	ALTER TABLE reputation_configs ADD COLUMN blacklisted_give_roles BIGINT[];
	UPDATE reputation_configs SET blacklisted_give_roles=ARRAY[blacklisted_give_role]::BIGINT[] WHERE blacklisted_give_role IS NOT NULL AND blacklisted_give_role != '';
	
	-- blacklisted rec roles
	ALTER TABLE reputation_configs ADD COLUMN blacklisted_receive_roles BIGINT[];
	UPDATE reputation_configs SET blacklisted_receive_roles=ARRAY[blacklisted_receive_role]::BIGINT[] WHERE blacklisted_receive_role IS NOT NULL AND blacklisted_receive_role != '';

END IF;
END $$;
`, `
CREATE TABLE IF NOT EXISTS reputation_users (
	user_id  bigint NOT NULL,
	guild_id bigint NOT NULL,

	created_at  TIMESTAMP WITH TIME ZONE NOT NULL,
	points 		bigint NOT NULL,

	PRIMARY KEY(guild_id, user_id)
);
`, `
CREATE TABLE IF NOT EXISTS reputation_log (
	id bigserial 	 PRIMARY KEY,
	created_at	     TIMESTAMP WITH TIME ZONE NOT NULL,

	guild_id 		 bigint NOT NULL,
	sender_id 		 bigint NOT NULL,
	receiver_id 	 bigint NOT NULL,
	set_fixed_amount bool NOT NULL,
	amount 			 bigint NOT NULL
);
`, `
ALTER TABLE reputation_log ADD COLUMN IF NOT EXISTS receiver_username TEXT NOT NULL DEFAULT '';
`, `
ALTER TABLE reputation_log ADD COLUMN IF NOT EXISTS sender_username TEXT NOT NULL DEFAULT '';
`, `
CREATE INDEX IF NOT EXISTS reputation_log_guild_idx ON reputation_log (guild_id);
`, `
CREATE INDEX IF NOT EXISTS reputation_log_sender_idx ON reputation_log (sender_id);
`, `
CREATE INDEX IF NOT EXISTS reputation_log_receiver_idx ON reputation_log (receiver_id);	
`, `
CREATE TABLE IF NOT EXISTS reputation_roles (
	id bigserial PRIMARY KEY,
	guild_id bigint NOT NULL,
	rep_threshold bigint NOT NULL,
	role bigint NOT NULL
);
`, `
CREATE INDEX IF NOT EXISTS reputation_roles_guild_idx ON reputation_roles(guild_id);
`}
