package automod

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	"github.com/jonas747/discordgo/v2"
	"github.com/jonas747/dstate/v4"
	"github.com/jonas747/yagpdb/analytics"
	"github.com/jonas747/yagpdb/automod/models"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/scheduledevents2"
	schEventsModels "github.com/jonas747/yagpdb/common/scheduledevents2/models"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

func (p *Plugin) BotInit() {

	commands.MessageFilterFuncs = append(commands.MessageFilterFuncs, p.checkMessage)

	eventsystem.AddHandlerAsyncLastLegacy(p, p.handleGuildMemberUpdate, eventsystem.EventGuildMemberUpdate)
	eventsystem.AddHandlerAsyncLastLegacy(p, p.handleMsgUpdate, eventsystem.EventMessageUpdate)
	eventsystem.AddHandlerAsyncLastLegacy(p, p.handleGuildMemberJoin, eventsystem.EventGuildMemberAdd)

	scheduledevents2.RegisterHandler("amod2_reset_channel_ratelimit", ResetChannelRatelimitData{}, handleResetChannelRatelimit)
}

type ResetChannelRatelimitData struct {
	ChannelID int64
}

func (p *Plugin) handleMsgUpdate(evt *eventsystem.EventData) {
	p.checkMessage(evt, evt.MessageUpdate().Message)
}

// called on new messages and edits
func (p *Plugin) checkMessage(evt *eventsystem.EventData, msg *discordgo.Message) bool {
	if !bot.IsNormalUserMessage(msg) {
		return false
	}

	if !evt.HasFeatureFlag(featureFlagEnabled) || msg.GuildID == 0 {
		return true
	}

	cs := evt.GS.GetChannel(msg.ChannelID)
	if cs == nil {
		return true
	}

	ms := dstate.MemberStateFromMember(msg.Member)

	stripped := ""
	return !p.CheckTriggers(nil, evt.GS, ms, msg, cs, func(trig *ParsedPart) (activated bool, err error) {
		if stripped == "" {
			stripped = PrepareMessageForWordCheck(msg.Content)
		}

		cast, ok := trig.Part.(MessageTrigger)
		if !ok {
			return
		}

		return cast.CheckMessage(&TriggerContext{GS: evt.GS, MS: ms, Data: trig.ParsedSettings}, cs, msg, stripped)
	})
}

