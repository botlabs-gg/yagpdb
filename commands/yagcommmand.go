package commands

import (
	"context"
	"fmt"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands/models"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"strings"
	"time"
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
	RKeyCommandCooldown = func(uID int64, cmd string) string { return "cmd_cd:" + discordgo.StrID(uID) + ":" + cmd }
	RKeyCommandLock     = func(uID int64, cmd string) string { return "cmd_lock:" + discordgo.StrID(uID) + ":" + cmd }

	CommandExecTimeout = time.Minute
)

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

	Cooldown    int // Cooldown in seconds before user can use it again
	CmdCategory *dcmd.Category

	RunInDM      bool // Set to enable this commmand in DM's
	HideFromHelp bool // Set to hide from help

	RequireDiscordPerms []int64 // Require users to have one of these permission sets to run the command

	// Run is ran the the command has sucessfully been parsed
	// It returns a reply and an error
	// the reply can have a type of string, *MessageEmbed or error
	RunFunc dcmd.RunFunc
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

func (yc *YAGCommand) Run(data *dcmd.Data) (interface{}, error) {
	if !yc.RunInDM && data.Source == dcmd.DMSource {
		return nil, nil
	}

	// Send typing to indicate the bot's working
	common.BotSession.ChannelTyping(data.Msg.ChannelID)

	logger := yc.Logger(data)

	// Track how long execution of a command took
	started := time.Now()
	defer func() {
		yc.logExecutionTime(time.Since(started), data.Msg.Content, data.Msg.Author.Username)
	}()

	cState := data.CS

	// Set up log entry for later use
	logEntry := &common.LoggedExecutedCommand{
		UserID:    discordgo.StrID(data.Msg.Author.ID),
		ChannelID: discordgo.StrID(data.Msg.ChannelID),

		Command:    yc.Name,
		RawCommand: data.Msg.Content,
		TimeStamp:  time.Now(),
	}

	if cState != nil && cState.Guild != nil {
		logEntry.GuildID = discordgo.StrID(cState.Guild.ID)
	}

	if common.Statsd != nil {
		go common.Statsd.Incr("yagpdb.cmd.executed", nil, 1)
	}

	logger.Info("Handling command: " + data.Msg.Content)

	runCtx, cancelExec := context.WithTimeout(data.Context(), CommandExecTimeout)
	defer cancelExec()

	// Run the command
	r, cmdErr := yc.RunFunc(data.WithContext(runCtx))
	if cmdErr != nil {
		if errors.Cause(cmdErr) == context.Canceled || errors.Cause(cmdErr) == context.DeadlineExceeded {
			r = "Took longer than " + CommandExecTimeout.String() + " to handle command: `" + common.EscapeSpecialMentions(data.Msg.Content) + "`, Cancelled the command."
		}
	}

	if (r == nil || r == "") && cmdErr != nil {
		r = yc.HumanizeError(cmdErr)
	}

	logEntry.ResponseTime = int64(time.Since(started))

	// Log errors
	if cmdErr == nil {
		err := yc.SetCooldown(data.Msg.Author.ID)
		if err != nil {
			logger.WithError(err).Error("Failed setting cooldown")
		}
	}

	// Create command log entry
	err := common.GORM.Create(logEntry).Error
	if err != nil {
		logger.WithError(err).Error("Failed creating command execution log")
	}

	return r, cmdErr
}

func (yc *YAGCommand) HumanizeError(err error) string {
	cause := errors.Cause(err)

	if pe, ok := cause.(PublicError); ok {
		return "The command returned an error: " + pe.Error()
	}

	if dErr, ok := cause.(*discordgo.RESTError); ok && dErr.Message != nil && dErr.Message.Message != "" {
		if dErr.Response != nil && dErr.Response.StatusCode == 403 {
			return "The bot permissions has been incorrectly set up on this server for it to run this command: " + dErr.Message.Message
		}

		return "The bot was not able to perform the action, discord responded with: " + dErr.Message.Message
	}

	return "Something went wrong when running this command, either discord or the bot may be having issues."
}

