package commands

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/common"
	prfx "github.com/botlabs-gg/yagpdb/v2/common/prefix"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
)

var (
	CommandSystem *dcmd.System
)

var _ bot.BotInitHandler = (*Plugin)(nil)
var _ bot.BotStopperHandler = (*Plugin)(nil)

func (p *Plugin) BotInit() {
	eventsystem.AddHandlerAsyncLastLegacy(p, handleMsgCreate, eventsystem.EventMessageCreate)
	eventsystem.AddHandlerAsyncLastLegacy(p, handleInteractionCreate, eventsystem.EventInteractionCreate)

	// Slash command permissions are currently pretty fucked so can't use them
	//
	// eventsystem.AddHandlerAsyncLastLegacy(p, p.handleGuildCreate, eventsystem.EventGuildCreate)
	// eventsystem.AddHandlerAsyncLastLegacy(p, p.handleDiscordEventUpdateSlashCommandPermissions, eventsystem.EventGuildRoleCreate, eventsystem.EventGuildRoleUpdate, eventsystem.EventChannelCreate)
	// pubsub.AddHandler("update_slash_command_permissions", p.handleUpdateSlashCommandsPermissions, nil)

	CommandSystem.State = bot.State
	dcmd.CustomUsernameSearchFunc = p.customUsernameSearchFunc

	// err := clearGlobalCommands()
	// if err != nil {
	// 	logger.WithError(err).Errorf("failed clearing all commands")
	// }
	p.startSlashCommandsUpdater()

}

func (p *Plugin) customUsernameSearchFunc(tracker dstate.StateTracker, gs *dstate.GuildSet, query string) (ms *dstate.MemberState, err error) {
	logger.Info("Searching by username: ", query)
	members, err := bot.BatchMemberJobManager.SearchByUsername(gs.ID, query)
	if err != nil {
		if err == bot.ErrTimeoutWaitingForMember {
			return nil, &dcmd.UserNotFound{Part: query}
		}

		return nil, err
	}

	lowerIn := strings.ToLower(query)

	partialMatches := make([]*discordgo.Member, 0, 5)
	fullMatches := make([]*discordgo.Member, 0, 5)

	// filter out the results
	for _, v := range members {
		if v == nil {
			continue
		}

		if v.User.Username == "" {
			continue
		}

		if strings.EqualFold(query, v.User.Username) || strings.EqualFold(query, v.Nick) {
			fullMatches = append(fullMatches, v)
			if len(fullMatches) >= 5 {
				break
			}
		} else if len(partialMatches) < 5 {
			if strings.Contains(strings.ToLower(v.User.Username), lowerIn) {
				partialMatches = append(partialMatches, v)
			}
		}
	}

	if len(fullMatches) == 1 {
		return dstate.MemberStateFromMember(fullMatches[0]), nil
	}

	if len(fullMatches) == 0 && len(partialMatches) == 0 {
		return nil, &dcmd.UserNotFound{Part: query}
	}

	// Show some help output
	out := ""

	if len(fullMatches)+len(partialMatches) < 10 {
		for _, v := range fullMatches {
			if out != "" {
				out += ", "
			}

			out += "`" + v.User.Username + "`"
		}

		for _, v := range partialMatches {
			if out != "" {
				out += ", "
			}

			out += "`" + v.User.Username + "`"
		}
	} else {
		return nil, &dcmd.UserNotFound{Part: query}
	}

	if len(fullMatches) > 1 {
		return nil, dcmd.NewSimpleUserError("Too many users with the name: (" + out + ") Please re-run the command with a narrower search, mention or ID.")
	}

	return nil, dcmd.NewSimpleUserError("Did you mean one of these? (" + out + ") Please re-run the command with a narrower search, mention or ID")
}

func (p *Plugin) StopBot(wg *sync.WaitGroup) {
	atomic.StoreInt32(shuttingDown, 1)

	startedWaiting := time.Now()
	for {
		runningcommandsLock.Lock()
		n := len(runningCommands)
		runningcommandsLock.Unlock()

		if n < 1 {
			wg.Done()
			return
		}

		if time.Since(startedWaiting) > time.Second*60 {
			// timeout
			logger.Infof("[commands] timeout waiting for %d commands to finish running (d=%s)", n, time.Since(startedWaiting))
			wg.Done()
			return
		}

		logger.Infof("[commands] waiting for %d commands to finish running (d=%s)", n, time.Since(startedWaiting))
		time.Sleep(time.Millisecond * 500)
	}
}

var helpFormatter = &dcmd.StdHelpFormatter{}

