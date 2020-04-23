package automod

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/fatih/structs"
	"github.com/gorilla/schema"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/automod/models"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/featureflags"
	"github.com/jonas747/yagpdb/web"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"goji.io"
	"goji.io/pat"
)

type CtxKey int

const (
	CtxKeyCurrentRuleset CtxKey = iota
)

var _ web.Plugin = (*Plugin)(nil)

func (p *Plugin) InitWeb() {
	web.LoadHTMLTemplate("../../automod/assets/automod.html", "templates/plugins/automod.html")
	web.AddSidebarItem(web.SidebarCategoryTools, &web.SidebarItem{
		Name: "Automoderator v2",
		URL:  "automod",
		Icon: "fas fa-robot",
	})

	muxer := goji.SubMux()

	web.CPMux.Handle(pat.New("/automod"), muxer)
	web.CPMux.Handle(pat.New("/automod/*"), muxer)

	muxer.Use(web.RequireGuildChannelsMiddleware)

	getIndexHandler := web.ControllerHandler(p.handleGetAutomodIndex, "automod_index")

	muxer.Handle(pat.Get("/"), getIndexHandler)
	muxer.Handle(pat.Get(""), getIndexHandler)
	muxer.Handle(pat.Get("/logs"), web.ControllerHandler(p.handleGetLogs, "automod_index"))

	muxer.Handle(pat.Post("/new_ruleset"), web.ControllerPostHandler(p.handlePostAutomodCreateRuleset, getIndexHandler, CreateRulesetData{}, "Created a new automod ruleset"))

	// List handlers
	muxer.Handle(pat.Post("/new_list"), web.ControllerPostHandler(p.handlePostAutomodCreateList, getIndexHandler, CreateListData{}, "Created a new automod list"))
	muxer.Handle(pat.Post("/list/:listID/update"), web.ControllerPostHandler(p.handlePostAutomodUpdateList, getIndexHandler, UpdateListData{}, "Updated a automod list"))
	muxer.Handle(pat.Post("/list/:listID/delete"), web.ControllerPostHandler(p.handlePostAutomodDeleteList, getIndexHandler, nil, "Deleted a automod list"))

	// Ruleset specific handlers
	rulesetMuxer := goji.SubMux()
	muxer.Handle(pat.New("/ruleset/:rulesetID"), rulesetMuxer)
	muxer.Handle(pat.New("/ruleset/:rulesetID/*"), rulesetMuxer)

	rulesetMuxer.Use(p.currentRulesetMW(getIndexHandler))

	getRulesetHandler := web.ControllerHandler(p.handleGetAutomodRuleset, "automod_index")
	rulesetMuxer.Handle(pat.Get(""), getRulesetHandler)
	rulesetMuxer.Handle(pat.Get("/"), getRulesetHandler)

	rulesetMuxer.Handle(pat.Post("/update"), web.ControllerPostHandler(p.handlePostAutomodUpdateRuleset, getRulesetHandler, UpdateRulesetData{}, "Updated a ruleset"))
	rulesetMuxer.Handle(pat.Post("/delete"), web.ControllerPostHandler(p.handlePostAutomodDeleteRuleset, getIndexHandler, nil, "Deleted a ruleset"))

	rulesetMuxer.Handle(pat.Post("/new_rule"), web.ControllerPostHandler(p.handlePostAutomodCreateRule, getRulesetHandler, CreateRuleData{}, "Created a new automod rule"))
	rulesetMuxer.Handle(pat.Post("/rule/:ruleID/delete"), web.ControllerPostHandler(p.handlePostAutomodDeleteRule, getRulesetHandler, nil, "Deleted a automod rule"))
	rulesetMuxer.Handle(pat.Post("/rule/:ruleID/update"), web.ControllerPostHandler(p.handlePostAutomodUpdateRule, getRulesetHandler, UpdateRuleData{}, "Updated a automod rule"))
}

