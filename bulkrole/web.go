package bulkrole

import (
	_ "embed"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"slices"
	"strconv"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/autorole"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/cplogs"
	"github.com/botlabs-gg/yagpdb/v2/common/internalapi"
	"github.com/botlabs-gg/yagpdb/v2/common/pubsub"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/moderation"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/streaming"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/mediocregopher/radix/v3"
	"goji.io"
	"goji.io/pat"
)

//go:embed assets/bulkrole.html
var PageHTML string

type Form struct {
	BulkRoleConfig `valid:"traverse"`
}

var (
	_ web.SimpleConfigSaver = (*Form)(nil)
	_ web.CustomValidator   = (*Form)(nil)
)

func (f *Form) Validate(tmpl web.TemplateData, guildID int64) bool {
	ok := true

	if f.TargetRole == 0 {
		tmpl.AddAlerts(web.ErrorAlert("Please select a target role"))
		ok = false
	}

	if !slices.Contains(validOperations, f.Operation) {
		tmpl.AddAlerts(web.ErrorAlert("Please select an operation"))
		ok = false
	}

	if !slices.Contains(validFilterTypes, f.FilterType) {
		tmpl.AddAlerts(web.ErrorAlert("Please select a filter type"))
		ok = false
	}

	if f.FilterDate != "" {
		if _, err := time.Parse("2006-01-02", f.FilterDate); err != nil {
			tmpl.AddAlerts(web.ErrorAlert("Invalid date format. Use YYYY-MM-DD"))
			ok = false
		}
	}

	return ok
}

var (
	panelLogKeyStartedOperation   = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "bulkrole_started_operation", FormatString: "Started bulk role operation"})
	panelLogKeyCancelledOperation = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "bulkrole_cancelled_operation", FormatString: "Cancelled bulk role operation"})
)

// getExcludedRoleIDs returns the IDs of roles that should be excluded from bulk role operations
// This includes AutoRole, MuteRole, and StreamingRole
func getExcludedRoleIDs(guildID int64) []int64 {
	var excluded []int64

	if autoroleConfig, err := autorole.GetAutoroleConfig(guildID); err == nil && autoroleConfig.Role != 0 {
		excluded = append(excluded, autoroleConfig.Role)
	}

	if modConfig, err := moderation.FetchConfig(guildID); err == nil && modConfig.MuteRole != 0 {
		excluded = append(excluded, modConfig.MuteRole)
	}

	if streamingConfig, err := streaming.GetConfig(guildID); err == nil && streamingConfig.GiveRole != 0 {
		excluded = append(excluded, streamingConfig.GiveRole)
	}

	return excluded
}

func (f Form) Save(guildID int64) error {
	if f.FilterDate != "" {
		parsed, err := time.Parse("2006-01-02", f.FilterDate)
		if err != nil {
			return errors.WithMessage(err, "Invalid date format. Use YYYY-MM-DD")
		}
		f.BulkRoleConfig.FilterDateParsed = parsed
	}

	err := common.SetRedisJson(KeyGeneral(guildID), f.BulkRoleConfig)
	if err != nil {
		return err
	}

	pubsub.EvictCacheSet(configCache, guildID)
	return nil
}

func (f Form) Name() string {
	return "Bulk Role"
}

func (p *Plugin) InitWeb() {
	web.AddHTMLTemplate("bulkrole/assets/bulkrole.html", PageHTML)

	web.AddSidebarItem(web.SidebarCategoryRoles, &web.SidebarItem{
		Name:      "Bulk Role",
		URL:       "bulkrole",
		Icon:      "fas fa-users-cog",
		IsPremium: true,
	})

	muxer := goji.SubMux()

	web.CPMux.Handle(pat.New("/bulkrole"), muxer)
	web.CPMux.Handle(pat.New("/bulkrole/*"), muxer)

	muxer.Use(web.RequireBotMemberMW)
	muxer.Use(web.RequirePermMW(discordgo.PermissionManageRoles))
	muxer.Use(premium.PremiumGuildMW)

	getHandler := web.RenderHandler(handleGetBulkRoleMainPage, "cp_bulkrole")

	muxer.Handle(pat.Get(""), getHandler)
	muxer.Handle(pat.Get("/"), getHandler)

	muxer.Handle(pat.Post("/cancel"), web.ControllerPostHandler(handlePostCancelOperation, getHandler, nil))

	saveAndStartHandler := web.ControllerPostHandler(handlePostSaveAndStart, getHandler, Form{})
	muxer.Handle(pat.Post(""), saveAndStartHandler)
	muxer.Handle(pat.Post("/"), saveAndStartHandler)
}

