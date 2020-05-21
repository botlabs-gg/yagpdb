package automod

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
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

	cs := bot.State.Channel(true, msg.ChannelID)
	if cs == nil || cs.Guild == nil {
		return true
	}

	ms := dstate.MSFromDGoMember(cs.Guild, msg.Member)

	stripped := ""
	return !p.CheckTriggers(nil, ms, msg, cs, func(trig *ParsedPart) (activated bool, err error) {
		if stripped == "" {
			stripped = PrepareMessageForWordCheck(msg.Content)
		}

		cast, ok := trig.Part.(MessageTrigger)
		if !ok {
			return
		}

		return cast.CheckMessage(ms, cs, msg, stripped, trig.ParsedSettings)
	})
}

func (p *Plugin) checkViolationTriggers(ctxData *TriggeredRuleData, violationName string) {
	// reset context data
	ctxData.ActivatedTriggers = nil
	ctxData.CurrentRule = nil
	ctxData.TriggeredRules = nil

	if ctxData.RecursionCounter > 2 {
		logger.WithField("guild", ctxData.GS.ID).Warn("automod stopped infinite recursion")
		return
	}

	rulesets, err := p.FetchGuildRulesets(ctxData.GS)
	if err != nil {
		logger.WithError(err).WithField("guild", ctxData.GS.ID).Error("failed fetching guild rulesets")
		return
	}

	if len(rulesets) < 1 {
		return
	}

	// retrieve users violations
	userViolations, err := models.AutomodViolations(qm.Where("guild_id = ? AND user_id = ? AND name = ?", ctxData.GS.ID, ctxData.MS.ID, violationName)).AllG(context.Background())
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

			// check if one of the triggers should be activated
			for _, trig := range rule.Triggers {
				violationTrigger, ok := trig.Part.(ViolationListener)
				if !ok {
					continue
				}

				tDataCast := trig.ParsedSettings.(*ViolationsTriggerData)
				if tDataCast.Name != violationName {
					continue
				}

				matched, err := violationTrigger.CheckUser(ctxData, userViolations, trig.ParsedSettings, false)
				if err != nil {
					logger.WithError(err).WithField("part_id", trig.RuleModel.ID).Error("failed checking violations trigger")
					continue
				}

				if matched {
					activatedTriggers = append(activatedTriggers, trig)
					break
				}
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
		finalTriggeredRules := make([]*ParsedRule, 0, len(activatedTriggers))

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
				finalTriggeredRules = append(finalTriggeredRules, t.ParentRule)
				triggeredOne = true
			}
		}

		cClone := ctxData.Clone()
		cClone.Ruleset = rs
		cClone.TriggeredRules = finalTriggeredRules
		cClone.ActivatedTriggers = finalActivatedTriggers
		cClone.CurrentRule = nil

		go p.RulesetRulesTriggered(cClone, true)
		logger.WithField("guild", ctxData.GS.ID).Info("automod triggered ", len(finalTriggeredRules), " violation rules")
	}
}

func (p *Plugin) handleGuildMemberUpdate(evt *eventsystem.EventData) {
	evtData := evt.GuildMemberUpdate()

	ms := dstate.MSFromDGoMember(evt.GS, evtData.Member)
	if ms.Nick == "" {
		return
	}

	p.checkNickname(ms)
}

func (p *Plugin) handleGuildMemberJoin(evt *eventsystem.EventData) {
	evtData := evt.GuildMemberAdd()

	ms := dstate.MSFromDGoMember(evt.GS, evtData.Member)

	p.checkJoin(ms)
	p.checkUsername(ms)
}

func (p *Plugin) checkNickname(ms *dstate.MemberState) {
	p.CheckTriggers(nil, ms, nil, nil, func(trig *ParsedPart) (activated bool, err error) {
		cast, ok := trig.Part.(NicknameListener)
		if !ok {
			return false, nil
		}

		return cast.CheckNickname(ms, trig.ParsedSettings)
	})
}

