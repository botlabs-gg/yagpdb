package commands

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/analytics"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/commands/models"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/mediocregopher/radix/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"

	commonmodels "github.com/botlabs-gg/yagpdb/v2/common/models"
)

type ContextKey int

const (
	CtxKeyRedisClient ContextKey = iota
)

var (
	CategoryGeneral = &dcmd.Category{
		Name:        "General",
		Description: "General & informational commands",
		HelpEmoji:   "ℹ️",
		EmbedColor:  0xe53939,
	}
	CategoryTool = &dcmd.Category{
		Name:        "Tools & Utilities",
		Description: "Various miscellaneous commands",
		HelpEmoji:   "🔨",
		EmbedColor:  0xeaed40,
	}
	CategoryModeration = &dcmd.Category{
		Name:        "Moderation",
		Description: "Moderation commands",
		HelpEmoji:   "👮",
		EmbedColor:  0xdb0606,
	}
	CategoryFun = &dcmd.Category{
		Name:        "Fun",
		Description: "Various commands meant for entertainment",
		HelpEmoji:   "🎉",
		EmbedColor:  0x5ae26c,
	}
	CategoryDebug = &dcmd.Category{
		Name:        "Debug & Maintenance",
		Description: "Debug and other commands to inspect the bot",
		HelpEmoji:   "🖥",
		EmbedColor:  0,
	}
)

var (
	RKeyCommandCooldown      = func(uID int64, cmd string) string { return "cmd_cd:" + discordgo.StrID(uID) + ":" + cmd }
	RKeyCommandCooldownGuild = func(gID int64, cmd string) string { return "cmd_guild_cd:" + discordgo.StrID(gID) + ":" + cmd }
	RKeyCommandLock          = func(uID int64, cmd string) string { return "cmd_lock:" + discordgo.StrID(uID) + ":" + cmd }

	CommandExecTimeout = time.Minute

	runningCommands     = make([]*RunningCommand, 0)
	runningcommandsLock sync.Mutex
	shuttingDown        = new(int32)
)

type RunningCommand struct {
	GuildID   int64
	ChannelID int64
	AuthorID  int64

	Command *YAGCommand
}

type RolesRunFunc func(gs *dstate.GuildSet) ([]int64, error)

// Slight extension to the simplecommand, it will check if the command is enabled in the HandleCommand func
// And invoke a custom handlerfunc with provided redis client
type YAGCommand struct {
	Name            string   // Name of command, what its called from
	Aliases         []string // Aliases which it can also be called from
	Description     string   // Description shown in non targetted help
	LongDescription string   // Longer description when this command was targetted

	Arguments      []*dcmd.ArgDef // Slice of argument definitions, ctx.Args will always be the same size as this slice (although the data may be nil)
	RequiredArgs   int            // Number of reuquired arguments, ignored if combos is specified
	ArgumentCombos [][]int        // Slice of argument pairs, will override RequiredArgs if specified
	ArgSwitches    []*dcmd.ArgDef // Switches for the commadn to use

	AllowEveryoneMention bool

	HideFromCommandsPage bool   // Set to  hide this command from the commands page
	Key                  string // GuildId is appended to the key, e.g if key is "test:", it will check for "test:<guildid>"
	CustomEnabled        bool   // Set to true to handle the enable check itself
	Default              bool   // The default enabled state of this command

	Cooldown           int // Cooldown in seconds before user can use it again
	CmdCategory        *dcmd.Category
	GuildScopeCooldown int

	RunInDM      bool // Set to enable this commmand in DM's
	HideFromHelp bool // Set to hide from help

	RequireDiscordPerms      []int64   // Require users to have one of these permission sets to run the command
	RequiredDiscordPermsHelp string    // Optional message that shows up when users run the help command that documents user permission requirements for the command
	RequireBotPerms          [][]int64 // Discord permissions that the bot needs to run the command, (([0][0] && [0][1] && [0][2]) || ([1][0] && [1][1]...))

	Middlewares []dcmd.MiddleWareFunc

	// Run is ran when the command has sucessfully been parsed
	// It returns a reply and an error
	// the reply can have a type of string, *MessageEmbed or error
	RunFunc dcmd.RunFunc

	Plugin common.Plugin

	// Slash commands integration (this is unused on sub commands)
	//
	// Note about channel overrides:
	// Since the slash commands permissions is limited to roles/users only and can't be per channel, it takes the common set of roles required to run the command between all overrides
	// e.g if the command does not require a role in one channel, but it requires one in another channel, then the required permission for the slash command will be none set,
	// note that these settings are still checked when the command is run, but they just show the command in the client even if you can't use it in this case, so its just a visual limitation of slash commands.
	//
	// If it's disabled in all chanels, then for default_enabled = true commands, it adds the everyone role to the blacklist, otherwise it adds no role to the whitelist (does this even work? can i use the everyone role in this context?)
	SlashCommandEnabled bool

	// Wether the command is enabled in all guilds by default or not
	DefaultEnabled bool

	// If default enabled = false
	// then this returns the roles that CAN use the command
	// if default enabled = true
	// then this returns the roles that CAN'T use the command
	RolesRunFunc RolesRunFunc

	slashCommandID int64

	IsResponseEphemeral bool
	NSFW                bool
}