func (p *Plugin) checkViolationTriggers(ctxData *TriggeredRuleData, violationName string) {
	// reset context data
	ctxData.ActivatedTriggers = nil
	ctxData.CurrentRule = nil
	ctxData.TriggeredRules = nil
	ctxData.MultipleTriggerRules = make(map[int64]struct{})

	if ctxData.RecursionCounter > 2 {
		logger.WithField("guild", ctxData.GS.ID).Warn("automod stopped infinite recursion")
		return
	}

	rulesets, err := p.FetchGuildRulesets(ctxData.GS.ID)
	if err != nil {
		logger.WithError(err).WithField("guild", ctxData.GS.ID).Error("failed fetching guild rulesets")
		return
	}

	if len(rulesets) < 1 {
		return
	}

	// retrieve users violations
	userViolations, err := models.AutomodViolations(qm.Where("guild_id = ? AND user_id = ? AND name = ?", ctxData.GS.ID, ctxData.MS.User.ID, violationName)).AllG(context.Background())
	if err != nil {
		logger.WithError(err).Error("automod failed retrieving user violations")
		return
	}

	for _, rs := range rulesets {
		if !rs.RSModel.Enabled {
			continue
		}

		// Check for triggered rules in this ruleset
		ctxData.Ruleset = rs
		if !p.CheckConditions(ctxData, rs.ParsedConditions) {
			continue
		}

		var activatedTriggers []*ParsedPart

		for _, rule := range rs.Rules {
			ctxData.CurrentRule = rule

			// Check conditions
			if !p.CheckConditions(ctxData, rule.Conditions) {
				continue
			}

			matched := false
			for _, trig := range rule.Triggers {
				violationTrigger, ok := trig.Part.(ViolationListener)
				if !ok {
					continue
				}

				tDataCast := trig.ParsedSettings.(*ViolationsTriggerData)
				if tDataCast.Name != violationName {
					continue
				}

				matched, err = violationTrigger.CheckUser(ctxData, userViolations, trig.ParsedSettings, false)
				if err != nil {
					logger.WithError(err).WithField("part_id", trig.RuleModel.ID).Error("failed checking violations trigger")
					continue
				}

				if matched && rule.Model.TriggerModeOr {
					// one trigger matching is enough for OR mode
					activatedTriggers = append(activatedTriggers, trig)
					break
				} else if !matched && !rule.Model.TriggerModeOr {
					// one trigger not matching is enough to fail the check in AND mode
					break
				}
			}

			if matched && !rule.Model.TriggerModeOr {
				// if the rule matched in AND mode, add all its triggers since all must have matched
				activatedTriggers = append(activatedTriggers, rule.Triggers...)
			}
		}

		if len(activatedTriggers) < 1 {
			// no matches :(
			continue
		}

		// sort them in order from highest to lowest threshold
		sort.Slice(activatedTriggers, func(i, j int) bool {
			d1 := activatedTriggers[i].ParsedSettings.(*ViolationsTriggerData)
			d2 := activatedTriggers[j].ParsedSettings.(*ViolationsTriggerData)

			return d1.Treshold > d2.Treshold
		})

		// do a second pass with the triggers sorted, incase only the highest should be triggered
		finalActivatedTriggers := make([]*ParsedPart, 0, len(activatedTriggers))
		finalTriggeredRules := make(map[int64]*ParsedRule) // map to avoid duplicates

		cClone := ctxData.Clone()
		cClone.Ruleset = rs

		triggeredOne := false
		for _, t := range activatedTriggers {
			ctxData.CurrentRule = t.ParentRule

			violationTrigger := t.Part.(ViolationListener)
			matched, err := violationTrigger.CheckUser(ctxData, userViolations, t.ParsedSettings, triggeredOne)
			if err != nil {
				logger.WithError(err).WithField("part_id", t.RuleModel.ID).Error("failed checking violations trigger")
				continue
			}

			if matched {
				finalActivatedTriggers = append(finalActivatedTriggers, t)
				if _, ok := finalTriggeredRules[t.ParentRule.Model.ID]; ok {
					// this rule has more than 1 trigger that matched
					cClone.MultipleTriggerRules[t.ParentRule.Model.ID] = struct{}{}
				} else {
					finalTriggeredRules[t.ParentRule.Model.ID] = t.ParentRule
				}

				triggeredOne = true
			}
		}

		// convert map to slice of values
		rs := make([]*ParsedRule, 0, len(finalTriggeredRules))
		for _, r := range finalTriggeredRules {
			rs = append(rs, r)
		}
		cClone.TriggeredRules = rs

		cClone.ActivatedTriggers = finalActivatedTriggers
		cClone.CurrentRule = nil

		go p.RulesetRulesTriggered(cClone, true)
		logger.WithField("guild", ctxData.GS.ID).Info("automod triggered ", len(finalTriggeredRules), " violation rules")
	}
}

func (p *Plugin) handleGuildMemberUpdate(evt *eventsystem.EventData) {
	evtData := evt.GuildMemberUpdate()

	ms := dstate.MemberStateFromMember(evtData.Member)
	if ms.Member.Nick == "" {
		return
	}

	p.checkNickname(ms)
}

func (p *Plugin) handleGuildMemberJoin(evt *eventsystem.EventData) {
	evtData := evt.GuildMemberAdd()

	ms := dstate.MemberStateFromMember(evtData.Member)

	p.checkJoin(ms)
	p.checkUsername(ms)
}

