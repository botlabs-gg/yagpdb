package rolecommands

var DBSchemas = []string{`
CREATE TABLE IF NOT EXISTS role_groups (
	id bigserial NOT NULL PRIMARY KEY,
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

`, `
CREATE INDEX IF NOT EXISTS role_groups_guild_idx ON role_groups(guild_id);

`, `
CREATE TABLE IF NOT EXISTS role_commands (
	id bigserial NOT NULL PRIMARY KEY,
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

`, `
CREATE INDEX IF NOT EXISTS role_commands_guild_idx ON role_commands(guild_id);
`, `
CREATE INDEX IF NOT EXISTS role_commands_role_group_idx ON role_commands(role_group_id);
`, `
CREATE TABLE IF NOT EXISTS role_menus (
 	message_id bigint NOT NULL PRIMARY KEY,
	guild_id bigint NOT NULL,
	channel_id bigint NOT NULL,
	owner_id bigint NOT NULL,
	own_message boolean NOT NULL,
	state bigint NOT NULL,
	next_role_command_id bigint REFERENCES role_commands(id) ON DELETE SET NULL,
	role_group_id bigint REFERENCES role_groups(id) ON DELETE CASCADE
);

`, `
ALTER TABLE role_menus ADD COLUMN IF NOT EXISTS disable_send_dm BOOLEAN NOT NULL DEFAULT false;
`, `
ALTER TABLE role_menus ADD COLUMN IF NOT EXISTS remove_role_on_reaction_remove BOOLEAN NOT NULL DEFAULT false;
`, `
ALTER TABLE role_menus ADD COLUMN IF NOT EXISTS fixed_amount BOOLEAN NOT NULL DEFAULT false;
`, `
ALTER TABLE role_menus ADD COLUMN IF NOT EXISTS skip_amount INT NOT NULL DEFAULT 0;
`, `
ALTER TABLE role_menus ADD COLUMN IF NOT EXISTS setup_msg_id BIGINT NOT NULL DEFAULT 0;
`, `
ALTER TABLE role_menus ADD COLUMN IF NOT EXISTS standalone_mode SMALLINT;
`, `
ALTER TABLE role_menus ADD COLUMN IF NOT EXISTS standalone_multiple_min INT;
`, `
ALTER TABLE role_menus ADD COLUMN IF NOT EXISTS standalone_multiple_max INT;
`, `
ALTER TABLE role_menus ADD COLUMN IF NOT EXISTS standalone_single_auto_toggle_off BOOLEAN;
`, `
ALTER TABLE role_menus ADD COLUMN IF NOT EXISTS standalone_single_require_one BOOLEAN;
`, `
ALTER TABLE role_menus ADD COLUMN IF NOT EXISTS standalone_blacklist_roles BIGINT[];
`, `
ALTER TABLE role_menus ADD COLUMN IF NOT EXISTS standalone_whitelist_roles BIGINT[];
`, `
ALTER TABLE role_menus ADD COLUMN IF NOT EXISTS saved_content TEXT;
`, `
ALTER TABLE role_menus ADD COLUMN IF NOT EXISTS saved_embed TEXT;
`, `
ALTER TABLE role_menus ADD COLUMN IF NOT EXISTS kind SMALLINT NOT NULL DEFAULT 0;
`, `
ALTER TABLE role_menus ADD COLUMN IF NOT EXISTS editing_option_id BIGINT;
`, `
CREATE INDEX IF NOT EXISTS role_menus_setup_msg_idx ON role_menus(setup_msg_id);
`, `
CREATE TABLE IF NOT EXISTS role_menu_options (
	id bigserial NOT NULL PRIMARY KEY,
	role_command_id bigint REFERENCES role_commands(id) ON DELETE CASCADE,
	emoji_id bigint NOT NULL,
	unicode_emoji text NOT NULL,
	role_menu_id bigint NOT NULL REFERENCES role_menus(message_id) ON DELETE CASCADE
);
`, `
ALTER TABLE role_menu_options ADD COLUMN IF NOT EXISTS standalone_role_id BIGINT;
`, `
ALTER TABLE role_menu_options ADD COLUMN IF NOT EXISTS blacklist_roles BIGINT[];
`, `
ALTER TABLE role_menu_options ADD COLUMN IF NOT EXISTS whitelist_roles BIGINT[];
`, `
DO $$
BEGIN

  BEGIN
    ALTER TABLE role_menus ADD CONSTRAINT role_menus_editing_option_id_fkey FOREIGN KEY (editing_option_id) REFERENCES role_menu_options(id) ON DELETE SET NULL;
  EXCEPTION
    WHEN duplicate_object THEN RAISE NOTICE 'Table constraint role_menus.role_menus_editing_option_id_fkey already exists';
  END;

END $$;
`, `
ALTER TABLE role_menu_options ADD COLUMN IF NOT EXISTS emoji_animated BOOLEAN NOT NULL DEFAULT false;
`, `
CREATE INDEX IF NOT EXISTS role_menu_options_role_command_idx ON role_menu_options(role_command_id);
`, `
CREATE INDEX IF NOT EXISTS role_menu_options_role_menu_id_idx ON role_menu_options(role_menu_id);
`, `
ALTER TABLE role_groups ADD COLUMN IF NOT EXISTS temporary_role_duration INT NOT NULL DEFAULT 0;
`}
