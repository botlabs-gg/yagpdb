package quicksetup

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/automod"
	"github.com/botlabs-gg/yagpdb/v2/autorole"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/cplogs"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/botlabs-gg/yagpdb/v2/moderation"
	"github.com/botlabs-gg/yagpdb/v2/notifications"
	"github.com/botlabs-gg/yagpdb/v2/tickets"
	ticketmodels "github.com/botlabs-gg/yagpdb/v2/tickets/models"
	verificationmodels "github.com/botlabs-gg/yagpdb/v2/verification/models"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/botlabs-gg/yagpdb/v2/web/discorddata"
	"github.com/volatiletech/sqlboiler/v4/boil"
)

const DefaultWelcomeMessage = "Welcome {{.User.Mention}} to **{{.Guild.Name}}**!"

var panelLogKeySetUp = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "quicksetup_set_up", FormatString: "Quick Setup set up %s"})

// SetupForm holds every step of the setup. A picked channel or role of 0
// means one is created for it.
type SetupForm struct {
	Modules []string

	ModlogChannel  int64 `valid:"channel,true"`
	ModlogCategory int64

	AutomodInvites  bool
	AutomodMentions bool
	AutomodSpam     bool
	AutomodPhishing bool

	WelcomeChannel  int64 `valid:"channel,true"`
	WelcomeCategory int64
	WelcomeMessage  string `valid:"template,2000"`
	LeaveMessages   bool

	AutoroleRole int64 `valid:"role,true"`

	TicketsCategory int64
	TicketModRoles  []int64 `valid:"role,true"`

	VerifiedRole int64 `valid:"role,true"`
}

type moduleResult struct {
	Module  *module
	Done    bool
	Message string
}

type setupRun struct {
	ctx       context.Context
	guild     *dstate.GuildSet
	botMember *discordgo.Member
	botRole   *discordgo.Role
	form      *SetupForm

	modlogChannel    int64
	createdResources bool
	createdNames     map[int64]string
}

func applySetup(ctx context.Context, form *SetupForm) []*moduleResult {
	run := &setupRun{
		ctx:   ctx,
		guild: web.ContextGuild(ctx),
		form:  form,

		createdNames: make(map[int64]string),
	}
	run.botMember, _ = ctx.Value(common.ContextKeyBotMember).(*discordgo.Member)
	run.botRole, _ = ctx.Value(common.ContextKeyHighestBotRole).(*discordgo.Role)

	steps := map[string]func() (string, error){
		"moderation":   run.applyModeration,
		"automod":      run.applyAutomod,
		"welcome":      run.applyWelcome,
		"autorole":     run.applyAutorole,
		"tickets":      run.applyTickets,
		"verification": run.applyVerification,
	}

	var results []*moduleResult
	// Moderation goes first, as the later steps post to the modlog channel it sets up.
	for _, m := range availableModules() {
		if !slices.Contains(form.Modules, m.ID) {
			continue
		}

		message, err := steps[m.ID]()
		result := &moduleResult{Module: m, Done: err == nil, Message: message}
		if err != nil {
			result.Message = err.Error()
			web.CtxLogger(ctx).WithError(err).WithField("module", m.ID).Warn("quick setup step failed")
		} else {
			go cplogs.RetryAddEntry(web.NewLogEntryFromContext(ctx, panelLogKeySetUp, &cplogs.Param{Type: cplogs.ParamTypeString, Value: m.Name}))
		}
		results = append(results, result)
	}

	if run.createdResources {
		discorddata.PubEvictGuild(run.guild.ID)
	}

	return results
}

func (r *setupRun) applyModeration() (string, error) {
	channelID, err := r.textChannelOrCreate(r.form.ModlogChannel, "mod-log", r.form.ModlogCategory, true)
	if err != nil {
		return "", err
	}

	config, err := moderation.FetchConfig(r.guild.ID)
	if err != nil {
		return "", err
	}

	config.GuildID = r.guild.ID
	config.BanEnabled = true
	config.KickEnabled = true
	config.TimeoutEnabled = true
	config.WarnCommandsEnabled = true
	config.CleanEnabled = true
	config.ReportEnabled = true
	config.ActionChannel = channelID
	if config.ReportChannel == 0 {
		config.ReportChannel = channelID
	}
	config.LogBans = true
	config.LogUnbans = true
	config.LogKicks = true
	config.LogTimeouts = true
	config.WarnSendToModlog = true
	config.DelwarnSendToModlog = true

	if err := moderation.SaveConfig(config); err != nil {
		return "", err
	}

	r.modlogChannel = channelID
	message := fmt.Sprintf("Moderation commands are on, and actions are logged in %s.", r.channelLabel(channelID))
	return r.withPostingWarning(message, channelID), nil
}