func (p *Plugin) checkNickname(ms *dstate.MemberState) {
	gs := bot.State.GetGuild(ms.GuildID)
	if gs == nil {
		return
	}

	p.CheckTriggers(nil, gs, ms, nil, nil, func(trig *ParsedPart) (activated bool, err error) {
		cast, ok := trig.Part.(NicknameListener)
		if !ok {
			return false, nil
		}

		return cast.CheckNickname(&TriggerContext{GS: gs, MS: ms, Data: trig.ParsedSettings})
	})
}

func (p *Plugin) checkUsername(ms *dstate.MemberState) {
	gs := bot.State.GetGuild(ms.GuildID)
	if gs == nil {
		return
	}

	p.CheckTriggers(nil, gs, ms, nil, nil, func(trig *ParsedPart) (activated bool, err error) {
		cast, ok := trig.Part.(UsernameListener)
		if !ok {
			return false, nil
		}

		return cast.CheckUsername(&TriggerContext{GS: gs, MS: ms, Data: trig.ParsedSettings})
	})
}

func (p *Plugin) checkJoin(ms *dstate.MemberState) {
	gs := bot.State.GetGuild(ms.GuildID)
	if gs == nil {
		return
	}

	p.CheckTriggers(nil, gs, ms, nil, nil, func(trig *ParsedPart) (activated bool, err error) {
		cast, ok := trig.Part.(JoinListener)
		if !ok {
			return false, nil
		}

		return cast.CheckJoin(&TriggerContext{GS: gs, MS: ms, Data: trig.ParsedSettings})
	})
}

func (p *Plugin) CheckTriggers(rulesets []*ParsedRuleset, gs *dstate.GuildSet, ms *dstate.MemberState, msg *discordgo.Message, cs *dstate.ChannelState, checkF func(trp *ParsedPart) (activated bool, err error)) bool {

	if rulesets == nil {
		var err error
		rulesets, err = p.FetchGuildRulesets(gs.ID)
		if err != nil {
			logger.WithError(err).WithField("guild", msg.GuildID).Error("failed fetching triggers")
			return false
		}

		if len(rulesets) < 1 {
			return false
		}
	}

	activatededRules := false

	for _, rs := range rulesets {
		if !rs.RSModel.Enabled {
			continue
		}

		ctxData := &TriggeredRuleData{
			MS:      ms,
			CS:      cs,
			GS:      gs,
			Plugin:  p,
			Ruleset: rs,

			Message:              msg,
			MultipleTriggerRules: make(map[int64]struct{}),
		}

		// Check conditions
		if !p.CheckConditions(ctxData, rs.ParsedConditions) {
			continue
		}

		// Check for triggered rules in this ruleset
		var triggeredRules []*ParsedRule
		var activatedTriggers []*ParsedPart

	OUTER:
		for _, rule := range rs.Rules {

			// Check the rule conditions
			ctxData.CurrentRule = rule
			if !p.CheckConditions(ctxData, rule.Conditions) {
				continue OUTER
			}
			ctxData.CurrentRule = nil

			var err error
			activated := false
			for _, trig := range rule.Triggers {

				activated, err = checkF(trig)
				if err != nil {
					logger.WithError(err).WithField("part_id", trig.RuleModel.ID).Error("failed checking trigger")
					continue
				}

				if activated && rule.Model.TriggerModeOr {
					// just 1 trigger matching is enough for OR mode
					triggeredRules = append(triggeredRules, rule)
					activatedTriggers = append(activatedTriggers, trig)
					break
				} else if !activated && !rule.Model.TriggerModeOr {
					// just 1 trigger not matching is enough to fail the whole match for AND mode
					break
				}

			}

			if activated && !rule.Model.TriggerModeOr {
				triggeredRules = append(triggeredRules, rule)
				// if the rule's triggers matched in AND mode, add all of its
				// triggers since all must have matched
				activatedTriggers = append(activatedTriggers, rule.Triggers...)

				if len(activatedTriggers) > 1 {
					// More than 1 trigger matched for this rule
					ctxData.MultipleTriggerRules[rule.Model.ID] = struct{}{}
				}
			}

		}

		if len(triggeredRules) < 1 {
			// no matches :(
			continue
		}

		ctxData.TriggeredRules = triggeredRules
		ctxData.ActivatedTriggers = activatedTriggers

		if ctxData.Message != nil {
			ctxData.StrippedMessageContent = PrepareMessageForWordCheck(ctxData.Message.Content)
		}

		go p.RulesetRulesTriggered(ctxData, true)
		activatededRules = true

		logger.WithField("guild", ctxData.GS.ID).Info("automod triggered ", len(triggeredRules), " rules")
	}

	return activatededRules
}