// PostCommandExecuted sends the response and handles the trigger and response deletions
func (yc *YAGCommand) PostCommandExecuted(settings *CommandSettings, cmdData *dcmd.Data, resp interface{}, err error) {
	if err != nil {
		yc.Logger(cmdData).WithError(err).Error("Command returned error")
	}

	if cmdData.GS != nil {
		if resp == nil && err != nil {
			err = errors.New(FilterResp(err.Error(), cmdData.GS.ID).(string))
		} else if resp != nil {
			resp = FilterResp(resp, cmdData.GS.ID)
		}
	}

	if settings.DelResponse && settings.DelResponseDelay < 1 {
		// Set up the trigger deletion if set
		if settings.DelTrigger {
			go func() {
				time.Sleep(time.Duration(settings.DelTriggerDelay) * time.Second)
				common.BotSession.ChannelMessageDelete(cmdData.CS.ID, cmdData.Msg.ID)
			}()
		}
		return // Don't bother sending the reponse if it has no delete delay
	}

	// Use the error as the response if no response was provided
	if resp == nil && err != nil {
		resp = fmt.Sprintf("%q command returned an error: %s", cmdData.Cmd.FormatNames(false, "/"), err)
	}

	// send a alternative message in case of embeds in channels with no embeds perms
	if cmdData.GS != nil {
		switch resp.(type) {
		case *discordgo.MessageEmbed, []*discordgo.MessageEmbed:
			if !bot.BotProbablyHasPermissionGS(true, cmdData.GS, cmdData.CS.ID, discordgo.PermissionEmbedLinks) {
				resp = "This command returned an embed but the bot does not have embed links permissions in this channel, cannot send the response."
			}
		}
	}

	// Send the response
	var replies []*discordgo.Message
	if resp != nil {
		replies, err = dcmd.SendResponseInterface(cmdData, resp, true)
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
			if settings.DelTrigger && settings.DelTriggerDelay == settings.DelResponseDelay {
				ids = append(ids, cmdData.Msg.ID)
			}

			if len(ids) == 1 {
				common.BotSession.ChannelMessageDelete(cmdData.CS.ID, ids[0])
			} else if len(ids) > 1 {
				common.BotSession.ChannelMessagesBulkDelete(cmdData.CS.ID, ids)
			}
		}()
	}

	// If were deleting the trigger in a seperate call from the response deletion
	if settings.DelTrigger && (!settings.DelResponse || settings.DelTriggerDelay != settings.DelResponseDelay) {
		go func() {
			time.Sleep(time.Duration(settings.DelTriggerDelay) * time.Second)
			common.BotSession.ChannelMessageDelete(cmdData.CS.ID, cmdData.Msg.ID)
		}()
	}

	return
}

