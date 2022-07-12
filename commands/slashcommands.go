package commands

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/commands/models"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/pubsub"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
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
		Name:              strings.ToLower(container.container.Names[0]),
		Description:       common.CutStringShort(container.container.Description, 100),
		DefaultPermission: &t,
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
		Name:              strings.ToLower(cmd.Trigger.Names[0]),
		Description:       common.CutStringShort(cast.Description, 100),
		DefaultPermission: &t,
		Options:           opts,
	}
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

func (p *Plugin) handleGuildCreate(evt *eventsystem.EventData) {
	// TODO: add queue?
	waitForSlashCommandIDs()

	gs := bot.State.GetGuild(evt.GuildCreate().ID)
	if gs == nil {
		panic("gs is nil")
	}

	_, err := updateSlashCommandGuildPermissions(gs)
	if err != nil {
		logger.WithError(err).Error("failed updating guild slash command permissions")
	}
}

func (p *Plugin) handleDiscordEventUpdateSlashCommandPermissions(evt *eventsystem.EventData) {
	if evt.GS == nil {
		return
	}

	waitForSlashCommandIDs()

	_, err := updateSlashCommandGuildPermissions(evt.GS)
	if err != nil {
		logger.WithError(err).Error("failed updating guild slash command permissions")
	}
}

func updateSlashCommandGuildPermissions(gs *dstate.GuildSet) (updated bool, err error) {
	commandSettings, err := GetAllOverrides(context.Background(), gs.ID)
	if err != nil {
		return false, err
	}

	result := make([]*discordgo.GuildApplicationCommandPermissions, 0)

	// Start with root commands
	for _, v := range CommandSystem.Root.Commands {
		if cast, ok := v.Command.(*YAGCommand); ok {
			if cast.SlashCommandEnabled {
				perms, err := cast.TopLevelSlashCommandPermissions(commandSettings, gs)
				if err != nil {
					return false, err
				}
				result = append(result, &discordgo.GuildApplicationCommandPermissions{
					ID:            cast.slashCommandID,
					ApplicationID: common.BotApplication.ID,
					GuildID:       gs.ID,
					Permissions:   perms,
				})
			}
		}
	}

	// do the containers
	for _, v := range slashCommandsContainers {
		perms, err := ContainerSlashCommandPermissions(v, commandSettings, gs)
		if err != nil {
			return false, err
		}
		result = append(result, &discordgo.GuildApplicationCommandPermissions{
			ID:            v.slashCommandID,
			ApplicationID: common.BotApplication.ID,
			GuildID:       gs.ID,
			Permissions:   perms,
		})
	}

	serialized, _ := json.MarshalIndent(result, ">", "  ")
	fmt.Println(string(serialized))
	hash := sha256.Sum256(serialized)
	oldHash := []byte{}
	err = common.RedisPool.Do(radix.Cmd(&oldHash, "GET", fmt.Sprintf("slash_cmd_perms_sum:%d", gs.ID)))
	if err != nil {
		return false, err
	}

	fmt.Println("Hash: ", hash, "old", oldHash)
	if bytes.Equal(hash[:], oldHash) {
		logger.Info("Skipped updating guild slash command perms, hash matched ", gs.ID)
		return false, nil
	}

	ret, err := common.BotSession.BatchEditGuildApplicationCommandsPermissions(common.BotApplication.ID, gs.ID, result)
	if err != nil {
		return false, err
	}

	err = common.RedisPool.Do(radix.FlatCmd(nil, "SET", fmt.Sprintf("slash_cmd_perms_sum:%d", gs.ID), hash[:]))
	if err != nil {
		return false, err
	}

	serialized, _ = json.MarshalIndent(ret, "perms<", "  ")
	fmt.Println(string(serialized))

	return true, nil
}

