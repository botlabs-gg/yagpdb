package automod

import (
	"context"
	"encoding/json"
	"github.com/fatih/structs"
	"github.com/gorilla/schema"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/automod/models"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"goji.io"
	"goji.io/pat"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type CtxKey int

const (
	CtxKeyCurrentRuleset CtxKey = iota
)

var _ web.Plugin = (*Plugin)(nil)

func (p *Plugin) InitWeb() {
	tmplPath := "templates/plugins/automod.html"
	if common.Testing {
		tmplPath = "../../automod/assets/automod.html"
	}

	web.Templates = template.Must(web.Templates.ParseFiles(tmplPath))

	muxer := goji.SubMux()

	web.CPMux.Handle(pat.New("/automod"), muxer)
	web.CPMux.Handle(pat.New("/automod/*"), muxer)

	muxer.Use(web.RequireGuildChannelsMiddleware)
	muxer.Use(web.RequireFullGuildMW)

	getIndexHandler := web.ControllerHandler(p.handleGetAutomodIndex, "automod_index")

	muxer.Handle(pat.Get("/"), getIndexHandler)
	muxer.Handle(pat.Get(""), getIndexHandler)

	muxer.Handle(pat.Post("/new_ruleset"), web.ControllerPostHandler(p.handlePostAutomodCreateRuleset, getIndexHandler, CreateRulesetData{}, "Created a new automod ruleset"))

	// Ruleset specific handlers
	rulesetMuxer := goji.SubMux()
	muxer.Handle(pat.New("/ruleset/:rulesetID"), rulesetMuxer)
	muxer.Handle(pat.New("/ruleset/:rulesetID/*"), rulesetMuxer)

	rulesetMuxer.Use(p.currentRulesetMW(getIndexHandler))

	getRulesetHandler := web.ControllerHandler(p.handleGetAutomodRuleset, "automod_index")
	rulesetMuxer.Handle(pat.Get(""), getRulesetHandler)
	rulesetMuxer.Handle(pat.Get("/"), getRulesetHandler)

	rulesetMuxer.Handle(pat.Post("/update"), web.ControllerPostHandler(p.handlePostAutomodUpdateRuleset, getRulesetHandler, UpdateRulesetData{}, "Updated a ruleset"))

	rulesetMuxer.Handle(pat.Post("/new_rule"), web.ControllerPostHandler(p.handlePostAutomodCreateRule, getRulesetHandler, nil, "Created a new automod rule"))
	rulesetMuxer.Handle(pat.Post("/rule/:ruleID/delete"), web.ControllerPostHandler(p.handlePostAutomodDeleteRule, getRulesetHandler, nil, "Deleted a automod rule"))
	rulesetMuxer.Handle(pat.Post("/rule/:ruleID/update"), web.ControllerPostHandler(p.handlePostAutomodUpdateRule, getRulesetHandler, UpdateRuleData{}, "Updated a automod rule"))
}

func (p *Plugin) handleGetAutomodIndex(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	g, tmpl := web.GetBaseCPContextData(r.Context())

	rulesets, err := models.AutomodRulesets(qm.Where("guild_id = ?", g.ID)).AllG(r.Context())
	if err != nil {
		return tmpl, err
	}

	tmpl["AutomodRulesets"] = rulesets
	tmpl["PartMap"] = RulePartMap

	return tmpl, nil
}

type CreateRulesetData struct {
	Name string `valid:",1,100"`
}

func (p *Plugin) handlePostAutomodCreateRuleset(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	g, tmpl := web.GetBaseCPContextData(r.Context())
	data := r.Context().Value(common.ContextKeyParsedForm).(*CreateRulesetData)

	rs := &models.AutomodRuleset{
		Name:    data.Name,
		GuildID: g.ID,
	}

	err := rs.InsertG(r.Context(), boil.Infer())
	return tmpl, err
}

func (p *Plugin) currentRulesetMW(backupHandler http.Handler) func(http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler {
		mw := func(w http.ResponseWriter, r *http.Request) {
			g, tmpl := web.GetBaseCPContextData(r.Context())

			idStr := pat.Param(r, "rulesetID")
			parsed, _ := strconv.ParseInt(idStr, 10, 64)

			ruleset, err := models.AutomodRulesets(qm.Where("guild_id=? AND id=?", g.ID, parsed), qm.Load("RulesetAutomodRules.RuleAutomodRuleData"), qm.Load("RulesetAutomodRulesetConditions")).OneG(r.Context())
			if err != nil {
				tmpl.AddAlerts(web.ErrorAlert("Failed retrieving ruleset, maybe it was deleted?"))
				backupHandler.ServeHTTP(w, r)
				web.CtxLogger(r.Context()).WithError(err).Error("Failed retrieving automod ruleset")
				return
			}

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
}

func (p *Plugin) handlePostAutomodCreateRule(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	g, tmpl := web.GetBaseCPContextData(r.Context())

	ruleset := r.Context().Value(CtxKeyCurrentRuleset).(*models.AutomodRuleset)

	rule := &models.AutomodRule{
		GuildID:   g.ID,
		RulesetID: ruleset.ID,
	}

	err := rule.InsertG(r.Context(), boil.Infer())
	if err == nil {
		ruleset.R.RulesetAutomodRules = append(ruleset.R.RulesetAutomodRules, rule)
	}

	return tmpl, err
}

type UpdateRulesetData struct {
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

	ruleSet := r.Context().Value(CtxKeyCurrentRuleset).(*models.AutomodRuleset)

	tx, err := common.PQ.BeginTx(r.Context(), nil)
	if err != nil {
		return tmpl, err
	}

	// First wipe all previous rule data
	_, err = models.AutomodRulesetConditions(qm.Where("guild_id = ? AND ruleset_id = ?", g.ID, ruleSet.ID)).DeleteAll(r.Context(), tx)
	if err != nil {
		tx.Rollback()
		return tmpl, err
	}

	properConditions := make([]*models.AutomodRulesetCondition, len(conditions))

	// Insert the new data
	for i, cond := range conditions {
		proper := &models.AutomodRulesetCondition{
			GuildID:   g.ID,
			RulesetID: ruleSet.ID,
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

	// All done
	err = tx.Commit()
	if err != nil {
		return tmpl, err
	}

	// Reload the conditions now
	ruleSet.R.RulesetAutomodRulesetConditions = properConditions
	WebLoadRuleSettings(r, tmpl, ruleSet)

	return tmpl, err
}

type UpdateRuleData struct {
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

	// The form parsing utility dosen't take care of maps, so manually do that parsing for now
	triggers, validatedOK, err := ReadRuleRowData(g, tmpl, data.Triggers, r.Form, "Triggers")
	if err != nil || !validatedOK {
		return tmpl, err
	}

	conditions, validatedOK, err := ReadRuleRowData(g, tmpl, data.Conditions, r.Form, "Conditions")
	if err != nil || !validatedOK {
		return tmpl, err
	}

	effects, validatedOK, err := ReadRuleRowData(g, tmpl, data.Effects, r.Form, "Effects")
	if err != nil || !validatedOK {
		return tmpl, err
	}

	ruleSet := r.Context().Value(CtxKeyCurrentRuleset).(*models.AutomodRuleset)
	ruleIDStr := pat.Param(r, "ruleID")
	parsedRuleID, _ := strconv.ParseInt(ruleIDStr, 19, 64)

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
	saveDatumSlice := func(s []*models.AutomodRuleDatum) error {
		for _, d := range s {
			d.GuildID = g.ID
			d.RuleID = currentRule.ID

			err := d.Insert(r.Context(), tx, boil.Infer())
			if err != nil {
				tx.Rollback()
				return err
			}

		}
		return nil
	}

	err = saveDatumSlice(triggers)
	if err != nil {
		return tmpl, err
	}

	err = saveDatumSlice(conditions)
	if err != nil {
		return tmpl, err
	}

	err = saveDatumSlice(effects)
	if err != nil {
		return tmpl, err
	}

	// All done
	err = tx.Commit()
	if err != nil {
		return tmpl, err
	}

	// Reload the rules now
	currentRule.R.RuleAutomodRuleData = append(triggers, conditions...)
	currentRule.R.RuleAutomodRuleData = append(currentRule.R.RuleAutomodRuleData, effects...)

	WebLoadRuleSettings(r, tmpl, ruleSet)

	return tmpl, err
}

func ReadRuleRowData(guild *discordgo.Guild, tmpl web.TemplateData, rawData []RuleRowData, form url.Values, namePrefix string) (dst []*models.AutomodRuleDatum, validationOK bool, err error) {
	result := make([]*models.AutomodRuleDatum, 0, len(rawData))

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

		row := &models.AutomodRuleDatum{
			GuildID: guild.ID,
			Kind:    int(pType.Kind()),
			TypeID:  entry.Type,
		}

		// Process the settings for this part type if it has any
		dst := pType.DataType()
		if dst != nil {

			// Decode map[string][]string into the struct
			dec := schema.NewDecoder()
			dec.IgnoreUnknownKeys(true)
			err = dec.Decode(dst, entry.Data)
			if err != nil {
				return result, false, err
			}

			// Perform the validation
			validationOK = web.ValidateForm(guild, tmpl, dst)
			if !validationOK {
				return result, false, nil
			}

			// Serialize it back to store in the database
			serialized, err := json.Marshal(dst)
			if err != nil {
				return result, false, err
			}

			row.Settings = serialized
		} else {
			row.Settings = []byte("{}")
		}

		result = append(result, row)
	}

	return result, true, nil
}

func (p *Plugin) handlePostAutomodDeleteRule(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
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