// CmdWithCategory puts the command in a category, mostly used for the help generation
func (yc *YAGCommand) Category() *dcmd.Category {
	return yc.CmdCategory
}

func (yc *YAGCommand) Descriptions(data *dcmd.Data) (short, long string) {
	return yc.Description, yc.Description + "\n" + yc.LongDescription
}

func (yc *YAGCommand) ArgDefs(data *dcmd.Data) (args []*dcmd.ArgDef, required int, combos [][]int) {
	return yc.Arguments, yc.RequiredArgs, yc.ArgumentCombos
}

func (yc *YAGCommand) Switches() []*dcmd.ArgDef {
	return yc.ArgSwitches
}

var metricsExcecutedCommands = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "bot_commands_total",
	Help: "Commands the bot executed",
}, []string{"name", "trigger_type"})

func (yc *YAGCommand) Run(data *dcmd.Data) (interface{}, error) {
	if data.TriggerType == dcmd.TriggerTypeSlashCommands && data.SlashCommandTriggerData.Interaction.Type == discordgo.InteractionApplicationCommandAutocomplete {
		for _, v := range data.SlashCommandTriggerData.Options {
			if !v.Focused {
				continue
			}

			for _, arg := range data.Args {
				if strings.EqualFold(arg.Def.Name, v.Name) {
					return arg.Def.AutocompleteFunc(data, arg)
				}
			}
		}
		return []*discordgo.ApplicationCommandOptionChoice{}, nil // fallback in case something goes wrong so the user doesn't see failed
	}

	if !yc.RunInDM && data.Source == dcmd.TriggerSourceDM {
		return nil, nil
	}

	if yc.NSFW {
		channel := data.GuildData.GS.GetChannelOrThread(data.ChannelID)
		if !channel.NSFW {
			return "This command can be used only in age-restricted channels", nil
		}
	}

	// Send typing to indicate the bot's working
	if confSetTyping.GetBool() && data.TriggerType != dcmd.TriggerTypeSlashCommands {
		common.BotSession.ChannelTyping(data.ChannelID)
	}

	logger := yc.Logger(data)

	// Track how long execution of a command took
	started := time.Now()
	rawCommand := ""
	triggerType := ""
	switch data.TriggerType {
	case dcmd.TriggerTypeSlashCommands:
		rawCommand = yc.Name + " (slashcommand)"
		triggerType = "slashcommand"
	default:
		rawCommand = data.TraditionalTriggerData.Message.Content
		triggerType = "message"
	}
	defer func() {
		yc.logExecutionTime(time.Since(started), rawCommand, data.Author.Username)
	}()

	guildID := int64(0)
	if data.GuildData != nil {
		guildID = data.GuildData.GS.ID
	}

	cmdFullName := yc.Name
	if len(data.ContainerChain) > 1 {
		lastContainer := data.ContainerChain[len(data.ContainerChain)-1]
		cmdFullName = lastContainer.Names[0] + " " + cmdFullName
	}

	// Set up log entry for later use
	logEntry := &commonmodels.ExecutedCommand{
		UserID:    discordgo.StrID(data.Author.ID),
		ChannelID: discordgo.StrID(data.ChannelID),

		Command:    cmdFullName,
		RawCommand: rawCommand,
		TimeStamp:  time.Now(),
	}

	if data.GuildData != nil {
		logEntry.GuildID.SetValid(discordgo.StrID(data.GuildData.GS.ID))
	}

	metricsExcecutedCommands.With(prometheus.Labels{"name": "(other)", "trigger_type": triggerType}).Inc()

	logger.Info("Handling command: " + rawCommand)

	runCtx, cancelExec := context.WithTimeout(data.Context(), CommandExecTimeout)
	defer cancelExec()

	// Run the command
	r, cmdErr := yc.RunFunc(data.WithContext(runCtx))
	logEntry.ResponseTime = int64(time.Since(started))

	if cmdErr != nil {
		if errors.Cause(cmdErr) == context.Canceled || errors.Cause(cmdErr) == context.DeadlineExceeded {
			r = &EphemeralOrGuild{Content: "Took longer than " + CommandExecTimeout.String() + " to handle command: `" + rawCommand + "`, Cancelled the command."}
		}

		if r == nil || r == "" {
			r = &EphemeralOrGuild{Content: yc.humanizeError(cmdErr)}
		}

		// set cmdErr to nil if this was a user error top stop it from being recorded and logged as an actual error
		if _, isUserErr := errors.Cause(cmdErr).(dcmd.UserError); isUserErr {
			cmdErr = nil
		}
	} else {
		// set cooldowns
		err := yc.SetCooldowns(data.ContainerChain, data.Author.ID, guildID)
		if err != nil {
			logger.WithError(err).Error("Failed setting cooldown")
		}

		if yc.Plugin != nil {
			go analytics.RecordActiveUnit(guildID, yc.Plugin, "cmd_executed_"+strings.ToLower(cmdFullName))
		}
	}

	// Create command log entry
	err := logEntry.InsertG(data.Context(), boil.Infer())
	if err != nil {
		logger.WithError(err).Error("Failed creating command execution log")
	}

	return r, cmdErr
}

