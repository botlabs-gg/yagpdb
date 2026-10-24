package automod

import (
	"context"
	"encoding/json"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/automod/models"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/featureflags"
	"github.com/botlabs-gg/yagpdb/v2/common/pubsub"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

const StarterRulesetName = "Starter Protection"

// StarterProtections picks the rules of the starter ruleset. Moderators are
// alerted in AlertChannel when it is set.
type StarterProtections struct {
	ServerInvites bool
	MassMentions  bool
	Spam          bool
	PhishingLinks bool

	AlertChannel int64
}

type starterRulePart struct {
	typeID   int
	settings any
}

type starterRule struct {
	name  string
	parts []starterRulePart
}

func (p StarterProtections) rules() []starterRule {
	var rules []starterRule

	add := func(enabled bool, name string, trigger starterRulePart, effects ...starterRulePart) {
		if !enabled {
			return
		}
		if p.AlertChannel != 0 {
			effects = append(effects, starterRulePart{315, SendModeratorAlertMessageData{LogChannel: p.AlertChannel}})
		}
		rules = append(rules, starterRule{name: name, parts: append([]starterRulePart{trigger}, effects...)})
	}

	deleteMessage := starterRulePart{300, nil}

	add(p.ServerInvites, "Block server invites", starterRulePart{9, nil},
		deleteMessage, starterRulePart{305, WarnUserEffectData{CustomReason: "Posted a server invite"}})
	add(p.MassMentions, "Block mass mentions", starterRulePart{2, MentionsTriggerData{Treshold: 5}},
		deleteMessage, starterRulePart{314, TimeoutUserEffectData{Duration: 10, CustomReason: "Mass mentions"}})
	add(p.Spam, "Stop message spam", starterRulePart{11, SlowmodeTriggerData{Treshold: 5, Interval: 5}},
		deleteMessage, starterRulePart{314, TimeoutUserEffectData{Duration: 10, CustomReason: "Spamming messages"}})
	add(p.PhishingLinks, "Block phishing links", starterRulePart{33, nil},
		deleteMessage, starterRulePart{305, WarnUserEffectData{CustomReason: "Posted a phishing link"}})

	return rules
}

// CreateStarterRuleset adds an enabled ruleset with the chosen protections,
// ignoring bots, which can then be edited like any other ruleset.
func CreateStarterRuleset(ctx context.Context, guildID int64, protections StarterProtections) error {
	rules := protections.rules()
	if len(rules) == 0 {
		return nil
	}

	rulesets, err := models.AutomodRulesets(qm.Where("guild_id = ?", guildID)).CountG(ctx)
	if err != nil {
		return err
	}
	if rulesets >= int64(GuildMaxRulesets(guildID)) {
		return errors.New("this server already has the most rulesets it can have")
	}

	totalRules, err := models.AutomodRules(qm.Where("guild_id = ?", guildID)).CountG(ctx)
	if err != nil {
		return err
	}
	if int(totalRules)+len(rules) > GuildMaxTotalRules(guildID) {
		return errors.New("this server does not have room for the starter rules")
	}

	tx, err := common.PQ.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	if err := insertStarterRuleset(ctx, tx, guildID, rules); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	pubsub.EvictCacheSet(cachedRulesets, guildID)
	featureflags.MarkGuildDirty(guildID)
	return nil
}

func insertStarterRuleset(ctx context.Context, exec boil.ContextExecutor, guildID int64, rules []starterRule) error {
	ruleset := &models.AutomodRuleset{GuildID: guildID, Name: StarterRulesetName, Enabled: true}
	if err := ruleset.Insert(ctx, exec, boil.Infer()); err != nil {
		return err
	}

	ignoreBots := &models.AutomodRulesetCondition{GuildID: guildID, RulesetID: ruleset.ID, Kind: int(RulePartCondition), TypeID: 209, Settings: []byte("{}")}
	if err := ignoreBots.Insert(ctx, exec, boil.Infer()); err != nil {
		return err
	}

	for _, rule := range rules {
		model := &models.AutomodRule{GuildID: guildID, RulesetID: ruleset.ID, Name: rule.name}
		if err := model.Insert(ctx, exec, boil.Infer()); err != nil {
			return err
		}

		for _, part := range rule.parts {
			settings := []byte("{}")
			if part.settings != nil {
				encoded, err := json.Marshal(part.settings)
				if err != nil {
					return err
				}
				settings = encoded
			}

			datum := &models.AutomodRuleDatum{
				GuildID:  guildID,
				RuleID:   model.ID,
				Kind:     int(RulePartMap[part.typeID].Kind()),
				TypeID:   part.typeID,
				Settings: settings,
			}
			if err := datum.Insert(ctx, exec, boil.Infer()); err != nil {
				return err
			}
		}
	}

	return nil
}