func (p *Plugin) RulesetRulesTriggered(ctxData *TriggeredRuleData, checkedConditions bool) {
	ruleset := ctxData.Ruleset

	if checkedConditions {
		p.RulesetRulesTriggeredCondsPassed(ruleset, ctxData.TriggeredRules, ctxData)
		return
	}

	// check if we match all conditions, starting with the ruleset conditions
	if !p.CheckConditions(ctxData, ctxData.Ruleset.ParsedConditions) {
		return
	}

	filteredRules := make([]*ParsedRule, 0, len(ctxData.TriggeredRules))

	// Check the rule specific conditins
	for _, rule := range ctxData.TriggeredRules {
		ctxData.CurrentRule = rule

		if !p.CheckConditions(ctxData, rule.Conditions) {
			continue
		}

		// all conditions passed
		filteredRules = append(filteredRules, rule)
	}

	if len(filteredRules) < 1 {
		return // no rules passed
	}

	p.RulesetRulesTriggeredCondsPassed(ruleset, filteredRules, ctxData)
}

func (p *Plugin) CheckConditions(ctxData *TriggeredRuleData, conditions []*ParsedPart) bool {
	// whether the match mode is OR rather than AND (only 1 match is needed instead of all)
	// if we're checking rulesets, it's always AND
	isOr := ctxData.CurrentRule != nil && ctxData.CurrentRule.Model.ConditionModeOr

	met := true
	var err error
	for _, cond := range conditions {
		met, err = cond.Part.(Condition).IsMet(ctxData, cond.ParsedSettings)
		if err != nil {
			logger.WithError(err).WithField("guild", ctxData.GS.ID).Error("failed checking if automod condition was met")
			met = false // assume the condition failed
		}

		if met && isOr {
			// one condition met is enough for OR mode
			return true
		} else if !met && !isOr {
			// one condition not met on AND mode is enough to fail the check
			return false
		}
	}

	return met
}