func handleInteractionCreate(evt *eventsystem.EventData) {
	interaction := evt.InteractionCreate()
	if interaction.Type != discordgo.InteractionApplicationCommand {
		return
	}
	if interaction.DataCommand == nil {
		logger.Warn("Interaction had no data")
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

// Since we can't put permissions on subcommands we do a similar thing we do to command settings that is
func ContainerSlashCommandPermissions(container *slashCommandsContainer, overrides []*models.CommandsChannelsOverride, gs *dstate.GuildSet) ([]*discordgo.ApplicationCommandPermissions, error) {
	allowRoles, denyRoles, allowAll, denyAll, err := slashCommandPermissionsFromRolesFunc(container.rolesRunFunc, gs, container.defaultPermissions)
	if err != nil {
		return nil, err
	}

	childAllows, childDenies, childAllowAll, childDenyAll, err := sumContainerSlashCommandsChildren(container, overrides, gs)

	if allowAll {
		allowAll = childAllowAll
	}

	if !denyAll {
		denyAll = childDenyAll
	}

	allowRoles = commonSet(childAllows, allowRoles)
	denyRoles = mergeInt64Slice(denyRoles, childDenies)

	fmt.Printf("CONTAINER PERMS: %s(%d): allowAll: %v, denyAll: %v, allow: %v, deny: %v \n", container.container.Names[0], container.slashCommandID, allowAll, denyAll, allowRoles, denyRoles)

	result := toApplicationCommandPermissions(gs, container.defaultPermissions, allowRoles, denyRoles, allowAll, denyAll)
	return result, nil
}

// merges all subcommand allow roles
// and returns a common set of all subcommand deny roles
func sumContainerSlashCommandsChildren(container *slashCommandsContainer, overrides []*models.CommandsChannelsOverride, gs *dstate.GuildSet) (allowRoles []int64, denyRoles []int64, allowAll bool, denyAll bool, err error) {
	allowRoles = make([]int64, 0)
	denyRoles = make([]int64, 0)
	allowAll = false
	denyAll = true

	for _, v := range container.container.Commands {
		cast, ok := v.Command.(*YAGCommand)
		if !ok {
			continue
		}

		_allowRoles, _denyRoles, _allowAll, _denyAll, err := cast.SlashCommandPermissions(overrides, container.defaultPermissions, []*dcmd.Container{CommandSystem.Root}, gs)
		if err != nil {
			return nil, nil, false, false, err
		}

		if !allowAll {
			allowAll = _allowAll
		}

		allowRoles = mergeInt64Slice(_allowRoles, allowRoles)

		if denyAll {
			denyAll = _denyAll
			denyRoles = _denyRoles
		} else {
			denyRoles = commonSet(denyRoles, _denyRoles)
		}
	}

	return
}

func toApplicationCommandPermissions(gs *dstate.GuildSet, defaultEnabeld bool, allowRoles, denyRoles []int64, allowAll, denyAll bool) []*discordgo.ApplicationCommandPermissions {
	result := make([]*discordgo.ApplicationCommandPermissions, 0, 10)

	// allGuildroles := guildRoles(gs)

	// Add allow roles
	// if allowAll {
	// 	if !defaultEnabeld {
	// 		result = append(result, &discordgo.ApplicationCommandPermissions{
	// 			ID:         gs.ID,
	// 			Kind:       discordgo.CommandPermissionTypeRole,
	// 			Permission: true,
	// 		})
	// 	}
	// } else {
	// 	for _, v := range allowRoles {
	// 		result = append(result, &discordgo.ApplicationCommandPermissions{
	// 			ID:         v,
	// 			Kind:       discordgo.CommandPermissionTypeRole,
	// 			Permission: true,
	// 		})
	// 	}
	// }

	// for now were keeping the permissions simple because of limitations with interactions currently
	if allowAll || len(allowRoles) > 0 {
		if !defaultEnabeld {
			result = append(result, &discordgo.ApplicationCommandPermissions{
				ID:         gs.ID,
				Type:       discordgo.ApplicationCommandPermissionTypeRole,
				Permission: true,
			})
		}
	} else if denyAll {
		if defaultEnabeld {
			// for _, v := range allGuildroles {
			// 	if v == gs.ID {
			// 		continue
			// 	}

			// 	result = append(result, &discordgo.ApplicationCommandPermissions{
			// 		ID:         v,
			// 		Kind:       discordgo.CommandPermissionTypeRole,
			// 		Permission: false,
			// 	})
			// }

			result = append(result, &discordgo.ApplicationCommandPermissions{
				ID:         gs.ID,
				Type:       discordgo.ApplicationCommandPermissionTypeRole,
				Permission: false,
			})
		}
	} else {
		// we cannot do this atm because of the max 10 limit of permission overwrites on commands, so keep it simple

		// for _, v := range denyRoles {
		// 	result = append(result, &discordgo.ApplicationCommandPermissions{
		// 		ID:         v,
		// 		Kind:       discordgo.CommandPermissionTypeRole,
		// 		Permission: false,
		// 	})
		// }
	}

	return result
}

func (yc *YAGCommand) TopLevelSlashCommandPermissions(overrides []*models.CommandsChannelsOverride, gs *dstate.GuildSet) ([]*discordgo.ApplicationCommandPermissions, error) {

	allowRoles, denyRoles, allowAll, denyAll, err := yc.SlashCommandPermissions(overrides, yc.DefaultEnabled, []*dcmd.Container{CommandSystem.Root}, gs)
	if err != nil {
		return nil, err
	}

	result := toApplicationCommandPermissions(gs, yc.DefaultEnabled, allowRoles, denyRoles, allowAll, denyAll)
	return result, nil
}

func (yc *YAGCommand) SlashCommandPermissions(overrides []*models.CommandsChannelsOverride, defaultEnabeld bool, containerChain []*dcmd.Container, gs *dstate.GuildSet) (allowRoles []int64, denyRoles []int64, allowAll bool, denyAll bool, err error) {
	allowRoles = make([]int64, 0)
	denyRoles = make([]int64, 0)
	allowAll = true
	// denyAll = true

	// generate from custom roles funct first
	if yc.RolesRunFunc != nil {
		allowRoles, denyRoles, allowAll, denyAll, err = slashCommandPermissionsFromRolesFunc(yc.RolesRunFunc, gs, defaultEnabeld)
		if err != nil {
			return
		}
	}

	// check fixed required perms on the command
	if len(yc.RequireDiscordPerms) > 0 {
		roles := findRolesWithDiscordPerms(gs, yc.RequireDiscordPerms, defaultEnabeld)
		if defaultEnabeld {
			denyRoles = mergeInt64Slice(denyRoles, roles)
		} else {
			if allowAll {
				allowRoles = roles
			} else {
				allowRoles = commonSet(roles, allowRoles)
			}
		}

		allowAll = false
	}

	// check guild command settings
	fullName := containerChain[len(containerChain)-1].FullName(false)

	if !IsSlashCommandPermissionCommandEnabled(fullName, overrides) {
		// command is disabled in all channels
		return nil, nil, false, true, nil
	}

	// check overrides
	commonRequiredRoles, commonBlacklistedRoles := channelOverridesCommonSet(fullName, overrides)
	if len(commonRequiredRoles) > 0 {

		if defaultEnabeld {
			// Apply the inverse of the required roles to the deny roles, this effectively means were only blacklisting roles.
			// which means the permissions need to be updated after any roles are added.
		OUTER:
			for _, v := range gs.Roles {
				for _, required := range commonRequiredRoles {
					if v.ID == required {
						continue OUTER
					}
				}

				for _, alreadyDenied := range denyRoles {
					if alreadyDenied == v.ID {
						continue OUTER
					}
				}

				denyRoles = append(denyRoles, v.ID)
			}
		} else {
			if allowAll {
				allowRoles = commonRequiredRoles
			} else {
				allowRoles = commonSet(allowRoles, commonRequiredRoles)
			}
		}

		allowAll = false
	}

	denyRoles = mergeInt64Slice(denyRoles, commonBlacklistedRoles)

	return
}

func findRolesWithDiscordPerms(gs *dstate.GuildSet, requiredPerms []int64, defaultEnabled bool) []int64 {

	result := make([]int64, 0)

	// check fixed required perms on the command

OUTER:
	for _, r := range gs.Roles {
		perms := int64(r.Permissions)
		for _, rp := range requiredPerms {
			if perms&rp == rp || perms&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator || perms&discordgo.PermissionManageServer == discordgo.PermissionManageServer {
				// this role can run the command
				if !defaultEnabled {
					result = append(result, r.ID)
				}
				continue OUTER
			}
		}

		// this role can not run the command
		if defaultEnabled {
			result = append(result, r.ID)
		}
	}

	return result
}

// Since permissions in discord permissions is not per channel we can't apply this losslessly, which means i had to make a compromise
// and that compromise was to allow the slash command to be used in cases where it can't (altough during actual execution these will be checked again and you wont be able to run it in the end)
// blacklistRoles is a common set of all the overrides, meaning unless a blacklist role is present in all relevant overrides for the command it wont be applied
// allowRoles is a sum of all the relevant overrides
// this means that if a command is enabled in only a single channel, it will still show up as usable in all channels (but again, execution will be blocked if you try to use it)
func channelOverridesCommonSet(fullName string, overrides []*models.CommandsChannelsOverride) (allowRoles []int64, blacklistRoles []int64) {
	first := true

OUTER:
	for _, chOverride := range overrides {
		for _, cmdOverride := range chOverride.R.CommandsCommandOverrides {
			if common.ContainsStringSliceFold(cmdOverride.Commands, fullName) {
				if !cmdOverride.CommandsEnabled {
					continue OUTER
				}

				allowRoles = mergeInt64Slice(allowRoles, cmdOverride.RequireRoles)
				if first {
					first = false
					blacklistRoles = cmdOverride.IgnoreRoles
				} else {
					blacklistRoles = commonSet(blacklistRoles, cmdOverride.IgnoreRoles)
				}

				continue OUTER
			}
		}

		if !chOverride.CommandsEnabled {
			continue OUTER
		}

		// No command override found for this specific command, default to channel settings
		allowRoles = mergeInt64Slice(allowRoles, chOverride.RequireRoles)

		if first {
			first = false
			blacklistRoles = chOverride.IgnoreRoles
		} else {
			blacklistRoles = commonSet(blacklistRoles, chOverride.IgnoreRoles)
		}
	}

	return
}

// merges the 2 slices, without duplicates
func mergeInt64Slice(a []int64, b []int64) []int64 {
OUTER:
	for _, va := range a {
		// check if va is in b already
		for _, vb := range b {
			if va == vb {
				continue OUTER
			}
		}

		// va not found in b, add it to b
		b = append(b, va)
	}

	return b
}

func commonSet(a []int64, b []int64) []int64 {
	commonSet := make([]int64, 0, len(a))
OUTER:
	for _, av := range a {
		for _, bv := range b {
			if av == bv {
				commonSet = append(commonSet, av)
				continue OUTER
			}
		}
	}

	return commonSet
}

// IsSlashCommandPermissionCommandEnabled following up on the no per channel permissions for slash commands
// This might seem like a mouthfull but essentially for us to be able to disable a command in a guild
// it has to be disabled in all channels. If its enabled it atleast 1 channel override of command override we need to be able to pick it
// from the slash command list
func IsSlashCommandPermissionCommandEnabled(fullName string, overrides []*models.CommandsChannelsOverride) bool {
	// check if atleast one command override has it explicitly enabled, this takes precedence
	for _, override := range overrides {
		if isSlashCommandEnabledChannelOverride(fullName, override) {
			return true
		}
	}

	return false
}

func isSlashCommandEnabledChannelOverride(fullName string, override *models.CommandsChannelsOverride) bool {
	for _, cmdOverride := range override.R.CommandsCommandOverrides {
		if common.ContainsStringSliceFold(cmdOverride.Commands, fullName) {
			return cmdOverride.CommandsEnabled
		}
	}

	return override.CommandsEnabled
}

func slashCommandPermissionsFromRolesFunc(rf RolesRunFunc, gs *dstate.GuildSet, defaultEnabled bool) (allow []int64, deny []int64, allowAll bool, denyAll bool, err error) {

	roles, err := rf(gs)
	if err != nil {
		return nil, nil, false, false, err
	}

	if defaultEnabled {
		for _, v := range roles {
			if v == gs.ID {
				denyAll = true
				roles = []int64{}
				break
			}
		}

		return nil, roles, false, denyAll, nil
	} else {
		for _, v := range roles {
			if v == gs.ID {
				allowAll = true
				roles = []int64{}
				break
			}
		}

		return roles, nil, allowAll, false, nil
	}
}

func (p *Plugin) handleUpdateSlashCommandsPermissions(event *pubsub.Event) {
	gs := bot.State.GetGuild(event.TargetGuildInt)
	if gs == nil {
		return
	}

	waitForSlashCommandIDs()

	_, err := updateSlashCommandGuildPermissions(gs)
	if err != nil {
		logger.WithError(err).Error("failed updating slash command permissions")
	}
}

func waitForSlashCommandIDs() {
	for {
		if atomic.LoadInt32(slashCommandsIdsSet) == 1 {
			break
		}
		time.Sleep(time.Second)
	}
}

func PubsubSendUpdateSlashCommandsPermissions(gID int64) {
	err := pubsub.Publish("update_slash_command_permissions", gID, nil)
	if err != nil {
		logger.WithError(err).Error("failed sending pubsub for update_slash_command_permissions")
	}
}
