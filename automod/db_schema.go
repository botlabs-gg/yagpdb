package automod

var DBSchemas = []string{`
CREATE TABLE IF NOT EXISTS automod_rulesets (
	id BIGSERIAL PRIMARY KEY,
	guild_id BIGINT NOT NULL,

	name TEXT NOT NULL,
	enabled BOOLEAN NOT NULL
);
`, `
CREATE INDEX IF NOT EXISTS automod_rulesets_guild_idx ON automod_rulesets(guild_id);


`, `
CREATE TABLE IF NOT EXISTS automod_rules (
	id BIGSERIAL PRIMARY KEY,
	guild_id BIGINT NOT NULL,
	ruleset_id BIGINT references automod_rulesets(id) ON DELETE CASCADE NOT NULL,
	name TEXT NOT NULL,
	trigger_counter BIGINT NOT NULL
);

`, `
CREATE INDEX IF NOT EXISTS automod_rules_guild_idx ON automod_rules(guild_id);

`, `
CREATE INDEX IF NOT EXISTS automod_rules_ruleset_idx ON automod_rules(ruleset_id);

`, `
CREATE TABLE IF NOT EXISTS automod_rule_data (
	id BIGSERIAL PRIMARY KEY,
	guild_id BIGINT NOT NULL,
	rule_id BIGINT references automod_rules(id) ON DELETE CASCADE NOT NULL,

	kind int NOT NULL,
	type_id INT NOT NULL,
	settings JSONB NOT NULL
);

`, `
CREATE INDEX IF NOT EXISTS automod_rule_data_guild_idx ON automod_rule_data(guild_id);

`, `
CREATE TABLE IF NOT EXISTS automod_ruleset_conditions (
	id BIGSERIAL PRIMARY KEY,
	guild_id BIGINT NOT NULL,
	ruleset_id BIGINT references automod_rulesets(id) ON DELETE CASCADE NOT NULL,

	kind int NOT NULL,
	type_id INT NOT NULL,
	settings JSONB NOT NULL
);

`, `
CREATE INDEX IF NOT EXISTS automod_ruleset_conditions_guild_idx ON automod_ruleset_conditions(guild_id);

`, `
CREATE INDEX IF NOT EXISTS automod_ruleset_conditions_ruleset_idx ON automod_ruleset_conditions(ruleset_id);

`, `
CREATE TABLE IF NOT EXISTS automod_violations (
	id BIGSERIAL PRIMARY KEY,
	guild_id BIGINT NOT NULL,
	user_id BIGINT NOT NULL,
	rule_id BIGINT references automod_rules(id) ON DELETE SET NULL,

	created_at TIMESTAMP WITH TIME ZONE NOT NULL,

	name TEXT NOT NULL
);

`, `
CREATE INDEX IF NOT EXISTS automod_violations_guild_idx ON automod_violations(guild_id);
`, `
CREATE INDEX IF NOT EXISTS automod_violations_user_idx ON automod_violations(user_id);

`, `
CREATE TABLE IF NOT EXISTS automod_lists (
	id BIGSERIAL PRIMARY KEY,
	guild_id BIGINT NOT NULL,

	name TEXT NOT NULL,
	kind INT NOT NULL,
	content TEXT[] NOT NULL
);

`, `
CREATE INDEX IF NOT EXISTS automod_lists_guild_idx ON automod_lists(guild_id);

`, `
CREATE TABLE IF NOT EXISTS automod_triggered_rules (
	id BIGSERIAL PRIMARY KEY,
	created_at TIMESTAMP WITH TIME ZONE NOT NULL,
	channel_id BIGINT NOT NULL,
	channel_name TEXT NOT NULL,
	guild_id BIGINT NOT NULL,

	trigger_id BIGINT references automod_rule_data(id) ON DELETE SET NULL,
	trigger_typeid INT NOT NULL, -- backup in case the actual trigger was deleted
	
	rule_id BIGINT references automod_rules(id) ON DELETE SET NULL,
	rule_name TEXT NOT NULL, -- backup in case the rule was deleted
	ruleset_name TEXT NOT NULL,

	user_id BIGINT NOT NULL,
	user_name TEXT NOT NULL,

	extradata JSONB NOT NULL 
);
`, `

CREATE INDEX IF NOT EXISTS automod_triggered_rules_guild_idx ON automod_triggered_rules(guild_id);
`, `
CREATE INDEX IF NOT EXISTS automod_triggered_rules_rule_id_idx on automod_triggered_rules(rule_id);
`, `
CREATE INDEX IF NOT EXISTS automod_triggered_rules_trigger_idx ON automod_triggered_rules(trigger_id);
`}
