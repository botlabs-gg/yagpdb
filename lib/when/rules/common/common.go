package common

import "github.com/botlabs-gg/yagpdb/v2/lib/when/rules"

var All = []rules.Rule{
	SlashDMY(rules.Override),
}