func (yc *YAGCommand) humanizeError(err error) string {
	cause := errors.Cause(err)

	switch t := cause.(type) {
	case PublicError:
		return "The command returned an error: " + t.Error()
	case UserError:
		return "Unable to run the command: " + t.Error()
	case *discordgo.RESTError:
		if t.Message != nil && t.Message.Message != "" {
			if t.Message.Message == "Unknown Message" {
				return "The bot was not able to perform the action, Discord responded with: " + t.Message.Message + ". Please be sure you ran the command in the same channel as the message."
			} else if t.Response != nil && t.Response.StatusCode == 403 {
				return "The bot permissions has been incorrectly set up on this server for it to run this command: " + t.Message.Message
			}

			return "The bot was not able to perform the action, discord responded with: " + t.Message.Message
		}
	}

	return "Something went wrong when running this command, either discord or the bot may be having issues."
}

// PostCommandExecuted sends the response and handles the trigger and response deletions
func (yc *YAGCommand) PostCommandExecuted(settings *CommandSettings, cmdData *dcmd.Data, resp interface{}, err error) {
	if err != nil {
		yc.Logger(cmdData).WithError(err).Error("Command returned error")
	}

	if cmdData.GuildData != nil {
		if resp == nil && err != nil {
			err = errors.New(FilterResp(err.Error(), cmdData.GuildData.GS.ID).(string))
		} else if resp != nil {
			resp = FilterResp(resp, cmdData.GuildData.GS.ID)
		}
	}

	if (settings.DelResponse && settings.DelResponseDelay < 1) && cmdData.TraditionalTriggerData != nil {
		// Set up the trigger deletion if set
		if settings.DelTrigger {
			go func() {
				time.Sleep(time.Duration(settings.DelTriggerDelay) * time.Second)
				common.BotSession.ChannelMessageDelete(cmdData.ChannelID, cmdData.TraditionalTriggerData.Message.ID)
			}()
		}
		return // Don't bother sending the reponse if it has no delete delay
	}

	// Use the error as the response if no response was provided
	if resp == nil && err != nil {
		resp = fmt.Sprintf("'%s' command returned an error: %s", cmdData.Cmd.FormatNames(false, "/"), err)
	}

	// send a alternative message in case of embeds in channels with no embeds perms
	if cmdData.GuildData != nil && cmdData.TriggerType != dcmd.TriggerTypeSlashCommands {
		switch resp.(type) {
		case *discordgo.MessageEmbed, []*discordgo.MessageEmbed:
			if hasPerms, _ := bot.BotHasPermissionGS(cmdData.GuildData.GS, cmdData.ChannelID, discordgo.PermissionEmbedLinks); !hasPerms {
				resp = "This command returned an embed but the bot does not have embed links permissions in this channel, cannot send the response."
			}
		}
	}

	// Send the response
	var replies []*discordgo.Message
	if resp == nil && cmdData.TriggerType == dcmd.TriggerTypeSlashCommands {
		common.BotSession.DeleteInteractionResponse(common.BotApplication.ID, cmdData.SlashCommandTriggerData.Interaction.Token)
	} else if resp != nil {
		replies, err = dcmd.SendResponseInterface(cmdData, resp, true)
		if err != nil {
			logger.WithError(err).Error("failed sending command response")
		}
	}

	if settings.DelResponse {
		go func() {
			time.Sleep(time.Second * time.Duration(settings.DelResponseDelay))
			ids := make([]int64, 0, len(replies))
			for _, v := range replies {
				if v == nil {
					continue
				}

				ids = append(ids, v.ID)
			}

			// If trigger deletion had the same delay, delete the trigger in the same batch
			if settings.DelTrigger && settings.DelTriggerDelay == settings.DelResponseDelay && cmdData.TraditionalTriggerData != nil {
				ids = append(ids, cmdData.TraditionalTriggerData.Message.ID)
			}

			if len(ids) == 1 {
				common.BotSession.ChannelMessageDelete(cmdData.ChannelID, ids[0])
			} else if len(ids) > 1 {
				common.BotSession.ChannelMessagesBulkDelete(cmdData.ChannelID, ids)
			}
		}()
	}

	// If were deleting the trigger in a seperate call from the response deletion
	if settings.DelTrigger && (!settings.DelResponse || settings.DelTriggerDelay != settings.DelResponseDelay) && cmdData.TraditionalTriggerData != nil {
		go func() {
			time.Sleep(time.Duration(settings.DelTriggerDelay) * time.Second)
			common.BotSession.ChannelMessageDelete(cmdData.ChannelID, cmdData.TraditionalTriggerData.Message.ID)
		}()
	}
}