func (p *Plugin) handleGetAutomodIndex(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	g, tmpl := web.GetBaseCPContextData(r.Context())

	rulesets, err := models.AutomodRulesets(qm.Where("guild_id = ?", g.ID), qm.OrderBy("id asc")).AllG(r.Context())
	if err != nil {
		return tmpl, err
	}

	lists, err := models.AutomodLists(qm.Where("guild_id=?", g.ID), qm.OrderBy("id asc")).AllG(r.Context())
	if err != nil {
		return tmpl, err
	}

	tmpl["AutomodLists"] = lists
	tmpl["AutomodRulesets"] = rulesets
	tmpl["PartMap"] = RulePartMap
	tmpl["PartList"] = RulePartList

	return tmpl, nil
}

func (p *Plugin) handleGetLogs(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	g, tmpl := web.GetBaseCPContextData(r.Context())

	tmpl["InLogs"] = true

	before := r.URL.Query().Get("before")
	after := r.URL.Query().Get("after")

	var beforeParsed int
	var afterParsed int
	if before != "" {
		beforeParsed, _ = strconv.Atoi(before)
	} else if after != "" {
		afterParsed, _ = strconv.Atoi(after)
	}

	qms := []qm.QueryMod{qm.Where("guild_id=?", g.ID), qm.OrderBy("id desc"), qm.Limit(100)}
	if beforeParsed != 0 {
		qms = append(qms, qm.Where("id < ?", beforeParsed))
	} else if afterParsed != 0 {
		qms = append(qms, qm.Where("id > ?", afterParsed))
	}

	entries, err := models.AutomodTriggeredRules(qms...).AllG(r.Context())
	if err != nil {
		return tmpl, err
	}

	tmpl["AutomodLogEntries"] = entries

	return p.handleGetAutomodIndex(w, r)
}

type CreateRulesetData struct {
	Name string `valid:",1,100"`
}

func (p *Plugin) handlePostAutomodCreateRuleset(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	g, tmpl := web.GetBaseCPContextData(r.Context())

	currentCount, err := models.AutomodRulesets(qm.Where("guild_id=?", g.ID)).CountG(r.Context())
	if err != nil {
		return tmpl, err
	}

	if currentCount >= int64(GuildMaxRulesets(g.ID)) {
		tmpl.AddAlerts(web.ErrorAlert("Reached max number of rulesets, ", MaxRulesets))
		return tmpl, nil
	}

	data := r.Context().Value(common.ContextKeyParsedForm).(*CreateRulesetData)

	rs := &models.AutomodRuleset{
		Name:    data.Name,
		GuildID: g.ID,
		Enabled: true,
	}

	err = rs.InsertG(r.Context(), boil.Infer())
	return tmpl, err
}

type CreateListData struct {
	Name string `valid:",1,50"`
}

func (p *Plugin) handlePostAutomodCreateList(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	g, tmpl := web.GetBaseCPContextData(r.Context())

	totalLists, err := models.AutomodLists(qm.Where("guild_id = ? ", g.ID)).CountG(r.Context())
	if err != nil {
		return tmpl, err
	}
	if totalLists >= int64(GuildMaxLists(g.ID)) {
		tmpl.AddAlerts(web.ErrorAlert(fmt.Sprintf("Reached max number of lists, %d for normal servers and %d for premium servers", MaxLists, MaxListsPremium)))
		return tmpl, nil
	}

	data := r.Context().Value(common.ContextKeyParsedForm).(*CreateListData)

	list := &models.AutomodList{
		Name:    data.Name,
		GuildID: g.ID,
		Content: []string{},
	}

	err = list.InsertG(r.Context(), boil.Infer())
	if err == nil {
		bot.EvictGSCache(g.ID, CacheKeyLists)
	}
	return tmpl, err
}

type UpdateListData struct {
	Content string `valid:",0,5000"`
}

