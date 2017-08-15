BEGIN;

CREATE TABLE IF NOT EXISTS role_groups (
	id serial NOT NULL PRIMARY KEY,
	guild_id bigint NOT NULL,
	name text NOT NULL,
	require_roles bigint[],
	ignore_roles bigint[],
	mode bigint NOT NULL,
	multiple_max bigint NOT NULL,
	multiple_min bigint NOT NULL,
	single_auto_toggle_off boolean NOT NULL,
	single_require_one boolean NOT NULL
);


CREATE TABLE IF NOT EXISTS role_commands (
	id serial NOT NULL PRIMARY KEY,
	guild_id bigint NOT NULL,
	name text NOT NULL,
	role_group_id bigint REFERENCES role_groups(id),
	role bigint NOT NULL,
	require_roles bigint[],
	ignore_roles bigint[]
);

CREATE INDEX IF NOT EXISTS role_commands_gidx ON role_commands(guild_id);
CREATE INDEX IF NOT EXISTS role_groups_gidx ON role_groups(guild_id);

COMMIT;