func (r *setupRun) applyAutomod() (string, error) {
	alertChannel := r.modlogChannel
	if alertChannel == 0 {
		if config, err := moderation.FetchConfig(r.guild.ID); err == nil {
			alertChannel = config.ActionChannel
		}
	}

	protections := automod.StarterProtections{
		ServerInvites: r.form.AutomodInvites,
		MassMentions:  r.form.AutomodMentions,
		Spam:          r.form.AutomodSpam,
		PhishingLinks: r.form.AutomodPhishing,
		AlertChannel:  alertChannel,
	}
	if !protections.ServerInvites && !protections.MassMentions && !protections.Spam && !protections.PhishingLinks {
		return "", errors.New("No protections were picked.")
	}

	if err := automod.CreateStarterRuleset(r.ctx, r.guild.ID, protections); err != nil {
		return "", err
	}

	return fmt.Sprintf("Added the %q ruleset, which you can change in the automoderator.", automod.StarterRulesetName), nil
}

func (r *setupRun) applyWelcome() (string, error) {
	channelID, err := r.textChannelOrCreate(r.form.WelcomeChannel, "welcome", r.form.WelcomeCategory, false)
	if err != nil {
		return "", err
	}

	config, err := notifications.FetchConfig(r.guild.ID)
	if err != nil {
		return "", err
	}

	message := strings.TrimSpace(r.form.WelcomeMessage)
	if message == "" {
		message = DefaultWelcomeMessage
	}

	config.GuildID = r.guild.ID
	config.JoinServerEnabled = true
	config.JoinServerChannel = channelID
	config.JoinServerMsgs = []string{message}
	if r.form.LeaveMessages {
		config.LeaveEnabled = true
		config.LeaveChannel = channelID
	}

	if err := notifications.SaveConfig(config); err != nil {
		return "", err
	}

	return r.withPostingWarning(fmt.Sprintf("New members are welcomed in %s.", r.channelLabel(channelID)), channelID), nil
}

func (r *setupRun) applyAutorole() (string, error) {
	roleID, err := r.assignableRoleOrCreate(r.form.AutoroleRole, "Member")
	if err != nil {
		return "", err
	}

	config, err := autorole.GetAutoroleConfig(r.guild.ID)
	if err != nil {
		return "", err
	}

	config.Role = roleID
	config.IgnoreBots = true
	if err := (autorole.Form{AutoroleConfig: *config}).Save(r.guild.ID); err != nil {
		return "", err
	}

	return fmt.Sprintf("New members get the %s role.", r.roleLabel(roleID)), nil
}

func (r *setupRun) applyTickets() (string, error) {
	categoryID, err := r.categoryOrCreate(r.form.TicketsCategory, "Tickets")
	if err != nil {
		return "", err
	}

	config, err := ticketmodels.FindTicketConfigG(r.ctx, r.guild.ID)
	if err == sql.ErrNoRows {
		config = &ticketmodels.TicketConfig{GuildID: r.guild.ID}
	} else if err != nil {
		return "", err
	}

	config.Enabled = true
	config.TicketsChannelCategory = categoryID
	if len(r.form.TicketModRoles) > 0 {
		config.ModRoles = r.form.TicketModRoles
	}
	if config.AppendButtons == 0 {
		config.AppendButtons = tickets.AppendButtonsClose | tickets.AppendButtonsCloseWithReason
	}

	if err := config.UpsertG(r.ctx, true, []string{"guild_id"}, boil.Infer(), boil.Infer()); err != nil {
		return "", err
	}

	return "Members can open a ticket with the ticket open command.", nil
}

func (r *setupRun) applyVerification() (string, error) {
	roleID, err := r.assignableRoleOrCreate(r.form.VerifiedRole, "Verified")
	if err != nil {
		return "", err
	}

	config, err := verificationmodels.FindVerificationConfigG(r.ctx, r.guild.ID)
	if err == sql.ErrNoRows {
		config = &verificationmodels.VerificationConfig{GuildID: r.guild.ID}
	} else if err != nil {
		return "", err
	}

	config.Enabled = true
	config.VerifiedRole = roleID
	if config.LogChannel == 0 {
		config.LogChannel = r.modlogChannel
	}

	if err := config.UpsertG(r.ctx, true, []string{"guild_id"}, boil.Infer(), boil.Infer()); err != nil {
		return "", err
	}

	return fmt.Sprintf("New members get the %s role once they solve the captcha. Hide your channels from @everyone and allow them for this role, so only verified members can see them.", r.roleLabel(roleID)), nil
}