func (p *Plugin) handlePostAutomodUpdateList(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	g, tmpl := web.GetBaseCPContextData(r.Context())
	data := r.Context().Value(common.ContextKeyParsedForm).(*UpdateListData)

	id := pat.Param(r, "listID")
	list, err := models.AutomodLists(qm.Where("guild_id=? AND id=?", g.ID, id)).OneG(r.Context())
	if err != nil {
		return nil, err
	}

	list.Content = strings.Fields(data.Content)
	_, err = list.UpdateG(r.Context(), boil.Whitelist("content"))
	if err == nil {
		bot.EvictGSCache(g.ID, CacheKeyLists)
	}
	return tmpl, err
}

func (p *Plugin) handlePostAutomodDeleteList(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	g, tmpl := web.GetBaseCPContextData(r.Context())

	id := pat.Param(r, "listID")
	list, err := models.AutomodLists(qm.Where("guild_id=? AND id=?", g.ID, id)).OneG(r.Context())
	if err != nil {
		return nil, err
	}

	_, err = list.DeleteG(r.Context())
	if err == nil {
		bot.EvictGSCache(g.ID, CacheKeyLists)
	}
	return tmpl, err
}

func (p *Plugin) currentRulesetMW(backupHandler http.Handler) func(http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler {
		mw := func(w http.ResponseWriter, r *http.Request) {
			g, tmpl := web.GetBaseCPContextData(r.Context())

			idStr := pat.Param(r, "rulesetID")
			parsed, _ := strconv.ParseInt(idStr, 10, 64)

			ruleset, err := models.AutomodRulesets(qm.Where("guild_id=? AND id=?", g.ID, parsed),
				qm.Load("RulesetAutomodRules.RuleAutomodRuleData"), qm.Load("RulesetAutomodRulesetConditions")).OneG(r.Context())

			if err != nil {
				tmpl.AddAlerts(web.ErrorAlert("Failed retrieving ruleset, maybe it was deleted?"))
				backupHandler.ServeHTTP(w, r)
				web.CtxLogger(r.Context()).WithError(err).Error("Failed retrieving automod ruleset")
				return
			}

			sort.Slice(ruleset.R.RulesetAutomodRules, func(i, j int) bool {
				return ruleset.R.RulesetAutomodRules[i].ID < ruleset.R.RulesetAutomodRules[j].ID
			})

			tmpl["CurrentRuleset"] = ruleset

			WebLoadRuleSettings(r, tmpl, ruleset)

			r = r.WithContext(context.WithValue(r.Context(), CtxKeyCurrentRuleset, ruleset))
			inner.ServeHTTP(w, r)
		}
		return http.HandlerFunc(mw)
	}
}

func (p *Plugin) handleGetAutomodRuleset(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	return p.handleGetAutomodIndex(w, r)
}

type CreateRuleData struct {
	Name string `valid:",1,50"`
}

func (p *Plugin) handlePostAutomodCreateRule(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	g, tmpl := web.GetBaseCPContextData(r.Context())

	ruleset := r.Context().Value(CtxKeyCurrentRuleset).(*models.AutomodRuleset)

	totalRules, err := models.AutomodRules(qm.Where("guild_id = ? ", g.ID)).CountG(r.Context())
	if err != nil {
		return tmpl, err
	}

	if totalRules >= int64(GuildMaxTotalRules(g.ID)) {
		tmpl.AddAlerts(web.ErrorAlert(fmt.Sprintf("Reached max number of rules, %d for normal servers and %d for premium servers", MaxTotalRules, MaxTotalRulesPremium)))
		return tmpl, nil
	}

	data := r.Context().Value(common.ContextKeyParsedForm).(*CreateRuleData)

	rule := &models.AutomodRule{
		GuildID:   g.ID,
		RulesetID: ruleset.ID,
		Name:      data.Name,
	}

	err = rule.InsertG(r.Context(), boil.Infer())
	if err == nil {
		ruleset.R.RulesetAutomodRules = append(ruleset.R.RulesetAutomodRules, rule)
	}

	return tmpl, err
}

