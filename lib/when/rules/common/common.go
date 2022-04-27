package common

import "github.com/botlabs-gg/yagpdb/lib/when/rules"

var All = []rules.Rule{
	SlashDMY(rules.Override),
}