// textChannelOrCreate creates the channel at the top of the channel list
// when no category is picked for it.
func (r *setupRun) textChannelOrCreate(channelID int64, name string, categoryID int64, private bool) (int64, error) {
	if channelID != 0 {
		return channelID, nil
	}

	if categoryID != 0 && !r.isCategory(categoryID) {
		return 0, errors.New("The category picked for the #" + name + " channel no longer exists.")
	}

	var overwrites []*discordgo.PermissionOverwrite
	if private {
		overwrites = []*discordgo.PermissionOverwrite{
			{ID: r.guild.ID, Type: discordgo.PermissionOverwriteTypeRole, Deny: discordgo.PermissionViewChannel},
			{ID: common.BotUser.ID, Type: discordgo.PermissionOverwriteTypeMember, Allow: discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks | discordgo.PermissionReadMessageHistory},
		}
	}

	channel, err := common.BotSession.GuildChannelCreateWithOverwrites(r.guild.ID, name, discordgo.ChannelTypeGuildText, categoryID, overwrites)
	if err != nil {
		return 0, errors.WithMessage(err, "Could not create the #"+name+" channel")
	}

	r.createdResources = true
	r.createdNames[channel.ID] = "#" + channel.Name
	return channel.ID, nil
}

func (r *setupRun) categoryOrCreate(categoryID int64, name string) (int64, error) {
	if categoryID != 0 {
		if !r.isCategory(categoryID) {
			return 0, errors.New("The picked category no longer exists.")
		}
		return categoryID, nil
	}

	category, err := common.BotSession.GuildChannelCreate(r.guild.ID, name, discordgo.ChannelTypeGuildCategory)
	if err != nil {
		return 0, errors.WithMessage(err, "Could not create the "+name+" category")
	}

	r.createdResources = true
	r.createdNames[category.ID] = category.Name
	return category.ID, nil
}

// assignableRoleOrCreate refuses roles above the bot, which it could not give
// out and which would silently turn the feature off later.
func (r *setupRun) assignableRoleOrCreate(roleID int64, name string) (int64, error) {
	if roleID != 0 {
		role := r.guild.GetRole(roleID)
		if role == nil {
			return 0, errors.New("The picked role no longer exists.")
		}
		if role.Managed || r.botRole == nil || !common.IsRoleAbove(r.botRole, role) {
			return 0, errors.New("The bot cannot give out the picked role, as it is above the bot's highest role.")
		}
		return roleID, nil
	}

	role, err := common.BotSession.GuildRoleCreateComplex(r.guild.ID, discordgo.RoleCreate{Name: name})
	if err != nil {
		return 0, errors.WithMessage(err, "Could not create the "+name+" role")
	}

	r.createdResources = true
	r.createdNames[role.ID] = "@" + role.Name
	return role.ID, nil
}

func (r *setupRun) withPostingWarning(message string, channelID int64) string {
	if r.botMember == nil || r.guild.GetChannel(channelID) == nil {
		return message
	}

	const needed = discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks
	perms, err := r.guild.GetMemberPermissions(channelID, r.botMember.User.ID, r.botMember.Roles)
	if err != nil || perms&needed == needed || perms&discordgo.PermissionAdministrator != 0 {
		return message
	}

	return message + fmt.Sprintf(" The bot cannot post in %s yet, check its permissions there.", r.channelLabel(channelID))
}

func (r *setupRun) isCategory(channelID int64) bool {
	channel := r.guild.GetChannel(channelID)
	return channel != nil && channel.Type == discordgo.ChannelTypeGuildCategory
}

func (r *setupRun) channelLabel(channelID int64) string {
	if name, ok := r.createdNames[channelID]; ok {
		return name
	}
	if channel := r.guild.GetChannel(channelID); channel != nil {
		return "#" + channel.Name
	}

	return "the channel"
}

func (r *setupRun) roleLabel(roleID int64) string {
	if name, ok := r.createdNames[roleID]; ok {
		return name
	}
	if role := r.guild.GetRole(roleID); role != nil {
		return "@" + role.Name
	}

	return "new"
}