type CanExecuteType int

const (
	// ReasonError                    = "An error occured"
	// ReasonCommandDisabaledSettings = "Command is disabled in the settings"
	// ReasonMissingRole              = "Missing a required role for this command"
	// ReasonIgnoredRole              = "Has a ignored role for this command"
	// ReasonUserMissingPerms         = "User is missing one or more permissions to run this command"
	// ReasonCooldown                 = "This command is on cooldown"

	ReasonError CanExecuteType = iota
	ReasonCommandDisabaledSettings
	ReasonMissingRole
	ReasonIgnoredRole
	ReasonUserMissingPerms
	ReasonBotMissingPerms
	ReasonCooldown
)

type CanExecuteError struct {
	Type    CanExecuteType
	Message string
}

// checks if the specified user can execute the command, and if so returns the settings for said command
func (yc *YAGCommand) checkCanExecuteCommand(data *dcmd.Data) (canExecute bool, resp *CanExecuteError, settings *CommandSettings, err error) {
	// Check guild specific settings if not triggered from a DM
	if data.GuildData != nil {
		guild := data.GuildData.GS

		if data.TriggerType != dcmd.TriggerTypeSlashCommands {
			if hasPerms, _ := bot.BotHasPermissionGS(guild, data.ChannelID, discordgo.PermissionViewChannel|discordgo.PermissionSendMessages); !hasPerms {
				return false, nil, nil, nil
			}
		}

		settings, err = yc.GetSettings(data.ContainerChain, data.GuildData.CS, guild)
		if err != nil {
			resp = &CanExecuteError{
				Type:    ReasonError,
				Message: "Failed retrieving cs.settings",
			}

			return false, resp, settings, errors.WithMessage(err, "cs.GetSettings")
		}

		if !settings.Enabled {
			resp = &CanExecuteError{
				Type:    ReasonCommandDisabaledSettings,
				Message: "Command is disabled in this channel by server admins",
			}

			return false, resp, settings, nil
		}

		member := data.GuildData.MS
		guildRoles := roleNames(guild)

		if missingWhitelistErr := checkWhitelistRoles(guildRoles, settings.RequiredRoles, data); missingWhitelistErr != nil {
			return false, missingWhitelistErr, settings, nil
		}

		if blacklistErr := checkBlacklistRoles(guildRoles, settings.IgnoreRoles, data); blacklistErr != nil {
			return false, blacklistErr, settings, nil
		}

		if userPermsErr := yc.checkRequiredMemberPerms(guild, member, data.ChannelID); userPermsErr != nil {
			return false, userPermsErr, settings, nil
		}

		if userPermsErr := yc.checkRequiredBotPerms(guild, data.ChannelID); userPermsErr != nil {
			return false, userPermsErr, settings, nil
		}
	} else {
		settings = &CommandSettings{
			Enabled: true,
		}
	}

	guildID := int64(0)
	if data.GuildData != nil {
		guildID = data.GuildData.GS.ID
	}

	// Check the command cooldown
	cdLeft, err := yc.LongestCooldownLeft(data.ContainerChain, data.Author.ID, guildID)
	if err != nil {
		// Just pretend the cooldown is off...
		yc.Logger(data).Error("Failed checking command cooldown")
	}

	if cdLeft > 0 {
		resp = &CanExecuteError{
			Type:    ReasonCooldown,
			Message: "Command is on cooldown",
		}
		return false, resp, settings, nil
	}

	// If we got here then we can execute the command
	return true, nil, settings, nil
}

