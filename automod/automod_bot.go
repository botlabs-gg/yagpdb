package automod

import (
	"context"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/automod/models"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

func (p *Plugin) BotInit() {
	eventsystem.AddHandler(p.handleMessageCreate, eventsystem.EventMessageCreate)
}

func (p *Plugin) handleMessageCreate(evt *eventsystem.EventData) {
	m := evt.MessageCreate()
	messageTriggerModels, err := models.AutomodRuleData(qm.Where("guild_id = ?", m.GuildID), qm.AndIn("type_id in ?", messageTriggers...), qm.Load("Rule.Ruleset.RulesetAutomodRulesetConditions"), qm.Load("Rule.RuleAutomodRuleData")).AllG(evt.Context())
	if err != nil {
		logrus.WithError(err).Error("automod failed retrieving message triggers")
		return
	}

	if len(messageTriggerModels) < 1 {
		return
	}

	ms, err := bot.GetMember(m.GuildID, m.Author.ID)
	cs := bot.State.Channel(true, m.ChannelID)
	if err != nil || cs == nil {
		return
	}

	parsedSettings, err := ParseAllRulePartData(messageTriggerModels)
	if err != nil {
		logrus.WithError(err).Error("automod failed parsing message trigger data")
		return
	}

	var triggeredRules []*models.AutomodRule
OUTER_CHECK_TRIGGERS:
	for i, triggerModel := range messageTriggerModels {
		for _, v := range triggeredRules {
			if v.ID == triggerModel.RuleID {
				// Already triggered this rule
				continue OUTER_CHECK_TRIGGERS
			}
		}

		trigger := RulePartMap[triggerModel.TypeID].(MessageTrigger)

		matched, err := trigger.CheckMessage(ms, cs, m.Message, parsedSettings[i])
		if err != nil {
			logrus.WithError(err).WithField("part_id", triggerModel.ID).Error("failed checking trigger")
			continue
		}

		if matched {
			triggeredRules = append(triggeredRules, triggerModel.R.Rule)
		}
	}

	if len(triggeredRules) < 1 {
		return
	}

	ctxData := &TriggeredRuleData{
		MS:     ms,
		CS:     cs,
		GS:     cs.Guild,
		Plugin: p,

		Message: m.Message,
	}

	p.RuleTriggersMatched(triggeredRules, ctxData)
	logrus.Println("Triggered rules: ", triggeredRules, m.GuildID)
}

func (p *Plugin) checkViolationTriggers(gs *dstate.GuildState, ms *dstate.MemberState, violationName string) {
	triggerModels, err := models.AutomodRuleData(qm.Where("guild_id = ?", gs.ID), qm.AndIn("type_id in ?", violationTriggers...), qm.Load("Rule.Ruleset.RulesetAutomodRulesetConditions"), qm.Load("Rule.RuleAutomodRuleData")).AllG(context.Background())
	if err != nil {
		logrus.WithError(err).Error("automod failed retrieving violation triggers")
		return
	}

	if len(triggerModels) < 1 {
		return
	}

	parsedSettings, err := ParseAllRulePartData(triggerModels)
	if err != nil {
		logrus.WithError(err).Error("automod failed parsing violation trigger data")
		return
	}

	userViolations, err := models.AutomodViolations(qm.Where("guild_id = ? AND user_id = ? AND name = ?", gs.ID, ms.ID, violationName)).AllG(context.Background())
	if err != nil {
		logrus.WithError(err).Error("automod failed retrieving user violations")
		return
	}

	var triggeredRules []*models.AutomodRule
OUTER_CHECK_TRIGGERS:
	for i, triggerModel := range triggerModels {
		for _, v := range triggeredRules {
			if v.ID == triggerModel.RuleID {
				// Already triggered this rule
				continue OUTER_CHECK_TRIGGERS
			}
		}

		trigger := RulePartMap[triggerModel.TypeID].(ViolationListener)

		matched, err := trigger.CheckUser(ms, gs, userViolations, parsedSettings[i])
		if err != nil {
			logrus.WithError(err).WithField("part_id", triggerModel.ID).Error("failed checking violations trigger")
			continue
		}

		if matched {
			triggeredRules = append(triggeredRules, triggerModel.R.Rule)
		}
	}

	if len(triggeredRules) < 1 {
		return
	}

	ctxData := &TriggeredRuleData{
		MS:     ms,
		GS:     gs,
		Plugin: p,
	}

	logrus.Println("Triggered violation rules: ", triggeredRules, gs.ID)
	p.RuleTriggersMatched(triggeredRules, ctxData)
}

func (p *Plugin) RuleTriggersMatched(rules []*models.AutomodRule, ctxData *TriggeredRuleData) {
	// parse the rulesets
	var handledRulesets []int64

	// parse the relevant rulesets and handle the relevant rules in batches by ruleset
	for _, r := range rules {
		if common.ContainsInt64Slice(handledRulesets, r.RulesetID) {
			continue
		}

		handledRulesets = append(handledRulesets, r.RulesetID)

		parsedRS, err := ParseRuleset(r.R.Ruleset)
		if err != nil {
			logrus.WithError(err).Error("failed parsing ruleset")
			continue
		}

		commonRulesetRules := make([]*ParsedRule, 0, 1)
		for _, parsedRule := range parsedRS.Rules {
			for _, sr := range rules {
				if sr.ID == parsedRule.Model.ID {
					commonRulesetRules = append(commonRulesetRules, parsedRule)
					break
				}
			}
		}

		go p.RulesetRulesTriggered(parsedRS, commonRulesetRules, ctxData)

	}
}

func (p *Plugin) RulesetRulesTriggered(ruleset *ParsedRuleset, triggeredRules []*ParsedRule, ctxData *TriggeredRuleData) {
	ctxDataCop := *ctxData
	ctxDataCop.Ruleset = ruleset
	ctxData = &ctxDataCop

	// check if we match all conditions, starting with the ruleset conditions
	for _, cond := range ruleset.ParsedConditions {
		met, err := cond.Part.(Condition).IsMet(ctxData, cond.ParsedSettings)
		if err != nil {
			logrus.WithError(err).WithField("guild", ctxData.GS.ID).Error("failed checking if automod ruleset condition was met")
			return // assume the condition failed
		}

		if !met {
			return // condition was not met
		}
	}

	filteredRules := make([]*ParsedRule, 0, len(triggeredRules))

	// Check the rule specific conditins
OUTE_RULE_LOOP:
	for _, rule := range triggeredRules {
		ctxData.Rule = rule
		for _, cond := range rule.Conditions {
			met, err := cond.Part.(Condition).IsMet(ctxData, cond.ParsedSettings)
			if err != nil {
				logrus.WithError(err).WithField("guild", ctxData.GS.ID).Error("failed checking if automod rule condition was met")
				continue OUTE_RULE_LOOP
			}

			if !met {
				continue OUTE_RULE_LOOP // condition was not met
			}
		}

		// all conditions passed
		filteredRules = append(filteredRules, rule)
	}

	if len(filteredRules) < 1 {
		return // no rules passed
	}

	p.RulesetRulesTriggeredCondsPassed(ruleset, filteredRules, ctxData)

}

func (p *Plugin) RulesetRulesTriggeredCondsPassed(ruleset *ParsedRuleset, triggeredRules []*ParsedRule, ctxData *TriggeredRuleData) error {

	// apply the effects
	for _, rule := range triggeredRules {
		ctxData.Rule = rule

		for _, effect := range rule.Effects {
			err := effect.Part.(Effect).Apply(ctxData, effect.ParsedSettings)
			if err != nil {
				logrus.WithError(err).WithField("guild", ruleset.RSModel.GuildID).WithField("part", effect.Part.Name()).Error("failed applying automod effect")
			}
		}
	}

	return nil
}