type UpdateRulesetData struct {
	Name       string `valid:",1,50"`
	Enabled    bool
	Conditions []RuleRowData
}

func (p *Plugin) handlePostAutomodUpdateRuleset(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	g, tmpl := web.GetBaseCPContextData(r.Context())

	data := r.Context().Value(common.ContextKeyParsedForm).(*UpdateRulesetData)

	// The form parsing utility dosen't take care of maps, so manually do that parsing for now
	conditions, validatedOK, err := ReadRuleRowData(g, tmpl, data.Conditions, r.Form, "Conditions")
	if err != nil || !validatedOK {
		return tmpl, err
	}

	ruleset := r.Context().Value(CtxKeyCurrentRuleset).(*models.AutomodRuleset)

	tx, err := common.PQ.BeginTx(r.Context(), nil)
	if err != nil {
		return tmpl, err
	}

	// First wipe all previous rule data
	_, err = models.AutomodRulesetConditions(qm.Where("guild_id = ? AND ruleset_id = ?", g.ID, ruleset.ID)).DeleteAll(r.Context(), tx)
	if err != nil {
		tx.Rollback()
		return tmpl, err
	}

	properConditions := make([]*models.AutomodRulesetCondition, len(conditions))

	// Insert the new data
	for i, cond := range conditions {
		proper := &models.AutomodRulesetCondition{
			GuildID:   g.ID,
			RulesetID: ruleset.ID,
			Kind:      cond.Kind,
			TypeID:    cond.TypeID,
			Settings:  cond.Settings,
		}
		properConditions[i] = proper

		err := proper.Insert(r.Context(), tx, boil.Infer())
		if err != nil {
			tx.Rollback()
			return tmpl, err
		}
	}

	// Update the ruleset model itself
	ruleset.Name = data.Name
	ruleset.Enabled = data.Enabled
	_, err = ruleset.Update(r.Context(), tx, boil.Whitelist("name", "enabled"))
	if err != nil {
		tx.Rollback()
		return tmpl, err
	}

	// All done
	err = tx.Commit()
	if err != nil {
		return tmpl, err
	}

	bot.EvictGSCache(g.ID, CacheKeyRulesets)
	featureflags.MarkGuildDirty(g.ID)

	// Reload the conditions now
	ruleset.R.RulesetAutomodRulesetConditions = properConditions
	WebLoadRuleSettings(r, tmpl, ruleset)

	return tmpl, err
}

func (p *Plugin) handlePostAutomodDeleteRuleset(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	g, tmpl := web.GetBaseCPContextData(r.Context())

	ruleset := r.Context().Value(CtxKeyCurrentRuleset).(*models.AutomodRuleset)
	_, err := ruleset.DeleteG(r.Context())
	if err != nil {
		return tmpl, err
	}

	delete(tmpl, "CurrentRuleset")

	bot.EvictGSCache(g.ID, CacheKeyRulesets)
	featureflags.MarkGuildDirty(g.ID)

	return tmpl, err
}

type UpdateRuleData struct {
	Name       string `valid:",1,50"`
	Triggers   []RuleRowData
	Conditions []RuleRowData
	Effects    []RuleRowData
}

type RuleRowData struct {
	Type int
	Data map[string][]string
}