func checkWhitelistRoles(guildRoles map[int64]string, whitelistRoles []int64, data *dcmd.Data) *CanExecuteError {
	member := data.GuildData.MS

	if len(whitelistRoles) < 1 {
		// no whitelist roles
		return nil
	}

	for _, r := range member.Member.Roles {
		if common.ContainsInt64Slice(whitelistRoles, r) {
			// we have a whitelist role!
			return nil
		}
	}

	var humanizedRoles strings.Builder
	for i, v := range whitelistRoles {
		if i != 0 {
			humanizedRoles.WriteString(", ")
		}

		if i >= 20 {
			left := len(whitelistRoles) - i
			if left > 1 {
				// if there's only 1 role left then just finished, otherwise add this
				humanizedRoles.WriteString(fmt.Sprintf("(+%d roles)", left))
				break
			}
		}

		name := "unknown-role"
		if v, ok := guildRoles[v]; ok {
			name = v
		}

		humanizedRoles.WriteString(name)
	}

	return &CanExecuteError{
		Type:    ReasonMissingRole,
		Message: "You need at least one of the server allowed roles: " + humanizedRoles.String(),
	}
}

func checkBlacklistRoles(guildRoles map[int64]string, blacklistRoles []int64, data *dcmd.Data) *CanExecuteError {
	member := data.GuildData.MS

	if len(blacklistRoles) < 1 {
		// no blacklist roles
		return nil
	}

	hasRole := int64(0)
	for _, r := range member.Member.Roles {
		if common.ContainsInt64Slice(blacklistRoles, r) {
			// we have a blacklist role!
			hasRole = r
			break
		}
	}

	if hasRole == 0 {
		// We don't have a blacklist role!
		return nil
	}

	// we do have a blacklist roles :(
	humanizedRole := "unknown-role"
	if v, ok := guildRoles[hasRole]; ok {
		humanizedRole = v
	}

	return &CanExecuteError{
		Type:    ReasonIgnoredRole,
		Message: "You have one of the server denylist roles: " + humanizedRole,
	}
}

func (yc *YAGCommand) checkRequiredMemberPerms(gs *dstate.GuildSet, ms *dstate.MemberState, channelID int64) *CanExecuteError {
	// This command has permission sets required, if the user has one of them then allow this command to be used
	if len(yc.RequireDiscordPerms) < 1 {
		return nil
	}

	perms, err := gs.GetMemberPermissions(channelID, ms.User.ID, ms.Member.Roles)
	if err != nil {
		return &CanExecuteError{
			Type:    ReasonError,
			Message: "Failed fetching member perms?",
		}
	}

	for _, permSet := range yc.RequireDiscordPerms {
		if permSet&int64(perms) == permSet {
			// we have one of the required perms!
			return nil
		}
	}

	humanizedPerms := make([]string, 0, len(yc.RequireDiscordPerms))
	for _, v := range yc.RequireDiscordPerms {
		h := common.HumanizePermissions(v)
		joined := strings.Join(h, " and ")
		humanizedPerms = append(humanizedPerms, "("+joined+")")
	}

	return &CanExecuteError{
		Type:    ReasonUserMissingPerms,
		Message: "You need at least one of the following permissions to run this command: " + strings.Join(humanizedPerms, " or "),
	}
}

func (yc *YAGCommand) checkRequiredBotPerms(gs *dstate.GuildSet, channelID int64) *CanExecuteError {
	// This command has permission sets required, if the user has one of them then allow this command to be used
	if len(yc.RequireBotPerms) < 1 {
		return nil
	}

	perms, err := bot.BotPermissions(gs, channelID)
	if err != nil {
		return &CanExecuteError{
			Type:    ReasonError,
			Message: "Failed fetching bot perms",
		}
	}

	// need all the perms within atleast one group
OUTER:
	for _, permGroup := range yc.RequireBotPerms {

		for _, v := range permGroup {
			if perms&v != v {
				continue OUTER
			}
		}

		// if we got here we had them all in the group
		return nil
	}

	humanizedPerms := make([]string, 0, len(yc.RequireDiscordPerms))
	for _, group := range yc.RequireBotPerms {
		gHumanized := make([]string, 0, len(group))
		for _, v := range group {
			h := common.HumanizePermissions(v)
			joined := strings.Join(h, " and ")
			gHumanized = append(gHumanized, joined)
		}

		humanizedPerms = append(humanizedPerms, "("+strings.Join(gHumanized, " and ")+")")
	}

	return &CanExecuteError{
		Type:    ReasonBotMissingPerms,
		Message: "The bot needs at least one of the following permissions to run this command: " + strings.Join(humanizedPerms, " or "),
	}
}

