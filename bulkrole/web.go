package bulkrole

import (
	_ "embed"
	"fmt"
	"html"
	"html/template"
	"net/http"
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

var _ web.SimpleConfigSaver = (*Form)(nil)

var (
	panelLogKeyStartedOperation = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "bulkrole_started_operation", FormatString: "Started bulk role operation"})
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
		f.FilterDateParsed = parsed
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

	muxer.Handle(pat.Post(""), web.ControllerPostHandler(handlePostSaveAndStart, getHandler, nil))
	muxer.Handle(pat.Post("/"), web.ControllerPostHandler(handlePostSaveAndStart, getHandler, nil))
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

	err := r.ParseForm()
	if err != nil {
		return tmpl.AddAlerts(web.ErrorAlert("Failed to parse form data")), nil
	}

	user := web.ContextUser(r.Context())

	// Debug: Log the raw form values for FilterRoleIDs
	logger.WithField("guild", activeGuild.ID).WithField("filterRoleIDs_raw", r.Form["FilterRoleIDs"]).Info("Processing FilterRoleIDs from form")

	config := &BulkRoleConfig{
		TargetRole:          parseFormInt64(r.FormValue("TargetRole")),
		Operation:           r.FormValue("Operation"),
		FilterType:          r.FormValue("FilterType"),
		FilterRoleIDs:       parseFormInt64Slice(r.Form["FilterRoleIDs"]),
		FilterRequireAll:    r.FormValue("FilterRequireAll") == "true",
		FilterDate:          r.FormValue("FilterDate"),
		NotificationChannel: parseFormInt64(r.FormValue("NotificationChannel")),
		StartedBy:           user.ID,
		StartedByUsername:   user.String(),
	}

	// Debug: Log the parsed FilterRoleIDs
	logger.WithField("guild", activeGuild.ID).WithField("filterRoleIDs_parsed", config.FilterRoleIDs).Info("Parsed FilterRoleIDs")

	if config.FilterDate != "" {
		parsed, err := time.Parse("2006-01-02", config.FilterDate)
		if err != nil {
			return tmpl.AddAlerts(web.ErrorAlert("Invalid date format. Use YYYY-MM-DD")), nil
		}
		config.FilterDateParsed = parsed
	}

	if config.TargetRole == 0 {
		return tmpl.AddAlerts(web.ErrorAlert("Please select a target role")), nil
	}

	if config.Operation == "" {
		return tmpl.AddAlerts(web.ErrorAlert("Please select an operation")), nil
	}

	if config.FilterType == "" {
		return tmpl.AddAlerts(web.ErrorAlert("Please select a filter type")), nil
	}

	err = common.SetRedisJson(KeyGeneral(activeGuild.ID), config)
	if err != nil {
		return tmpl.AddAlerts(web.ErrorAlert("Failed to save configuration")), nil
	}

	pubsub.EvictCacheSet(configCache, activeGuild.ID)

	err = internalapi.PostWithGuild(activeGuild.ID, strconv.FormatInt(activeGuild.ID, 10)+"/bulkrole/start", nil, nil)
	if err != nil {
		return tmpl.AddAlerts(web.ErrorAlert("Failed to start operation: " + err.Error())), nil
	}

	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyStartedOperation))

	return nil, nil
}

func parseFormInt64(value string) int64 {
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func parseFormInt64Slice(values []string) []int64 {
	var result []int64
	for _, value := range values {
		if value != "" && value != "0" {
			if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
				result = append(result, parsed)
			}
		}
	}
	return result
}

func handlePostCancelOperation(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, tmpl := web.GetBaseCPContextData(ctx)

	if premium.ContextPremiumTier(ctx) != premium.PremiumTierPremium {
		return tmpl.AddAlerts(web.ErrorAlert("Bulk Role Manager is premium only")), nil
	}

	err := internalapi.PostWithGuild(activeGuild.ID, strconv.FormatInt(activeGuild.ID, 10)+"/bulkrole/cancel", nil, nil)
	if err != nil {
		return tmpl.AddAlerts(web.ErrorAlert("Failed to cancel operation: " + err.Error())), nil
	}

	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyStartedOperation))

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