func (p *Plugin) handlePostAutomodUpdateRule(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	g, tmpl := web.GetBaseCPContextData(r.Context())

	data := r.Context().Value(common.ContextKeyParsedForm).(*UpdateRuleData)

	combinedParts := make([]*models.AutomodRuleDatum, 0, 10)
	// The form parsing utility dosen't take care of maps, so manually do that parsing for now
	triggers, validatedOK, err := ReadRuleRowData(g, tmpl, data.Triggers, r.Form, "Triggers")
	if err != nil || !validatedOK {
		return tmpl, err
	}
	combinedParts = append(combinedParts, triggers...)

	conditions, validatedOK, err := ReadRuleRowData(g, tmpl, data.Conditions, r.Form, "Conditions")
	if err != nil || !validatedOK {
		return tmpl, err
	}
	combinedParts = append(combinedParts, conditions...)

	effects, validatedOK, err := ReadRuleRowData(g, tmpl, data.Effects, r.Form, "Effects")
	if err != nil || !validatedOK {
		return tmpl, err
	}
	combinedParts = append(combinedParts, effects...)

	// retrieve the ruleset and the rule were working on
	ruleSet := r.Context().Value(CtxKeyCurrentRuleset).(*models.AutomodRuleset)
	ruleIDStr := pat.Param(r, "ruleID")
	parsedRuleID, _ := strconv.ParseInt(ruleIDStr, 10, 64)

	var currentRule *models.AutomodRule
	for _, v := range ruleSet.R.RulesetAutomodRules {
		if v.ID == parsedRuleID {
			currentRule = v
			break
		}
	}

	if currentRule == nil {
		return tmpl.AddAlerts(web.ErrorAlert("Unknown rule, maybe someone else deleted it in the meantime?")), nil
	}

	// Check limits
	combinedParts, ok, err := CheckLimits(common.PQ, currentRule, tmpl, combinedParts)
	if !ok || err != nil {
		return tmpl, err
	}

	tx, err := common.PQ.BeginTx(r.Context(), nil)
	if err != nil {
		return tmpl, err
	}

	// First wipe all previous rule data
	_, err = models.AutomodRuleData(qm.Where("guild_id = ? AND rule_id = ?", g.ID, currentRule.ID)).DeleteAll(r.Context(), tx)
	if err != nil {
		tx.Rollback()
		return tmpl, err
	}

	// Then insert the new data
	for _, part := range combinedParts {
		part.GuildID = g.ID
		part.RuleID = currentRule.ID

		err := part.Insert(r.Context(), tx, boil.Infer())
		if err != nil {
			tx.Rollback()
			return tmpl, err
		}
	}

	currentRule.Name = data.Name
	_, err = currentRule.Update(r.Context(), tx, boil.Whitelist("name"))
	if err != nil {
		tx.Rollback()
		return tmpl, err
	}

	// All done
	err = tx.Commit()
	if err != nil {
		return tmpl, err
	}

	// Reload the rules now
	currentRule.R.RuleAutomodRuleData = combinedParts

	WebLoadRuleSettings(r, tmpl, ruleSet)

	bot.EvictGSCache(g.ID, CacheKeyRulesets)
	featureflags.MarkGuildDirty(g.ID)

	return tmpl, err
}

func CheckLimits(exec boil.ContextExecutor, rule *models.AutomodRule, tmpl web.TemplateData, parts []*models.AutomodRuleDatum) (newParts []*models.AutomodRuleDatum, ok bool, err error) {
	// truncate to 20
	newParts = parts
	if len(newParts) > MaxRuleParts {
		newParts = newParts[:MaxRuleParts]
		tmpl.AddAlerts(web.WarningAlert("Truncated rule down to 20 triggers/conditions/effects, thats the max per rule."))
	}

	// Check number of message triggers and violation triggers
	numMessageTriggers := 0
	numViolationTriggers := 0
	for _, v := range newParts {
		for _, p := range RulePartList {
			if p.ID != v.TypeID {
				continue
			}

			if _, ok := p.Part.(MessageTrigger); ok {
				numMessageTriggers++
				break
			}

			if _, ok := p.Part.(ViolationListener); ok {
				numViolationTriggers++
				break
			}

		}
	}

	maxTotalMT := GuildMaxMessageTriggers(rule.GuildID)
	maxTotalVT := GuildMaxViolationTriggers(rule.GuildID)

	allParts, err := models.AutomodRuleData(qm.Where("guild_id = ? AND rule_id != ?", rule.GuildID, rule.ID)).All(context.Background(), exec)
	if err != nil {
		return
	}
	for _, v := range allParts {
		for _, p := range RulePartList {
			if p.ID != v.TypeID {
				continue
			}

			if _, ok := p.Part.(MessageTrigger); ok {
				numMessageTriggers++
				break
			}

			if _, ok := p.Part.(ViolationListener); ok {
				numViolationTriggers++
				break
			}
		}
	}

	ok = true
	if numMessageTriggers > maxTotalMT {
		tmpl.AddAlerts(web.ErrorAlert(fmt.Sprintf("Max message based triggers reached (%d for normal and %d for premium)", MaxMessageTriggers, MaxMessageTriggersPremium)))
		ok = false
	}

	if numViolationTriggers > maxTotalVT {
		tmpl.AddAlerts(web.ErrorAlert(fmt.Sprintf("Max violation based triggers reached (%d for normal and %d for premium)", MaxViolationTriggers, MaxViolationTriggersPremium)))
		ok = false
	}

	return
}