// checks if the specified user can execute the command, and if so returns the settings for said command
func (yc *YAGCommand) checkCanExecuteCommand(data *dcmd.Data, cState *dstate.ChannelState) (canExecute bool, resp string, settings *CommandSettings, err error) {
	// Check guild specific settings if not triggered from a DM
	var guild *dstate.GuildState

	if data.Source != dcmd.DMSource {

		canExecute = false
		guild = cState.Guild

		if guild == nil {
			resp = "You're not on a server?"
			return
		}

		if !bot.BotProbablyHasPermissionGS(true, guild, cState.ID, discordgo.PermissionReadMessages|discordgo.PermissionSendMessages) {
			return
		}

		cop := cState.Copy(true)

		settings, err = yc.GetSettings(data.ContainerChain, cState.ID, cop.ParentID, guild.ID)
		if err != nil {
			err = errors.WithMessage(err, "cs.GetSettings")
			resp = "Bot is having isssues, contact the bot owner."
			return
		}

		if !settings.Enabled {
			resp = fmt.Sprintf("The %q command is currently disabled on this server or channel. *(Control panel to enable/disable <https://%s>)*", yc.Name, common.Conf.Host)
			return
		}

		member := ContextMS(data.Context())
		// Check the required and ignored roles
		if len(settings.RequiredRoles) > 0 {
			found := false
			for _, r := range member.Roles {
				if common.ContainsInt64Slice(settings.RequiredRoles, r) {
					found = true
					break
				}
			}

			if !found {
				resp = "Missing a required role set up by the server admins for this command."
				return
			}
		}

		for _, ignored := range settings.IgnoreRoles {
			if common.ContainsInt64Slice(member.Roles, ignored) {
				resp = "One of your roles is set up to be ignored by the server admins."
				return
			}
		}

		// This command has permission sets required, if the user has one of them then allow this command to be used
		if len(yc.RequireDiscordPerms) > 0 {
			var perms int
			perms, err = cState.Guild.MemberPermissionsMS(true, cState.ID, member)
			if err != nil {
				resp = "Unable to check permissions"
				return
			}

			foundMatch := false
			for _, permSet := range yc.RequireDiscordPerms {
				if permSet&int64(perms) == permSet {
					foundMatch = true
					break
				}
			}

			if !foundMatch {
				resp = "Missing required permissions to use this command (" + yc.humanizedRequiredPerms() + ")"
				return
			}
		}
	} else {
		settings = &CommandSettings{
			Enabled: true,
		}
	}

	// Check the command cooldown
	cdLeft, err := yc.CooldownLeft(data.Msg.Author.ID)
	if err != nil {
		// Just pretend the cooldown is off...
		log.WithError(err).WithField("author", data.Msg.Author.ID).Error("Failed checking command cooldown")
	}

	if cdLeft > 0 {
		resp = fmt.Sprintf("**%q:** You need to wait %d seconds before you can use the %q command again", common.EscapeSpecialMentions(data.Msg.Author.Username), cdLeft, yc.Name)
		return
	}

	// If we got here then we can execute the command
	canExecute = true
	return
}

func (yc *YAGCommand) humanizedRequiredPerms() string {
	res := ""
	for i, permSet := range yc.RequireDiscordPerms {
		if i != 0 {
			res += " or "
		}
		res += "`" + strings.Join(common.HumanizePermissions(permSet), "+") + "`"
	}

	return res
}

func (cs *YAGCommand) logExecutionTime(dur time.Duration, raw string, sender string) {
	log.Infof("Handled Command [%4dms] %s: %s", int(dur.Seconds()*1000), sender, raw)
}

func (cs *YAGCommand) deleteResponse(msgs []*discordgo.Message) {
	ids := make([]int64, 0, len(msgs))
	var cID int64
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		cID = msg.ChannelID
		ids = append(ids, msg.ID)
	}

	if len(ids) < 1 {
		return // ...
	}

	time.Sleep(time.Second * 10)

	// Either do a bulk delete or single delete depending on how big the response was
	if len(ids) > 1 {
		common.BotSession.ChannelMessagesBulkDelete(cID, ids)
	} else {
		common.BotSession.ChannelMessageDelete(cID, ids[0])
	}
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
	Enabled bool

	DelTrigger       bool
	DelResponse      bool
	DelTriggerDelay  int
	DelResponseDelay int

	RequiredRoles []int64
	IgnoreRoles   []int64
}

func GetOverridesForChannel(channelID, channelParentID, guildID int64) ([]*models.CommandsChannelsOverride, error) {
	// Fetch the overrides from the database, we treat the global settings as an override for simplicity
	channelOverrides, err := models.CommandsChannelsOverrides(qm.Where("(? = ANY (channels) OR global=true OR ? = ANY (channel_categories)) AND guild_id=?", channelID, channelParentID, guildID), qm.Load("CommandsCommandOverrides")).AllG(context.Background())
	if err != nil {
		return nil, err
	}

	return channelOverrides, nil
}