func roleNames(gs *dstate.GuildSet) map[int64]string {
	result := make(map[int64]string)
	for _, v := range gs.Roles {
		result[v.ID] = v.Name
	}

	return result
}

func (cs *YAGCommand) logExecutionTime(dur time.Duration, raw string, sender string) {
	logger.Infof("Handled Command [%4dms] %s: %s", int(dur.Seconds()*1000), sender, raw)
}

// customEnabled returns wether the command is enabled by it's custom key or not
func (cs *YAGCommand) customEnabled(guildID int64) (bool, error) {
	// No special key so it's automatically enabled
	if cs.Key == "" || cs.CustomEnabled {
		return true, nil
	}

	// Check redis for settings
	var enabled bool
	err := common.RedisPool.Do(radix.Cmd(&enabled, "GET", cs.Key+discordgo.StrID(guildID)))
	if err != nil {
		return false, err
	}

	if cs.Default {
		enabled = !enabled
	}

	if !enabled {
		return false, nil
	}

	return enabled, nil
}

type CommandSettings struct {
	Enabled         bool
	AlwaysEphemeral bool

	DelTrigger       bool
	DelResponse      bool
	DelTriggerDelay  int
	DelResponseDelay int

	RequiredRoles []int64
	IgnoreRoles   []int64
}

func GetOverridesForChannel(cs *dstate.ChannelState, guild *dstate.GuildSet) ([]*models.CommandsChannelsOverride, error) {
	if cs.Type.IsThread() {
		// Look for overrides from the parent channel, not the thread.
		cs = guild.GetChannel(cs.ParentID)
	}

	// Fetch the overrides from the database, we treat the global settings as an override for simplicity
	channelOverrides, err := models.CommandsChannelsOverrides(qm.Where("(? = ANY (channels) OR global=true OR ? = ANY (channel_categories)) AND guild_id=?", cs.ID, cs.ParentID, guild.ID), qm.Load("CommandsCommandOverrides")).AllG(context.Background())
	if err != nil {
		return nil, err
	}

	return channelOverrides, nil
}

// GetSettings returns the settings from the command, generated from the servers channel and command overrides
func (yc *YAGCommand) GetSettings(containerChain []*dcmd.Container, cs *dstate.ChannelState, guild *dstate.GuildSet) (settings *CommandSettings, err error) {

	// Fetch the overrides from the database, we treat the global settings as an override for simplicity
	channelOverrides, err := GetOverridesForChannel(cs, guild)
	if err != nil {
		err = errors.WithMessage(err, "GetOverridesForChannel")
		return
	}

	return yc.GetSettingsWithLoadedOverrides(containerChain, guild.ID, channelOverrides)
}

func (yc *YAGCommand) GetSettingsWithLoadedOverrides(containerChain []*dcmd.Container, guildID int64, channelOverrides []*models.CommandsChannelsOverride) (settings *CommandSettings, err error) {
	settings = &CommandSettings{}

	// Some commands have custom places to toggle their enabled status
	ce, err := yc.customEnabled(guildID)
	if err != nil {
		err = errors.WithMessage(err, "customEnabled")
		return
	}

	if !ce {
		return
	}

	if yc.HideFromCommandsPage {
		settings.Enabled = true
		return
	}

	if len(channelOverrides) < 1 {
		settings.Enabled = true
		return // No overrides
	}

	// Find the global and per channel override
	var global *models.CommandsChannelsOverride
	var channelOverride *models.CommandsChannelsOverride

	for _, v := range channelOverrides {
		if v.Global {
			global = v
		} else {
			channelOverride = v
		}
	}

	cmdFullName := yc.Name
	if len(containerChain) > 1 {
		lastContainer := containerChain[len(containerChain)-1]
		cmdFullName = lastContainer.Names[0] + " " + cmdFullName
	}

	// Assign the global settings, if existing
	if global != nil {
		yc.fillSettings(cmdFullName, global, settings)
	}

	// Assign the channel override, if existing
	if channelOverride != nil {
		yc.fillSettings(cmdFullName, channelOverride, settings)
	}

	return
}

