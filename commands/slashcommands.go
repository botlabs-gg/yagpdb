package commands

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync/atomic"

	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/mediocregopher/radix/v3"
)

var (
	slashCommandsContainers []*slashCommandsContainer
	slashCommandsIdsSet     = new(int32)
)

type slashCommandsContainer struct {
	container          *dcmd.Container
	defaultPermissions bool
	rolesRunFunc       RolesRunFunc
	slashCommandID     int64
}

// register containers seperately as they need special handling
//
// note: we could infer all the info from the members of the container
// but i felt that this explicit method was better and less quirky
func RegisterSlashCommandsContainer(container *dcmd.Container, defaultPermissions bool, rolesRunFunc RolesRunFunc) {
	slashCommandsContainers = append(slashCommandsContainers, &slashCommandsContainer{
		container:          container,
		defaultPermissions: defaultPermissions,
		rolesRunFunc:       rolesRunFunc,
	})
}

func (p *Plugin) startSlashCommandsUpdater() {
	p.updateGlobalCommands()
}

func (p *Plugin) updateGlobalCommands() {
	result := make([]*discordgo.CreateApplicationCommandRequest, 0)

	for _, v := range CommandSystem.Root.Commands {
		if cmd := p.yagCommandToSlashCommand(v); cmd != nil {
			logger.Infof("%s is a global slash command: default enabled: %v", cmd.Name, cmd.DefaultPermission)
			result = append(result, cmd)
		}
	}

	for _, v := range slashCommandsContainers {
		logger.Infof("%s is a slash command container: default enabled: %v", v.container.Names[0], v.defaultPermissions)
		result = append(result, p.containerToSlashCommand(v))
	}

	encoded, _ := json.MarshalIndent(result, "", " ")

	current := ""
	err := common.RedisPool.Do(radix.Cmd(&current, "GET", "slash_commands_current"))
	if err != nil {
		logger.WithError(err).Error("failed retrieving current saved slash commands")
		return
	}

	if bytes.Equal([]byte(current), encoded) {
		logger.Info("Slash commands identical, skipping update")
		return
	}
	// fmt.Println(string(encoded))

	logger.Info("Slash commands changed, updating....")

	ret, err := common.BotSession.BulkOverwriteGlobalApplicationCommands(common.BotApplication.ID, result)
	// ret, err := common.BotSession.BulkOverwriteGuildApplicationCommands(common.BotApplication.ID, 614909558585819162, result)
	if err != nil {
		logger.WithError(err).Error("failed updating global slash commands")
		return
	}

	// assign the id's
OUTER:
	for _, v := range ret {
		for _, rs := range CommandSystem.Root.Commands {
			if cast, ok := rs.Command.(*YAGCommand); ok {
				if cast.SlashCommandEnabled && strings.EqualFold(v.Name, cast.Name) {
					cast.slashCommandID = v.ID
					continue OUTER
				}
			}
		}

		// top level command not found
		for _, c := range slashCommandsContainers {
			if strings.EqualFold(c.container.Names[0], v.Name) {
				c.slashCommandID = v.ID
				continue OUTER
			}
		}
	}

	atomic.StoreInt32(slashCommandsIdsSet, 1)

	err = common.RedisPool.Do(radix.Cmd(nil, "SET", "slash_commands_current", string(encoded)))
	if err != nil {
		logger.WithError(err).Error("failed setting current slash commands in redis")
	}
}

func (p *Plugin) containerToSlashCommand(container *slashCommandsContainer) *discordgo.CreateApplicationCommandRequest {
	t := true
	req := &discordgo.CreateApplicationCommandRequest{
		Name:                     strings.ToLower(container.container.Names[0]),
		Description:              common.CutStringShort(container.container.Description, 100),
		DefaultPermission:        &t,
		DefaultMemberPermissions: containerDefaultMemberPermissions(container.container),
	}

	for _, v := range container.container.Commands {
		cast, ok := v.Command.(*YAGCommand)
		if !ok {
			panic("Not a yag command? what is this a triple nested command or something?")
		}

		isSub, innerOpts := cast.slashCommandOptions()
		kind := discordgo.ApplicationCommandOptionSubCommand
		if isSub {
			kind = discordgo.ApplicationCommandOptionSubCommandGroup
		}

		opt := &discordgo.ApplicationCommandOption{
			Name:        strings.ToLower(cast.Name),
			Description: common.CutStringShort(cast.Description, 100),
			Type:        kind,
			Options:     innerOpts,
		}

		req.Options = append(req.Options, opt)
	}

	return req
}

func (p *Plugin) yagCommandToSlashCommand(cmd *dcmd.RegisteredCommand) *discordgo.CreateApplicationCommandRequest {

	cast, ok := cmd.Command.(*YAGCommand)
	if !ok {
		// probably a container, which is handled seperately, see RegisterSlashCommandsContainer
		return nil
	}

	if !cast.SlashCommandEnabled {
		// not enabled for slash commands
		return nil
	}
	t := true

	_, opts := cast.slashCommandOptions()
	return &discordgo.CreateApplicationCommandRequest{
		Name:                     strings.ToLower(cmd.Trigger.Names[0]),
		Description:              common.CutStringShort(cast.Description, 100),
		DefaultPermission:        &t,
		Options:                  opts,
		NSFW:                     cast.NSFW,
		DefaultMemberPermissions: slashCommandDefaultMemberPermissions(cast),
	}
}

