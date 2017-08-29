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
	created_at timestamptz NOT NULL,
	updated_at timestamptz NOT NULL,
	guild_id bigint NOT NULL,
	name text NOT NULL,
	role_group_id bigint REFERENCES role_groups(id) ON DELETE SET NULL,
	role bigint NOT NULL,
	require_roles bigint[],
	ignore_roles bigint[],
	position bigint NOT NULL
);


COMMIT;