func handleGetBulkRoleMainPage(w http.ResponseWriter, r *http.Request) interface{} {
	ctx := r.Context()
	activeGuild, tmpl := web.GetBaseCPContextData(ctx)

	general, err := GetBulkRoleConfig(activeGuild.ID)
	if err != nil {
		general = &BulkRoleConfig{
			Operation:           "assign",
			FilterType:          "all",
			NotificationChannel: 0,
			StartedBy:           0,
		}
	}
	tmpl["BulkRole"] = general

	excludedRoleIDs := getExcludedRoleIDs(activeGuild.ID)
	tmpl["ExcludedRoleIDs"] = excludedRoleIDs

	var autoroleStatus int
	common.RedisPool.Do(radix.Cmd(&autoroleStatus, "GET", "autorole:"+discordgo.StrID(activeGuild.ID)+":fullscan_status"))
	autoroleActive := autoroleStatus > 0

	var cooldownActive int
	common.RedisPool.Do(radix.Cmd(&cooldownActive, "EXISTS", "bulkrole:"+discordgo.StrID(activeGuild.ID)+":cooldown"))
	rateLimitActive := cooldownActive > 0

	var remainingCooldown int64
	if rateLimitActive {
		common.RedisPool.Do(radix.Cmd(&remainingCooldown, "TTL", "bulkrole:"+discordgo.StrID(activeGuild.ID)+":cooldown"))
	}

	var statusResp StatusResponse
	err = internalapi.GetWithGuild(activeGuild.ID, strconv.FormatInt(activeGuild.ID, 10)+"/bulkrole/status", &statusResp)
	status, processed, results := 0, 0, 0
	if err != nil {
		logger.WithError(err).Error("Failed to get bulk role status")
	} else {
		status, processed, results = statusResp.Status, statusResp.Processed, statusResp.Results
	}

	operationActive := status > 0
	tmpl["OperationActive"] = operationActive
	tmpl["AutoroleActive"] = autoroleActive
	tmpl["RateLimitActive"] = rateLimitActive
	tmpl["RemainingCooldown"] = remainingCooldown

	if operationActive {
		var statusText string
		switch status {
		case BulkRoleStarted:
			statusText = "Started"
		case BulkRoleIterating:
			statusText = "Processing members"
		case BulkRoleIterationDone:
			statusText = "Member processing completed, finalizing..."
		case BulkRoleProcessing:
			statusText = "Applying role changes"
		case BulkRoleCancelled:
			statusText = "Cancelled"
		case BulkRoleCompleted:
			statusText = "Completed"
		default:
			statusText = "Unknown"
		}
		tmpl["OperationStatus"] = statusText
		tmpl["ProcessedCount"] = processed
		tmpl["ResultsCount"] = results

		tmpl["TotalMembers"] = int(activeGuild.MemberCount)

		tmpl["CurrentOperation"] = map[string]interface{}{
			"TargetRole":          general.TargetRole,
			"Operation":           general.Operation,
			"FilterType":          general.FilterType,
			"FilterRoleIDs":       general.FilterRoleIDs,
			"FilterRequireAll":    general.FilterRequireAll,
			"MatchCriteria":       general.matchCriteriaText(),
			"FilterDate":          general.FilterDate,
			"NotificationChannel": general.NotificationChannel,
			"StartedBy":           general.StartedBy,
			"StartedByUsername":   general.StartedByUsername,
		}
	}

	return tmpl
}

func handlePostSaveAndStart(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, tmpl := web.GetBaseCPContextData(ctx)

	if premium.ContextPremiumTier(ctx) != premium.PremiumTierPremium {
		return tmpl.AddAlerts(web.ErrorAlert("Bulk Role Manager is premium only")), nil
	}

	form := ctx.Value(common.ContextKeyParsedForm).(*Form)
	user := web.ContextUser(ctx)
	form.StartedBy = user.ID
	form.StartedByUsername = user.String()

	err := form.Save(activeGuild.ID)
	if err != nil {
		return tmpl.AddAlerts(web.ErrorAlert("Failed to save configuration")), nil
	}

	err = internalapi.PostWithGuild(activeGuild.ID, strconv.FormatInt(activeGuild.ID, 10)+"/bulkrole/start", nil, nil)
	if err != nil {
		return tmpl.AddAlerts(web.ErrorAlert("Failed to start operation: " + err.Error())), nil
	}

	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyStartedOperation))

	return nil, nil
}

func handlePostCancelOperation(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, tmpl := web.GetBaseCPContextData(ctx)

	err := internalapi.PostWithGuild(activeGuild.ID, strconv.FormatInt(activeGuild.ID, 10)+"/bulkrole/cancel", nil, nil)
	if err != nil {
		return tmpl.AddAlerts(web.ErrorAlert("Failed to cancel operation: " + err.Error())), nil
	}

	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyCancelledOperation))

	return tmpl.AddAlerts(web.SucessAlert("Bulk role operation cancelled")), nil
}

var _ web.PluginWithServerHomeWidget = (*Plugin)(nil)

func (p *Plugin) LoadServerHomeWidget(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ag, templateData := web.GetBaseCPContextData(r.Context())

	if premium.ContextPremiumTier(r.Context()) != premium.PremiumTierPremium {
		templateData["WidgetTitle"] = "Bulk Role"
		templateData["WidgetBody"] = template.HTML("<p class='text-muted'>Premium feature</p>")
		return templateData, nil
	}

	templateData["WidgetTitle"] = "Bulk Role"
	templateData["SettingsPath"] = "/bulkrole"

	general, err := GetBulkRoleConfig(ag.ID)
	if err != nil {
		return templateData, err
	}

	enabledDisabled := ""
	targetRole := "none"

	if role := ag.GetRole(general.TargetRole); role != nil {
		templateData["WidgetEnabled"] = true
		enabledDisabled = web.EnabledDisabledSpanStatus(true)
		targetRole = html.EscapeString(role.Name)
	} else {
		templateData["WidgetDisabled"] = true
		enabledDisabled = web.EnabledDisabledSpanStatus(false)
	}

	format := `<ul>
	<li>Status: %s</li>
	<li>Target role: <code>%s</code></li>
	<li>Operation: <code>%s</code></li>
	<li>Notifications: <code>%s</code></li>
</ul>`

	notificationStatus := "disabled"
	if general.NotificationChannel != 0 {
		if channel := ag.GetChannel(general.NotificationChannel); channel != nil {
			notificationStatus = "#" + channel.Name
		} else {
			notificationStatus = "invalid channel"
		}
	}

	templateData["WidgetBody"] = template.HTML(fmt.Sprintf(format, enabledDisabled, targetRole, general.Operation, notificationStatus))

	return templateData, nil
}