// slashCommandDefaultMemberPermissions maps RequireDiscordPerms onto what discord
// will hide a command behind.
//
// RequireDiscordPerms is satisfied by any one of its entries, while discord
// requires every bit of default_member_permissions, so only a single unambiguous
// permission can be advertised. Anything else stays visible and is left to the
// checks done when the command actually runs.
func slashCommandDefaultMemberPermissions(yc *YAGCommand) *int64 {
	if len(yc.RequireDiscordPerms) != 1 {
		return nil
	}

	perms := yc.RequireDiscordPerms[0]
	if perms == 0 {
		return nil
	}

	return &perms
}

// containerDefaultMemberPermissions returns the permissions every subcommand
// agrees on, since discord only accepts one value for the whole container.
func containerDefaultMemberPermissions(container *dcmd.Container) *int64 {
	var common int64
	for i, v := range container.Commands {
		cast, ok := v.Command.(*YAGCommand)
		if !ok {
			return nil
		}

		perms := slashCommandDefaultMemberPermissions(cast)
		if perms == nil {
			return nil
		}

		if i == 0 {
			common = *perms
			continue
		}

		common &= *perms
	}

	if common == 0 {
		return nil
	}

	return &common
}

// IsInbuiltSlashCommandName checks if the provided name is already
// a built-in global slash command
func IsInbuiltSlashCommandName(name string) bool {
	name = strings.ToLower(name)
	if CommandSystem == nil || CommandSystem.Root == nil {
		return false
	}

	for _, v := range CommandSystem.Root.Commands {
		if cast, ok := v.Command.(*YAGCommand); ok {
			if cast.SlashCommandEnabled && strings.EqualFold(v.Trigger.Names[0], name) {
				return true
			}
		}
	}

	for _, c := range slashCommandsContainers {
		if strings.EqualFold(c.container.Names[0], name) {
			return true
		}
	}

	return false
}

func (yc *YAGCommand) slashCommandOptions() (turnedIntoSubCommands bool, result []*discordgo.ApplicationCommandOption) {

	var subCommands []*discordgo.ApplicationCommandOption

	for i, v := range yc.Arguments {

		opts := v.Type.SlashCommandOptions(v)
		for _, v := range opts {
			v.Name = strings.ToLower(v.Name)
		}

		if len(opts) > 1 && i == 0 {
			// turn this command into a container
			turnedIntoSubCommands = true
			kind := discordgo.ApplicationCommandOptionSubCommand

			for _, opt := range opts {
				if i < yc.RequiredArgs {
					opt.Required = true
				}

				subCommands = append(subCommands, &discordgo.ApplicationCommandOption{
					Type:        kind,
					Name:        "by-" + opt.Name,
					Description: common.CutStringShort(yc.Description, 100),
					Options: []*discordgo.ApplicationCommandOption{
						opt,
					},
				})
			}

			turnedIntoSubCommands = true
		} else {
			if len(opts) == 1 {
				if i < yc.RequiredArgs {
					opts[0].Required = true
				}
			}

			result = append(result, opts...)
		}
	}

	sortedResult := make([]*discordgo.ApplicationCommandOption, 0, len(result))

	// required args needs to be first
	for _, v := range result {
		if v.Required {
			sortedResult = append(sortedResult, v)
		}
	}

	// add the optional args last
	for _, v := range result {
		if !v.Required {
			sortedResult = append(sortedResult, v)
		}
	}

	for _, v := range yc.ArgSwitches {
		if v.Type == nil {
			adding := v.StandardSlashCommandOption(discordgo.ApplicationCommandOptionBoolean)
			adding.Name = strings.ToLower(adding.Name)
			sortedResult = append(sortedResult, adding)
		} else {
			adding := v.Type.SlashCommandOptions(v)
			for _, v := range adding {
				v.Name = strings.ToLower(v.Name)
			}
			sortedResult = append(sortedResult, adding...)
		}
	}

	if turnedIntoSubCommands {
		for _, v := range subCommands {
			v.Options = append(v.Options, sortedResult...)
		}

		return true, subCommands
	} else {
		return false, sortedResult
	}
}

func handleInteractionCreate(evt *eventsystem.EventData) {
	interaction := evt.InteractionCreate()
	if interaction.Type != discordgo.InteractionApplicationCommand && interaction.Type != discordgo.InteractionApplicationCommandAutocomplete {
		return
	}
	if interaction.DataCommand == nil {
		logger.Warn("Interaction had no data")
		return
	}

	//if guildID is present then this is a guild custom slash cmmand
	if interaction.DataCommand.GuildID != 0 {
		return
	}

	// serialized, _ := json.MarshalIndent(interaction.Interaction, "", "  ")
	// logger.Infof("Got interaction %#v", interaction.Interaction)
	// fmt.Println(string(serialized))

	err := CommandSystem.CheckInteraction(common.BotSession, &interaction.Interaction)
	if err != nil {
		logger.WithError(err).Error("failed handling command interaction")
	}
}
