package commands

import (
	"encoding/json"
	"fmt"

	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
)

var (
	slashCommandsContainers []*slashCommandsContainer
)

type slashCommandsContainer struct {
	container          *dcmd.Container
	defaultPermissions bool
	rolesRunFunc       RolesRunFunc
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

	encoded, _ := json.MarshalIndent(result, "", " ")
	fmt.Println(string(encoded))

	_, err := common.BotSession.BulkOverwriteGlobalApplicationCommands(common.BotApplication.ID, result)
	if err != nil {
		logger.WithError(err).Error("failed updating global slash commands")
	}
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

	return &discordgo.CreateApplicationCommandRequest{
		Name:              cmd.Trigger.Names[0],
		Description:       common.CutStringShort(cast.Description, 100),
		DefaultPermission: &cast.DefaultEnabled,
		Options:           cast.slashCommandOptions(),
	}
}

func (yc *YAGCommand) slashCommandOptions() []*discordgo.ApplicationCommandOption {
	var result []*discordgo.ApplicationCommandOption

	for i, v := range yc.Arguments {
		opts := v.Type.SlashCommandOptions(v)
		if len(opts) == 1 {
			if i < yc.RequiredArgs {
				opts[0].Required = true
			}
		}

		result = append(result, opts...)
	}
	for _, v := range yc.ArgSwitches {
		result = append(result, v.Type.SlashCommandOptions(v)...)
	}

	return result
}

// func (p *Plugin) yagCommandToSlashCommand(cmd *YAGCommand) *discordgo.Creat {

// }

func handleInteractionCreate(evt *eventsystem.EventData) {
	interaction := evt.InteractionCreate()
	if interaction.Data == nil {
		logger.Warn("Interaction had no data")
		return
	}

	serialized, _ := json.MarshalIndent(interaction.Interaction, "", "  ")
	logger.Infof("Got interaction %#v", interaction.Interaction)
	fmt.Println(string(serialized))

	err := CommandSystem.CheckInteraction(common.BotSession, interaction.Interaction)
	if err != nil {
		logger.WithError(err).Error("failed handling command interaction")
	}
}