// Fills the command settings from a channel override, and if a matching command override is found, the command override
func (cs *YAGCommand) fillSettings(cmdFullName string, override *models.CommandsChannelsOverride, settings *CommandSettings) {
	settings.Enabled = override.CommandsEnabled
	settings.AlwaysEphemeral = override.AlwaysEphemeral

	settings.IgnoreRoles = override.IgnoreRoles
	settings.RequiredRoles = override.RequireRoles

	settings.DelResponse = override.AutodeleteResponse
	settings.DelTrigger = override.AutodeleteTrigger
	settings.DelResponseDelay = override.AutodeleteResponseDelay
	settings.DelTriggerDelay = override.AutodeleteTriggerDelay

OUTER:
	for _, cmdOverride := range override.R.CommandsCommandOverrides {
		for _, cmd := range cmdOverride.Commands {
			if strings.EqualFold(cmd, cmdFullName) {
				settings.Enabled = cmdOverride.CommandsEnabled
				settings.AlwaysEphemeral = cmdOverride.AlwaysEphemeral

				settings.IgnoreRoles = cmdOverride.IgnoreRoles
				settings.RequiredRoles = cmdOverride.RequireRoles

				settings.DelResponse = cmdOverride.AutodeleteResponse
				settings.DelTrigger = cmdOverride.AutodeleteTrigger
				settings.DelResponseDelay = cmdOverride.AutodeleteResponseDelay
				settings.DelTriggerDelay = cmdOverride.AutodeleteTriggerDelay

				break OUTER
			}
		}
	}
}

// LongestCooldownLeft returns the longest cooldown for this command, either user scoped or guild scoped
func (cs *YAGCommand) LongestCooldownLeft(cc []*dcmd.Container, userID int64, guildID int64) (int, error) {
	cdUser, err := cs.UserScopeCooldownLeft(cc, userID)
	if err != nil {
		return 0, err
	}

	cdGuild, err := cs.GuildScopeCooldownLeft(cc, guildID)
	if err != nil {
		return 0, err
	}

	if cdUser > cdGuild {
		return cdUser, nil
	}

	return cdGuild, nil
}

// UserScopeCooldownLeft returns the number of seconds before a command can be used again by this user
func (cs *YAGCommand) UserScopeCooldownLeft(cc []*dcmd.Container, userID int64) (int, error) {
	if cs.Cooldown < 1 {
		return 0, nil
	}

	var ttl int
	err := common.RedisPool.Do(radix.Cmd(&ttl, "TTL", RKeyCommandCooldown(userID, cs.FindNameFromContainerChain(cc))))
	if err != nil {
		return 0, errors.WithStackIf(err)
	}

	return ttl, nil
}

// GuildScopeCooldownLeft returns the number of seconds before a command can be used again on this server
func (cs *YAGCommand) GuildScopeCooldownLeft(cc []*dcmd.Container, guildID int64) (int, error) {
	if cs.GuildScopeCooldown < 1 {
		return 0, nil
	}

	var ttl int
	err := common.RedisPool.Do(radix.Cmd(&ttl, "TTL", RKeyCommandCooldownGuild(guildID, cs.FindNameFromContainerChain(cc))))
	if err != nil {
		return 0, errors.WithStackIf(err)
	}

	return ttl, nil
}

// SetCooldowns is a helper that serts both User and Guild cooldown
func (cs *YAGCommand) SetCooldowns(cc []*dcmd.Container, userID int64, guildID int64) error {
	err := cs.SetCooldownUser(cc, userID)
	if err != nil {
		return errors.WithStackIf(err)
	}

	err = cs.SetCooldownGuild(cc, guildID)
	if err != nil {
		return errors.WithStackIf(err)
	}

	return nil
}

// SetCooldownUser sets the user scoped cooldown of the command as it's defined in the struct
func (cs *YAGCommand) SetCooldownUser(cc []*dcmd.Container, userID int64) error {
	if cs.Cooldown < 1 {
		return nil
	}
	now := time.Now().Unix()

	err := common.RedisPool.Do(radix.FlatCmd(nil, "SET", RKeyCommandCooldown(userID, cs.FindNameFromContainerChain(cc)), now, "EX", cs.Cooldown))
	return errors.WithStackIf(err)
}