func (p *Plugin) RulesetRulesTriggeredCondsPassed(ruleset *ParsedRuleset, triggeredRules []*ParsedRule, ctxData *TriggeredRuleData) {

	loggedModels := make([]*models.AutomodTriggeredRule, len(triggeredRules))

	go analytics.RecordActiveUnit(ruleset.RSModel.GuildID, p, "rule_triggered")

	// apply the effects
	for i, rule := range triggeredRules {
		ctxData.CurrentRule = rule

		for _, effect := range rule.Effects {
			go func(fx *ParsedPart, ctx *TriggeredRuleData) {
				err := fx.Part.(Effect).Apply(ctx, fx.ParsedSettings)
				if err != nil {
					logger.WithError(err).WithField("guild", ruleset.RSModel.GuildID).WithField("part", fx.Part.Name()).Error("failed applying automod effect")
				}
			}(effect, ctxData.Clone())
		}

		// Log the rule activation
		cname := ""
		cid := int64(0)

		if ctxData.CS != nil {
			cname = ctxData.CS.Name
			cid = ctxData.CS.ID
		}

		tID := null.NewInt64(0, false)
		tTypeID := null.NewInt(0, false)

		// only attempt to find the trigger if one and only one trigger matched
		// for this rule
		if _, ok := ctxData.MultipleTriggerRules[rule.Model.ID]; !ok {
			for _, v := range ctxData.ActivatedTriggers {
				if v.RuleModel.RuleID == rule.Model.ID {
					tID.SetValid(v.RuleModel.ID)
					tTypeID.SetValid(v.RuleModel.TypeID)
					break
				}
			}
		}

		serializedExtraData := []byte("{}")
		if ctxData.Message != nil {
			var err error
			serializedExtraData, err = json.Marshal(ctxData.Message)
			if err != nil {
				logger.WithError(err).Error("automod failed serializing extra data")
				serializedExtraData = []byte("{}")
			}
		}

		loggedModels[i] = &models.AutomodTriggeredRule{
			ChannelID:     cid,
			ChannelName:   cname,
			GuildID:       ctxData.GS.ID,
			TriggerID:     tID,
			TriggerTypeid: tTypeID,
			RuleID:        null.Int64{Int64: rule.Model.ID, Valid: true},
			RuleName:      rule.Model.Name,
			RulesetName:   rule.Model.R.Ruleset.Name,
			UserID:        ctxData.MS.User.ID,
			UserName:      ctxData.MS.User.Username + "#" + ctxData.MS.User.Discriminator,
			Extradata:     serializedExtraData,
		}
	}

	tx, err := common.PQ.BeginTx(context.Background(), nil)
	if err != nil {
		logger.WithError(err).Error("failed creating transaction")
		return
	}

	for _, v := range loggedModels {
		err = v.Insert(context.Background(), tx, boil.Infer())
		if err != nil {
			logger.WithError(err).Error("failed inserting logged triggered rule")
			tx.Rollback()
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		logger.WithError(err).Error("failed committing logging transaction")
	}
}

var (
	cachedRulesets = common.CacheSet.RegisterSlot("amod2_rulesets", nil, int64(0))
	cachedLists    = common.CacheSet.RegisterSlot("amod2_lists", nil, int64(0))
)

func (p *Plugin) FetchGuildRulesets(guildID int64) ([]*ParsedRuleset, error) {
	v, err := cachedRulesets.GetCustomFetch(guildID, func(key interface{}) (interface{}, error) {
		rulesets, err := models.AutomodRulesets(qm.Where("guild_id=?", guildID),
			qm.Load("RulesetAutomodRules.RuleAutomodRuleData"), qm.Load("RulesetAutomodRulesetConditions")).AllG(context.Background())

		if err != nil {
			return nil, err
		}

		parsedSets := make([]*ParsedRuleset, 0, len(rulesets))
		for _, v := range rulesets {
			parsed, err := ParseRuleset(v)
			if err != nil {
				return nil, err
			}
			parsedSets = append(parsedSets, parsed)
		}

		return parsedSets, nil
	})

	if err != nil {
		return nil, err
	}

	cast := v.([]*ParsedRuleset)
	return cast, nil
}

func FetchGuildLists(guildID int64) ([]*models.AutomodList, error) {
	v, err := cachedLists.GetCustomFetch(guildID, func(key interface{}) (interface{}, error) {
		lists, err := models.AutomodLists(qm.Where("guild_id = ?", guildID)).AllG(context.Background())
		if err != nil {
			return nil, err
		}

		return []*models.AutomodList(lists), nil
	})

	if err != nil {
		return nil, err
	}

	cast := v.([]*models.AutomodList)
	return cast, nil
}

var ErrListNotFound = errors.New("list not found")

func FindFetchGuildList(guildID int64, listID int64) (*models.AutomodList, error) {
	lists, err := FetchGuildLists(guildID)
	if err != nil {
		return nil, err
	}

	for _, v := range lists {
		if v.ID == listID {
			return v, nil
		}
	}

	return nil, ErrListNotFound
}

func handleResetChannelRatelimit(evt *schEventsModels.ScheduledEvent, data interface{}) (retry bool, err error) {
	dataCast := data.(*ResetChannelRatelimitData)

	rl := 0
	edit := &discordgo.ChannelEdit{
		RateLimitPerUser: &rl,
	}

	_, err = common.BotSession.ChannelEditComplex(dataCast.ChannelID, edit)
	if err != nil {
		return scheduledevents2.CheckDiscordErrRetry(err), err
	}

	return false, nil
}