// GetSettings returns the settings from the command, generated from the servers channel and command overrides
func (cs *YAGCommand) GetSettings(containerChain []*dcmd.Container, channelID, channelParentID, guildID int64) (settings *CommandSettings, err error) {

	// Fetch the overrides from the database, we treat the global settings as an override for simplicity
	channelOverrides, err := GetOverridesForChannel(channelID, channelParentID, guildID)
	if err != nil {
		err = errors.WithMessage(err, "GetOverridesForChannel")
		return
	}

	return cs.GetSettingsWithLoadedOverrides(containerChain, guildID, channelOverrides)
}

func (cs *YAGCommand) GetSettingsWithLoadedOverrides(containerChain []*dcmd.Container, guildID int64, channelOverrides []*models.CommandsChannelsOverride) (settings *CommandSettings, err error) {
	settings = &CommandSettings{}

	// Some commands have custom places to toggle their enabled status
	ce, err := cs.customEnabled(guildID)
	if err != nil {
		err = errors.WithMessage(err, "customEnabled")
		return
	}

	if !ce {
		return
	}

	if cs.HideFromCommandsPage {
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

	cmdFullName := cs.Name
	if len(containerChain) > 1 {
		lastContainer := containerChain[len(containerChain)-1]
		cmdFullName = lastContainer.Names[0] + " " + cmdFullName
	}

	// Assign the global settings, if existing
	if global != nil {
		cs.fillSettings(cmdFullName, global, settings)
	}

	// Assign the channel override, if existing
	if channelOverride != nil {
		cs.fillSettings(cmdFullName, channelOverride, settings)
	}

	return
}

// Fills the command settings from a channel override, and if a matching command override is found, the command override
func (cs *YAGCommand) fillSettings(cmdFullName string, override *models.CommandsChannelsOverride, settings *CommandSettings) {
	settings.Enabled = override.CommandsEnabled

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

// CooldownLeft returns the number of seconds before a command can be used again
func (cs *YAGCommand) CooldownLeft(userID int64) (int, error) {
	if cs.Cooldown < 1 || common.Testing {
		return 0, nil
	}

	var ttl int
	err := common.RedisPool.Do(radix.Cmd(&ttl, "TTL", RKeyCommandCooldown(userID, cs.Name)))
	if ttl < 1 {
		return 0, nil
	}

	return ttl, err
}

// SetCooldown sets the cooldown of the command as it's defined in the struct
func (cs *YAGCommand) SetCooldown(userID int64) error {
	if cs.Cooldown < 1 {
		return nil
	}
	now := time.Now().Unix()

	err := common.RedisPool.Do(radix.FlatCmd(nil, "SET", RKeyCommandCooldown(userID, cs.Name), now, "EX", cs.Cooldown))
	return err
}

func (yc *YAGCommand) Logger(data *dcmd.Data) *log.Entry {
	l := log.WithField("cmd", yc.Name)
	if data != nil {
		if data.Msg != nil {
			l = l.WithField("user_n", data.Msg.Author.Username)
			l = l.WithField("user_id", data.Msg.Author.ID)
		}

		if data.CS != nil {
			l = l.WithField("channel", data.CS.ID)
		}
	}

	return l
}

func (yc *YAGCommand) GetTrigger() *dcmd.Trigger {
	trigger := dcmd.NewTrigger(yc.Name, yc.Aliases...).SetDisableInDM(!yc.RunInDM)
	trigger = trigger.SetHideFromHelp(yc.HideFromHelp)
	return trigger
}

// Keys and other sensitive information shouldnt be sent in error messages, but just in case it is
func CensorError(err error) string {
	toCensor := []string{
		common.BotSession.Token,
		common.Conf.ClientSecret,
	}

	out := err.Error()
	for _, c := range toCensor {
		out = strings.Replace(out, c, "", -1)
	}

	return out
}