// SetCooldownGuild sets the guild scoped cooldown of the command as it's defined in the struct
func (cs *YAGCommand) SetCooldownGuild(cc []*dcmd.Container, guildID int64) error {
	if cs.GuildScopeCooldown < 1 {
		return nil
	}

	now := time.Now().Unix()
	err := common.RedisPool.Do(radix.FlatCmd(nil, "SET", RKeyCommandCooldownGuild(guildID, cs.FindNameFromContainerChain(cc)), now, "EX", cs.GuildScopeCooldown))
	return errors.WithStackIf(err)
}

func (yc *YAGCommand) Logger(data *dcmd.Data) *logrus.Entry {
	l := logger.WithField("cmd", yc.FindNameFromContainerChain(data.ContainerChain))
	if data != nil {
		if data.Author != nil {
			l = l.WithField("user_n", data.Author.Username)
			l = l.WithField("user_id", data.Author.ID)
		}

		if data.GuildData != nil && data.GuildData.CS != nil {
			l = l.WithField("channel", data.GuildData.CS.ID)
		}

		if data.GuildData != nil && data.GuildData.GS != nil {
			l = l.WithField("guild", data.GuildData.GS.ID)
		}
	}

	return l
}

func (yc *YAGCommand) GetTrigger() *dcmd.Trigger {
	trigger := dcmd.NewTrigger(yc.Name, yc.Aliases...).SetEnableInDM(yc.RunInDM).SetEnableInGuildChannels(true)
	trigger = trigger.SetHideFromHelp(yc.HideFromHelp)
	if len(yc.Middlewares) > 0 {
		trigger = trigger.SetMiddlewares(yc.Middlewares...)
	}
	return trigger
}

// Keys and other sensitive information shouldnt be sent in error messages, but just in case it is
func CensorError(err error) string {
	toCensor := []string{
		common.BotSession.Token,
		common.ConfClientSecret.GetString(),
	}

	out := err.Error()
	for _, c := range toCensor {
		out = strings.Replace(out, c, "", -1)
	}

	return out
}

func BlockingAddRunningCommand(guildID int64, channelID int64, authorID int64, cmd *YAGCommand, timeout time.Duration) bool {
	started := time.Now()
	for {
		if tryAddRunningCommand(guildID, channelID, authorID, cmd) {
			return true
		}

		if time.Since(started) > timeout {
			return false
		}

		if atomic.LoadInt32(shuttingDown) == 1 {
			return false
		}

		time.Sleep(time.Second)

		if atomic.LoadInt32(shuttingDown) == 1 {
			return false
		}
	}
}

func tryAddRunningCommand(guildID int64, channelID int64, authorID int64, cmd *YAGCommand) bool {
	runningcommandsLock.Lock()
	for _, v := range runningCommands {
		if v.GuildID == guildID && v.ChannelID == channelID && v.AuthorID == authorID && v.Command == cmd {
			runningcommandsLock.Unlock()
			return false
		}
	}

	runningCommands = append(runningCommands, &RunningCommand{
		GuildID:   guildID,
		ChannelID: channelID,
		AuthorID:  authorID,

		Command: cmd,
	})

	runningcommandsLock.Unlock()

	return true
}

func removeRunningCommand(guildID, channelID, authorID int64, cmd *YAGCommand) {
	runningcommandsLock.Lock()
	for i, v := range runningCommands {
		if v.GuildID == guildID && v.ChannelID == channelID && v.AuthorID == authorID && v.Command == cmd {
			runningCommands = append(runningCommands[:i], runningCommands[i+1:]...)
			runningcommandsLock.Unlock()
			return
		}
	}

	runningcommandsLock.Unlock()
}

func (yc *YAGCommand) FindNameFromContainerChain(cc []*dcmd.Container) string {
	name := ""
	for _, v := range cc {
		if len(v.Names) < 1 {
			continue
		}

		if name != "" {
			name += " "
		}

		name += v.Names[0]
	}

	if name != "" {
		name += " "
	}

	return name + yc.Name
}

// GetAllOverrides returns all channel overrides and ensures the global override with atleast a default is present
func GetAllOverrides(ctx context.Context, guildID int64) ([]*models.CommandsChannelsOverride, error) {
	channelOverrides, err := models.CommandsChannelsOverrides(qm.Where("guild_id=?", guildID), qm.Load("CommandsCommandOverrides")).AllG(ctx)
	if err != nil {
		return nil, err
	}

	for _, v := range channelOverrides {
		if v.Global {
			return channelOverrides, nil
		}
	}

	global := &models.CommandsChannelsOverride{
		Global:          true,
		CommandsEnabled: true,
		AlwaysEphemeral: false,
	}
	global.R = global.R.NewStruct()

	channelOverrides = append(channelOverrides, global)

	return channelOverrides, nil
}