func (p *Plugin) checkUsername(ms *dstate.MemberState) {
	p.CheckTriggers(nil, ms, nil, nil, func(trig *ParsedPart) (activated bool, err error) {
		cast, ok := trig.Part.(UsernameListener)
		if !ok {
			return false, nil
		}

		return cast.CheckUsername(ms, trig.ParsedSettings)
	})
}

func (p *Plugin) checkJoin(ms *dstate.MemberState) {
	p.CheckTriggers(nil, ms, nil, nil, func(trig *ParsedPart) (activated bool, err error) {
		cast, ok := trig.Part.(JoinListener)
		if !ok {
			return false, nil
		}

		return cast.CheckJoin(ms, trig.ParsedSettings)
	})
}

func (p *Plugin) CheckTriggers(rulesets []*ParsedRuleset, ms *dstate.MemberState, msg *discordgo.Message, cs *dstate.ChannelState, checkF func(trp *ParsedPart) (activated bool, err error)) bool {
	if rulesets == nil {
		var err error
		rulesets, err = p.FetchGuildRulesets(ms.Guild)
		if err != nil {
			logger.WithError(err).WithField("guild", ms.Guild.ID).Error("failed fetching triggers")
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
			GS:      ms.Guild,
			Plugin:  p,
			Ruleset: rs,

			Message: msg,
		}

		// check if we match all conditions, starting with the ruleset conditions
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

			for _, trig := range rule.Triggers {

				activated, err := checkF(trig)
				if err != nil {
					logger.WithError(err).WithField("part_id", trig.RuleModel.ID).Error("failed checking trigger")
					continue
				}

				if activated {
					triggeredRules = append(triggeredRules, rule)
					activatedTriggers = append(activatedTriggers, trig)
					break
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
	// check if we match all conditions, starting with the ruleset conditions
	for _, cond := range conditions {
		met, err := cond.Part.(Condition).IsMet(ctxData, cond.ParsedSettings)
		if err != nil {
			logger.WithError(err).WithField("guild", ctxData.GS.ID).Error("failed checking if automod condition was met")
			return false // assume the condition failed
		}

		if !met {
			return false // condition was not met
		}
	}

	return true
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
			ctxData.CS.Owner.RLock()
			cname = ctxData.CS.Name
			ctxData.CS.Owner.RUnlock()
			cid = ctxData.CS.ID
		}

		tID := int64(0)
		tTypeID := 0
		for _, v := range ctxData.ActivatedTriggers {
			if v.RuleModel.RuleID == rule.Model.ID {
				tID = v.RuleModel.ID
				tTypeID = v.RuleModel.TypeID
				break
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
			TriggerID:     null.Int64{Int64: tID, Valid: tID != 0},
			TriggerTypeid: tTypeID,
			RuleID:        null.Int64{Int64: rule.Model.ID, Valid: true},
			RuleName:      rule.Model.Name,
			RulesetName:   rule.Model.R.Ruleset.Name,
			UserID:        ctxData.MS.ID,
			UserName:      ctxData.MS.Username + "#" + ctxData.MS.StrDiscriminator(),
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

const (
	CacheKeyRulesets bot.GSCacheKey = "automod_2_rulesets"
	CacheKeyLists    bot.GSCacheKey = "automod_2_lists"
)

func (p *Plugin) FetchGuildRulesets(gs *dstate.GuildState) ([]*ParsedRuleset, error) {
	v, err := gs.UserCacheFetch(CacheKeyRulesets, func() (interface{}, error) {
		rulesets, err := models.AutomodRulesets(qm.Where("guild_id=?", gs.ID),
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

func FetchGuildLists(gs *dstate.GuildState) ([]*models.AutomodList, error) {
	v, err := gs.UserCacheFetch(CacheKeyLists, func() (interface{}, error) {
		lists, err := models.AutomodLists(qm.Where("guild_id = ?", gs.ID)).AllG(context.Background())
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

func FindFetchGuildList(gs *dstate.GuildState, listID int64) (*models.AutomodList, error) {
	lists, err := FetchGuildLists(gs)
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