func ReadRuleRowData(guild *discordgo.Guild, tmpl web.TemplateData, rawData []RuleRowData, form url.Values, namePrefix string) (result []*models.AutomodRuleDatum, validationOK bool, err error) {
	parsedSettings := make([]*ParsedPart, 0, len(rawData))

	for i, entry := range rawData {
		for k, fv := range form {

			if strings.HasPrefix(k, namePrefix+"."+strconv.Itoa(i)+".Data.") {
				dataKey := strings.TrimPrefix(k, namePrefix+"."+strconv.Itoa(i)+".Data.")
				if entry.Data == nil {
					entry.Data = make(map[string][]string)
				}

				entry.Data[dataKey] = fv
			}
		}

		pType, ok := RulePartMap[entry.Type]
		if !ok {
			continue // Ignore unknown rules
		}

		parsed := &ParsedPart{
			Part: pType,
		}

		// Process the settings for this part type if it has any
		dst := pType.DataType()
		if dst != nil {

			// Decode map[string][]string into the struct
			dec := schema.NewDecoder()
			dec.IgnoreUnknownKeys(true)
			err = dec.Decode(dst, entry.Data)
			if err != nil {
				return nil, false, err
			}

			// Perform the validation
			validationOK = web.ValidateForm(guild, tmpl, dst)
			if !validationOK {
				return nil, false, nil
			}

			parsed.ParsedSettings = dst

		}

		parsedSettings = append(parsedSettings, parsed)
	}

	// merge the duplicate parts
	deDuplicated := make([]*ParsedPart, 0, len(parsedSettings))
OUTER:
	for _, parsedPart := range parsedSettings {
		mergeable, ok := parsedPart.Part.(MergeableRulePart)
		if !ok {
			// not mergeable, continue as normal
			deDuplicated = append(deDuplicated, parsedPart)
			continue
		}

		for _, d := range deDuplicated {
			if parsedPart.Part == d.Part {
				// already merged before, skip
				continue OUTER
			}
		}

		dupes := []interface{}{parsedPart.ParsedSettings}
		// find duplicate types
		for _, v := range parsedSettings {
			if v.Part == parsedPart.Part && v != parsedPart {
				dupes = append(dupes, v.ParsedSettings)
			}
		}

		if len(dupes) > 1 {
			merged := mergeable.MergeDuplicates(dupes)
			parsedPart.ParsedSettings = merged
		}

		deDuplicated = append(deDuplicated, parsedPart)
	}

	// finally make proper db models
	result = make([]*models.AutomodRuleDatum, len(deDuplicated))
	for i, v := range deDuplicated {
		model := &models.AutomodRuleDatum{
			GuildID: guild.ID,
			Kind:    int(v.Part.Kind()),
			TypeID:  InverseRulePartMap[v.Part],
		}

		if v.ParsedSettings != nil {
			// Serialize it back to store in the database
			serialized, err := json.Marshal(v.ParsedSettings)
			if err != nil {
				return result, false, err
			}

			model.Settings = serialized
		} else {
			model.Settings = []byte("{}")
		}

		result[i] = model
	}

	return result, true, nil
}

