package reputation

const DBSchema = `
-- DROP TABLE IF EXISTS reputation_configs;

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

ALTER TABLE reputation_configs ADD COLUMN IF NOT EXISTS disable_thanks_detection BOOLEAN NOT NULL DEFAULT false;

-- DROP TABLE IF EXISTS reputation_users;

CREATE TABLE IF NOT EXISTS reputation_users (
	user_id  bigint NOT NULL,
	guild_id bigint NOT NULL,

	created_at  TIMESTAMP WITH TIME ZONE NOT NULL,
	points 		bigint NOT NULL,

	PRIMARY KEY(guild_id, user_id)
);

-- DROP TABLE IF EXISTS reputation_log;

CREATE TABLE IF NOT EXISTS reputation_log (
	id bigserial 	 PRIMARY KEY,
	created_at	     TIMESTAMP WITH TIME ZONE NOT NULL,

	guild_id 		 bigint NOT NULL,
	sender_id 		 bigint NOT NULL,
	receiver_id 	 bigint NOT NULL,
	set_fixed_amount bool NOT NULL,
	amount 			 bigint NOT NULL
);

ALTER TABLE reputation_log ADD COLUMN IF NOT EXISTS receiver_username TEXT NOT NULL DEFAULT '';
ALTER TABLE reputation_log ADD COLUMN IF NOT EXISTS sender_username TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS reputation_log_guild_idx ON reputation_log (guild_id);
CREATE INDEX IF NOT EXISTS reputation_log_sender_idx ON reputation_log (sender_id);
CREATE INDEX IF NOT EXISTS reputation_log_receiver_idx ON reputation_log (receiver_id);	
`