func YAGCommandMiddleware(inner dcmd.RunFunc) dcmd.RunFunc {
	return func(data *dcmd.Data) (interface{}, error) {
		yc, ok := data.Cmd.Command.(*YAGCommand)
		if !ok {
			resp, err := inner(data)
			// Filter the response
			if data.GuildData != nil {
				if resp == nil && err != nil {
					err = errors.New(FilterResp(err.Error(), data.GuildData.GS.ID).(string))
				} else if resp != nil {
					resp = FilterResp(resp, data.GuildData.GS.ID)
				}
			}

			return resp, err
		}
		guildID := int64(0)
		if data.GuildData != nil {
			guildID = data.GuildData.GS.ID
		}

		// Check if the user can execute the command
		canExecute, resp, settings, err := yc.checkCanExecuteCommand(data)
		if err != nil {
			yc.Logger(data).WithError(err).Error("An error occured while checking if we could run command")
		}

		if data.TriggerType == dcmd.TriggerTypeSlashCommands && data.SlashCommandTriggerData.Interaction.Type != discordgo.InteractionApplicationCommandAutocomplete {
			// Acknowledge the interaction
			response := discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			}
			if yc.IsResponseEphemeral || settings.AlwaysEphemeral {
				response.Data = &discordgo.InteractionResponseData{Flags: 64}
			}
			err := data.Session.CreateInteractionResponse(data.SlashCommandTriggerData.Interaction.ID, data.SlashCommandTriggerData.Interaction.Token, &response)
			if err != nil {
				return nil, err
			}
		}

		parseErr := dcmd.ParseCmdArgs(data)
		// Stop here if it's autocomplete; we don't need to block it
		if data.TriggerType == dcmd.TriggerTypeSlashCommands && data.SlashCommandTriggerData.Interaction.Type == discordgo.InteractionApplicationCommandAutocomplete {
			if parseErr != nil {
				return nil, parseErr
			}
			choices, err := inner(data)
			yc.PostCommandExecuted(settings, data, choices, err)
			return nil, nil
		}

		// Lock the command for execution
		if !BlockingAddRunningCommand(guildID, data.ChannelID, data.Author.ID, yc, time.Second*60) {
			if atomic.LoadInt32(shuttingDown) == 1 {
				return &EphemeralOrGuild{Content: yc.Name + ": Bot is restarting, please try again in a couple seconds..."}, nil
			}

			return &EphemeralOrGuild{Content: yc.Name + ": Gave up trying to run command after 60 seconds waiting for your previous instance of this command to finish"}, nil
		}

		defer removeRunningCommand(guildID, data.ChannelID, data.Author.ID, yc)

		if resp != nil {

			if resp.Type == ReasonCooldown && data.TriggerType != dcmd.TriggerTypeSlashCommands && data.GuildData != nil {
				if hasPerms, _ := bot.BotHasPermissionGS(data.GuildData.GS, data.GuildData.CS.ID, discordgo.PermissionAddReactions); hasPerms {
					common.BotSession.MessageReactionAdd(data.ChannelID, data.TraditionalTriggerData.Message.ID, "⏳")
					return nil, nil
				}
			}

			switch resp.Type {
			case ReasonBotMissingPerms:
				return &EphemeralOrGuild{
					Content: "You're unable to run this command:\n> " + resp.Message,
				}, nil
			default:
				return &EphemeralOrNone{
					Content: "You're unable to run this command:\n> " + resp.Message,
				}, nil
			}
		}

		if !canExecute {
			return &EphemeralOrNone{
				Content: "You're unable to run this command.",
			}, nil
		}

		if err != nil {
			return nil, err
		}

		data = data.WithContext(context.WithValue(data.Context(), CtxKeyCmdSettings, settings))

		if parseErr != nil {
			if dcmd.IsUserError(parseErr) {

				args := helpFormatter.ArgDefs(data.Cmd, data)
				switches := helpFormatter.Switches(data.Cmd.Command)

				resp := ""
				if args != "" {
					resp += "```\n" + args + "\n```"
				}
				if switches != "" {
					resp += "```\n" + switches + "\n```"
				}

				resp = resp + "\nInvalid arguments provided: " + parseErr.Error()
				yc.PostCommandExecuted(settings, data, &EphemeralOrGuild{
					Content: resp,
				}, nil)
				return nil, nil
			}

			return nil, parseErr
		}

		innerResp, err := inner(data)

		// Send the response
		yc.PostCommandExecuted(settings, data, innerResp, err)

		return nil, nil
	}
}

