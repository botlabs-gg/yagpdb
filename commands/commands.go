package commands

//go:generate esc -o assets_gen.go -pkg commands -ignore ".go" assets/

import (
	log "github.com/Sirupsen/logrus"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/docs"
	"github.com/mediocregopher/radix.v2/redis"
)

type Plugin struct{}

func RegisterPlugin() {
	plugin := &Plugin{}
	common.RegisterPlugin(plugin)
	err := common.GORM.AutoMigrate(&common.LoggedExecutedCommand{}).Error
	if err != nil {
		log.WithError(err).Error("Failed migrating database")
	}

	docs.AddPage("Commands", FSMustString(false, "/assets/help-page.md"), nil)
}

func (p *Plugin) Name() string {
	return "Commands"
}

type ChannelCommandSetting struct {
	Info           *YAGCommand `json:"-"` // Used for template info
	Cmd            string      `json:"cmd"`
	CommandEnabled bool        `json:"enabled"`
	AutoDelete     bool        `json:"autodelete"`
	RequiredRole   string      `json:"required_role"`
}

type ChannelOverride struct {
	Settings        []*ChannelCommandSetting `json:"settings"`
	OverrideEnabled bool                     `json:"enabled"`
	Channel         string                   `json:"channel"`
	ChannelName     string                   `json:"-"` // Used for the template rendering
}

type CommandsConfig struct {
	Prefix string `json:"-"` // Stored in a seperate key for speed

	Global           []*ChannelCommandSetting `json:"gloabl"`
	ChannelOverrides []*ChannelOverride       `json:"overrides"`
}

// Fills in the defaults for missing data, for when users create channels or commands are added
func CheckChannelsConfig(conf *CommandsConfig, channels []*discordgo.Channel) {

	commands := CommandSystem.Root.Commands

	if conf.Global == nil {
		conf.Global = []*ChannelCommandSetting{}
	}

	if conf.ChannelOverrides == nil {
		conf.ChannelOverrides = []*ChannelOverride{}
	}

ROOT:
	for _, channel := range channels {
		if channel.Type != discordgo.ChannelTypeGuildText {
			continue
		}

		// Look for an existing override
		for _, override := range conf.ChannelOverrides {
			// Found an existing override, check if it has all the commands
			if channel.ID == override.Channel {
				override.Settings = checkCommandSettings(override.Settings, commands, false)
				override.ChannelName = channel.Name // Update name if changed
				continue ROOT
			}
		}

		// Not found, create a default override
		override := &ChannelOverride{
			Settings:        []*ChannelCommandSetting{},
			OverrideEnabled: false,
			Channel:         channel.ID,
			ChannelName:     channel.Name,
		}

		// Fill in default command settings
		override.Settings = checkCommandSettings(override.Settings, commands, false)
		conf.ChannelOverrides = append(conf.ChannelOverrides, override)
	}

	newOverrides := make([]*ChannelOverride, 0, len(conf.ChannelOverrides))

	// Check for removed channels
	for _, override := range conf.ChannelOverrides {
		for _, channel := range channels {
			if channel.Type != discordgo.ChannelTypeGuildText {
				continue
			}

			if channel.ID == override.Channel {
				newOverrides = append(newOverrides, override)
				break
			}
		}
	}
	conf.ChannelOverrides = newOverrides

	// Check the global settings
	conf.Global = checkCommandSettings(conf.Global, commands, true)
}

// Checks a single list of ChannelCommandSettings and applies defaults if not found
func checkCommandSettings(settings []*ChannelCommandSetting, commands []*dcmd.RegisteredCommand, defaultEnabled bool) []*ChannelCommandSetting {

ROOT:
	for _, registeredCmd := range commands {
		cast, ok := registeredCmd.Command.(*YAGCommand)
		if !ok {
			continue
		}

		for _, settingsCmd := range settings {
			if cast.Name == settingsCmd.Cmd {
				// Bingo
				settingsCmd.Info = cast
				continue ROOT
			}
		}

		// Not found, add it to the list of overrides
		settingsCmd := &ChannelCommandSetting{
			Cmd:            cast.Name,
			CommandEnabled: defaultEnabled,
			AutoDelete:     false,
			Info:           cast,
		}
		settings = append(settings, settingsCmd)
	}

	newSettings := make([]*ChannelCommandSetting, 0, len(settings))

	// Check for commands that have been removed (e.g the config contains commands from an older version)
	for _, settingsCmd := range settings {
		for _, registeredCmd := range commands {
			cast, ok := registeredCmd.Command.(*YAGCommand)
			if !ok {
				continue
			}

			if cast.Name == settingsCmd.Cmd {
				newSettings = append(newSettings, settingsCmd)
				break
			}
		}
	}

	return newSettings
}

func GetConfig(client *redis.Client, guild string, channels []*discordgo.Channel) *CommandsConfig {
	var config *CommandsConfig
	err := common.GetRedisJson(client, "commands_settings:"+guild, &config)
	if err != nil {
		log.WithError(err).Error("Error retrieving command settings")
	}

	if config == nil {
		config = &CommandsConfig{}
	}

	prefix, err := GetCommandPrefix(client, guild)
	if err != nil {
		// Continue as normal with defaults
		log.WithError(err).Error("Error fetching command prefix")
	}

	config.Prefix = prefix

	// Fill in defaults
	CheckChannelsConfig(config, channels)

	return config
}

func GetCommandPrefix(client *redis.Client, guild string) (string, error) {
	reply := client.Cmd("GET", "command_prefix:"+guild)
	if reply.Err != nil {
		return "", reply.Err
	}
	if reply.IsType(redis.Nil) {
		return "", nil
	}

	return reply.Str()
}
