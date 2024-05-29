package notifications

var DBSchemas = []string{`
CREATE TABLE IF NOT EXISTS general_notification_configs (
	guild_id BIGINT PRIMARY KEY,
	created_at TIMESTAMP WITH TIME ZONE NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE NOT NULL,

	-- Many of the following columns should be non-nullable, but were originally
	-- managed by gorm (which does not add NOT NULL constraints by default) so are
	-- missing them. Unfortunately, it is unfeasible to retroactively fill missing
	-- values with defaults and add the constraints as there are simply too many
	-- rows in production.

	-- For similar legacy reasons, many fields that should have type BIGINT are TEXT.

	join_server_enabled BOOLEAN,
	join_server_channel TEXT,
	-- This column should be a TEXT[]. But for legacy reasons, it is instead a single
	-- TEXT column containing all template responses joined together and delimited by
	-- the character U+001E (INFORMATION SEPARATOR TWO.)
	join_server_msgs TEXT,
	join_dm_enabled BOOLEAN,
	join_dm_msg TEXT,

	leave_enabled BOOLEAN,
	leave_channel TEXT,
	-- Same deal as join_server_msgs.
	leave_msgs TEXT,
	
	topic_enabled BOOLEAN,
	topic_channel TEXT,

	censor_invites BOOLEAN
);
`, `

-- Tables created with gorm have missing NOT NULL constraints for created_at and
-- updated_at columns; since these columns are never null in existing rows, we can
-- retraoctively add the constraints without needing to update any data.

ALTER TABLE general_notification_configs ALTER COLUMN created_at SET NOT NULL;
`, `
ALTER TABLE general_notification_configs ALTER COLUMN updated_at SET NOT NULL;
`, `

-- Now the more complicated migration. For legacy reasons, the general_notification_configs
-- table previously contained two pairs of columns for join and leave message:
--   * join_server_msg, join_server_msgs_
--   * leave_msg, leave_msgs_
-- all of type TEXT. (The variants with _ were added when multiple-response support was
-- implemented and contain the individual responses separated by U+001E as described
-- previously.)

-- Ideally, we only have one column for each message. We achieve this state with the following
-- multi-step migration:
--   1. Update old records with join_server_msg != '' or leave_msg != '' to use the plural
--      join_server_msgs_ and leave_msgs_ columns respectively.
--   2. Drop the join_server_msg and leave_msg columns.
--   3. Rename join_server_msgs_ to join_server_msgs and leave_msgs_ to leave_msgs.

-- Here goes.
DO $$
BEGIN

-- only run if general_notifcation_configs.join_server_msg (indicative of legacy table) exists
IF EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='general_notification_configs' AND column_name='join_server_msg') THEN
	UPDATE general_notification_configs SET join_server_msgs_ = join_server_msg WHERE join_server_msg != '';
	UPDATE general_notification_configs SET leave_msgs_ = leave_msg WHERE leave_msg != '';

	ALTER TABLE general_notification_configs DROP COLUMN join_server_msg;
	ALTER TABLE general_notification_configs DROP COLUMN leave_msg;

	ALTER TABLE general_notification_configs RENAME COLUMN join_server_msgs_ to join_server_msgs;
	ALTER TABLE general_notification_configs RENAME COLUMN leave_msgs_ to leave_msgs;
END IF;
END $$;
`}