func FilterResp(in interface{}, guildID int64) interface{} {
	switch t := in.(type) {
	case string:
		return FilterBadInvites(t, guildID, "[removed-invite]")
	case error:
		return FilterBadInvites(t.Error(), guildID, "[removed-invite]")
	}

	return in
}

func AddRootCommands(p common.Plugin, cmds ...*YAGCommand) {
	for _, v := range cmds {
		v.Plugin = p
		CommandSystem.Root.AddCommand(v, v.GetTrigger())
	}
}

func AddRootCommandsWithMiddlewares(p common.Plugin, middlewares []dcmd.MiddleWareFunc, cmds ...*YAGCommand) {
	for _, v := range cmds {
		v.Plugin = p
		CommandSystem.Root.AddCommand(v, v.GetTrigger().SetMiddlewares(middlewares...))
	}
}

func handleMsgCreate(evt *eventsystem.EventData) {
	m := evt.MessageCreate()
	if !bot.IsNormalUserMessage(m.Message) {
		// Pls no panicerinos or banerinos self, also ignore webhooks
		return
	}

	abort := false
	for _, filterFunc := range MessageFilterFuncs {
		if !filterFunc(evt, m.Message) {
			abort = true
		}
	}

	if abort {
		return
	}

	prefix := prfx.DefaultCommandPrefix()
	if evt.GS != nil && evt.HasFeatureFlag(featureFlagHasCustomPrefix) {
		var err error
		prefix, err = prfx.GetCommandPrefixRedis(evt.GS.ID)
		if err != nil {
			logger.WithError(err).WithField("guild", evt.GS.ID).Error("failed fetching command prefix")
		}
	}

	CommandSystem.CheckMessageWtihPrefetchedPrefix(common.BotSession, evt.MessageCreate(), prefix)
	// CommandSystem.HandleMessageCreate(common.BotSession, evt.MessageCreate())
}

func GetCommandPrefixBotEvt(evt *eventsystem.EventData) (string, error) {
	prefix := prfx.DefaultCommandPrefix()
	if evt.GS != nil && evt.HasFeatureFlag(featureFlagHasCustomPrefix) {
		var err error
		prefix, err = prfx.GetCommandPrefixRedis(evt.GS.ID)
		return prefix, err
	}

	return prefix, nil
}

func (p *Plugin) Prefix(data *dcmd.Data) string {
	if data.Source == dcmd.TriggerSourceDM {
		return "-"
	}

	prefix, err := prfx.GetCommandPrefixRedis(data.GuildData.GS.ID)
	if err != nil {
		logger.WithError(err).Error("Failed retrieving commands prefix")
	}

	return prefix
}

func ensureEmbedLimits(embed *discordgo.MessageEmbed) {
	if utf8.RuneCountInString(embed.Description) < 2000 {
		return
	}

	lines := strings.Split(embed.Description, "\n")

	firstField := &discordgo.MessageEmbedField{
		Name: "Commands",
	}

	currentField := firstField
	for _, v := range lines {
		if utf8.RuneCountInString(currentField.Value)+utf8.RuneCountInString(v) >= 1024 {
			currentField = &discordgo.MessageEmbedField{
				Name:  "...",
				Value: v + "\n",
			}
			embed.Fields = append(embed.Fields, currentField)
		} else {
			currentField.Value += v + "\n"
		}
	}

	embed.Description = firstField.Value
}

var cmdPrefix = &YAGCommand{
	Name:        "Prefix",
	Description: "Shows command prefix of the current server, or the specified server",
	CmdCategory: CategoryTool,
	Arguments: []*dcmd.ArgDef{
		{Name: "Server-ID", Type: dcmd.BigInt, Default: 0},
	},

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		targetGuildID := data.Args[0].Int64()
		if targetGuildID == 0 {
			targetGuildID = data.GuildData.GS.ID
		}

		prefix, err := prfx.GetCommandPrefixRedis(targetGuildID)
		if err != nil {
			return nil, err
		}

		return fmt.Sprintf("Prefix of `%d`: `%s`", targetGuildID, prefix), nil
	},
}

func clearGlobalCommands() error {
	commands, err := common.BotSession.GetGlobalApplicationCommands(common.BotApplication.ID)
	if err != nil {
		return err
	}
	logger.Info("COMMANDS LENGTH: ", len(commands))
	for _, v := range commands {
		err = common.BotSession.DeleteGlobalApplicationCommand(common.BotApplication.ID, v.ID)
		if err != nil {
			return err
		}
	}

	logger.Info("DONE")
	return nil
}
