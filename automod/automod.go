package automod

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"unicode"

	"emperror.dev/errors"
	"github.com/jonas747/yagpdb/automod/models"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/featureflags"
	"github.com/jonas747/yagpdb/premium"
	"github.com/karlseguin/ccache"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

//go:generate sqlboiler --no-hooks psql

var (
	RegexCache *ccache.Cache
	logger     = common.GetPluginLogger(&Plugin{})
)

type Plugin struct {
}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Automod v2",
		SysName:  "automod_v2",
		Category: common.PluginCategoryModeration,
	}
}

func RegisterPlugin() {
	RegexCache = ccache.New(ccache.Configure())

	common.InitSchemas("automod_v2", DBSchemas...)

	p := &Plugin{}
	common.RegisterPlugin(p)
}

type ErrUnknownTypeID struct {
	TypeID int
}

func (e *ErrUnknownTypeID) Error() string {
	return "Unknown TypeID: " + strconv.Itoa(e.TypeID)
}

func ParseRulePartData(model *models.AutomodRuleDatum) (interface{}, error) {
	part, ok := RulePartMap[model.TypeID]
	if !ok {
		return nil, &ErrUnknownTypeID{model.TypeID}
	}

	settingsDestination := part.DataType()
	if settingsDestination == nil {
		// No user settings for this part
		return nil, nil
	}

	err := json.Unmarshal(model.Settings, settingsDestination)
	return settingsDestination, err
}

func ParseAllRulePartData(dataModels []*models.AutomodRuleDatum) ([]interface{}, error) {
	dst := make([]interface{}, len(dataModels))
	for i, v := range dataModels {
		parsed, err := ParseRulePartData(v)
		if err != nil {
			return nil, err
		}

		dst[i] = parsed
	}

	return dst, nil
}

const (
	MaxMessageTriggers        = 20
	MaxMessageTriggersPremium = 100

	MaxViolationTriggers        = 20
	MaxViolationTriggersPremium = 100

	MaxTotalRules        = 25
	MaxTotalRulesPremium = 150

	MaxLists        = 5
	MaxListsPremium = 25

	MaxRuleParts = 25

	MaxRulesets        = 10
	MaxRulesetsPremium = 25
)

func GuildMaxMessageTriggers(guildID int64) int {
	if isPremium, _ := premium.IsGuildPremium(guildID); isPremium {
		return MaxMessageTriggersPremium
	}

	return MaxMessageTriggers
}

func GuildMaxViolationTriggers(guildID int64) int {
	if isPremium, _ := premium.IsGuildPremium(guildID); isPremium {
		return MaxViolationTriggersPremium
	}

	return MaxViolationTriggers
}

func GuildMaxTotalRules(guildID int64) int {
	if isPremium, _ := premium.IsGuildPremium(guildID); isPremium {
		return MaxTotalRulesPremium
	}

	return MaxTotalRules
}

func GuildMaxLists(guildID int64) int {
	if isPremium, _ := premium.IsGuildPremium(guildID); isPremium {
		return MaxListsPremium
	}

	return MaxLists
}

func GuildMaxRulesets(guildID int64) int {
	if isPremium, _ := premium.IsGuildPremium(guildID); isPremium {
		return MaxRulesetsPremium
	}

	return MaxRulesets
}

func PrepareMessageForWordCheck(input string) string {
	var out strings.Builder

	split := strings.Fields(input)
	for i, w := range split {
		if i != 0 {
			out.WriteRune(' ')
		}

		// make 2 variants, 1 with all occurences replaced with space and 1 with all the occurences just removed
		// this i imagine will solve a low of cases
		w1 := ""
		w2 := ""

		for _, r := range w {
			// we replace them with spaces instead to make for a more accurate version
			// e.g "word1*word2" will become "word1 word2" instead of "word1word2"
			if unicode.IsPunct(r) || unicode.IsSymbol(r) {
				// replace with spaces for w1, and just remove for w2
				w1 += " "
			} else {
				w1 += string(r)
				w2 += string(r)
			}
		}

		out.WriteString(w1)
		if w1 != w2 && w1 != w {
			out.WriteString(" " + w2 + " " + w)
		} else if w1 != w2 {
			out.WriteString(" " + w2)
		} else if w1 != w {
			out.WriteString(" " + w)
		}
	}

	return out.String()
}

var _ featureflags.PluginWithFeatureFlags = (*Plugin)(nil)

const (
	featureFlagEnabled = "automod_v2_enabled"
)

func (p *Plugin) UpdateFeatureFlags(guildID int64) ([]string, error) {
	rulesets, err := models.AutomodRulesets(qm.Where("guild_id=?", guildID),
		qm.Load("RulesetAutomodRules.RuleAutomodRuleData"), qm.Load("RulesetAutomodRulesetConditions")).AllG(context.Background())
	if err != nil {
		return nil, errors.WithStackIf(err)
	}

	var flags []string
	for _, v := range rulesets {
		if !v.Enabled {
			continue
		}

		if len(v.R.RulesetAutomodRules) > 0 {
			// If theres a ruleset with atleast 1 rule, consider the plugin enabled
			flags = append(flags, featureFlagEnabled)
			break
		}
	}

	return flags, nil
}

func (p *Plugin) AllFeatureFlags() []string {
	return []string{
		featureFlagEnabled, // set if there is atleast one ruleset enabled with a rule in it
	}
}