func (p *Plugin) handlePostAutomodDeleteRule(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	g := web.ContextGuild(r.Context())
	ruleset := r.Context().Value(CtxKeyCurrentRuleset).(*models.AutomodRuleset)
	toDelete := pat.Param(r, "ruleID")
	parsedRuleID, err := strconv.ParseInt(toDelete, 10, 64)
	if err != nil {
		return nil, err
	}

	for k, v := range ruleset.R.RulesetAutomodRules {
		if v.ID == parsedRuleID {
			_, err := v.DeleteG(r.Context())
			if err == nil {
				ruleset.R.RulesetAutomodRules = append(ruleset.R.RulesetAutomodRules[:k], ruleset.R.RulesetAutomodRules[k+1:]...)
				bot.EvictGSCache(g.ID, CacheKeyRulesets)
				featureflags.MarkGuildDirty(g.ID)
			}

			return nil, err
		}
	}

	return nil, nil
}

func WebLoadRuleSettings(r *http.Request, tmpl web.TemplateData, ruleset *models.AutomodRuleset) {
	// Parse the rule settings data
	// 1st index: rule
	// 2nd index: part
	// keys: part data
	parsedData := make([][]map[string]interface{}, len(ruleset.R.RulesetAutomodRules))
	for i, rule := range ruleset.R.RulesetAutomodRules {
		ruleData := make([]map[string]interface{}, len(rule.R.RuleAutomodRuleData))

		for j, part := range rule.R.RuleAutomodRuleData {
			dst := RulePartMap[part.TypeID].DataType()
			if dst != nil {
				// Parse the settings into the relevant struct
				err := json.Unmarshal(part.Settings, dst)
				if err != nil {
					web.CtxLogger(r.Context()).WithError(err).Error("failed parsing rule part data")
					continue
				}

				// Convert to map for ease of use with index, can't unmarshal directly into interface{} because then it will use float64 for the snowflakes
				m := structs.Map(dst)
				ruleData[j] = m
			}
		}

		parsedData[i] = ruleData
	}

	// The ruleset scoped conditions
	parsedRSData := make([]map[string]interface{}, len(ruleset.R.RulesetAutomodRulesetConditions))
	for i, part := range ruleset.R.RulesetAutomodRulesetConditions {
		dst := RulePartMap[part.TypeID].DataType()
		if dst != nil {
			// Parse the settings into the relevant struct
			err := json.Unmarshal(part.Settings, dst)
			if err != nil {
				web.CtxLogger(r.Context()).WithError(err).Error("failed parsing ruleset condition part data")
				continue
			}

			// Convert to map for ease of use with index, can't unmarshal directly into interface{} because then it will use float64 for the snowflakes
			m := structs.Map(dst)
			parsedRSData[i] = m
		}

	}

	tmpl["RulePartData"] = parsedData
	tmpl["RSPartData"] = parsedRSData
}

var _ web.PluginWithServerHomeWidget = (*Plugin)(nil)

func (p *Plugin) LoadServerHomeWidget(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	g, templateData := web.GetBaseCPContextData(r.Context())
	templateData["WidgetTitle"] = "Automod v2"
	templateData["SettingsPath"] = "/automod"

	rulesets, err := models.AutomodRulesets(qm.Where("guild_id = ?", g.ID), qm.Where("enabled = true")).CountG(r.Context())
	if err != nil {
		return templateData, err
	}

	rules, err := models.AutomodRules(qm.Where("guild_id = ?", g.ID)).CountG(r.Context())
	if err != nil {
		return templateData, err
	}

	templateData["WidgetBody"] = template.HTML(fmt.Sprintf(`<ul>
    <li>Active and enabled Rulesets: <code>%d</code></li>
    <li>Total rules: <code>%d</code></li>
</ul>`, rulesets, rules))

	if rulesets > 0 {
		templateData["WidgetEnabled"] = true
	} else {
		templateData["WidgetDisabled"] = true
	}

	return templateData, nil
}
