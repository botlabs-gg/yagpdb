package cah

import (
	"slices"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/cah/models"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/cardsagainstdiscord"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
)

func (p *Plugin) AddCommands() {

	cmdCreate := &commands.YAGCommand{
		Name:            "Create",
		CmdCategory:     commands.CategoryFun,
		Aliases:         []string{"c"},
		Description:     "Creates a Cards Against Humanity game in this channel, with the packs picked from a menu or typed",
		LongDescription: "Leave out the packs to pick them from a menu, or type them separated by spaces, * for all packs. Use -mode vote for players to vote on the winner instead of having a card czar.",
		Arguments: []*dcmd.ArgDef{
			{Name: "packs", Type: dcmd.String, Default: "", Help: "Packs separated by space, or * for all of them. Shows a menu to pick them from if left out."},
		},
		IsResponseEphemeral: true,
		ArgSwitches: []*dcmd.ArgDef{
			{Name: "mode", Type: dcmd.String, Default: "", Help: "Card czar or vote mode for this game, uses the server default if left out", Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "Card czar", Value: "czar"},
				{Name: "Vote", Value: "vote"},
			}},
		},
		RunFunc: func(data *dcmd.Data) (any, error) {
			conf, err := BotCachedGetConfig(data.GuildData.GS.ID)
			if err != nil {
				return nil, err
			}

			voteMode := conf.VoteMode
			switch strings.ToLower(data.Switch("mode").Str()) {
			case "":
			case "vote":
				voteMode = true
			case "czar":
				voteMode = false
			default:
				return "The mode can be either `czar` or `vote`.", nil
			}

			customPacks, err := GuildCardPacks(data.Context(), data.GuildData.GS.ID)
			if err != nil {
				return nil, err
			}

			picker := &gamePicker{
				GuildID:   data.GuildData.GS.ID,
				ChannelID: data.GuildData.CS.ID,
				UserID:    data.Author.ID,
				Username:  data.Author.Username,
				VoteMode:  voteMode,
			}

			if packs := strings.Fields(data.Args[0].Str()); len(packs) > 0 {
				msg, err := p.startGame(data.Context(), conf, customPacks, picker, packs)
				if msg != "" || err != nil {
					return msg, err
				}
				return nil, nil
			}

			flags := discordgo.MessageFlagsIsComponentsV2
			if data.TriggerType == dcmd.TriggerTypeSlashCommands {
				flags |= discordgo.MessageFlagsEphemeral
			}

			return &discordgo.MessageSend{
				Components: newPicker(conf, customPacks, picker),
				Flags:      flags,
			}, nil
		},
	}

	cmdEnd := &commands.YAGCommand{
		Name:        "End",
		CmdCategory: commands.CategoryFun,
		Description: "Ends a Cards Against Humanity game that is ongoing in this channel.",
		RunFunc: func(data *dcmd.Data) (any, error) {
			isAdmin, err := bot.AdminOrPermMS(data.GuildData.GS.ID, data.ChannelID, data.GuildData.MS, 0)
			if err == nil && isAdmin {
				err = p.Manager.RemoveGame(data.ChannelID)
			} else {
				err = p.Manager.TryAdminRemoveGame(data.Author.ID)
			}

			if err != nil {
				if cahErr := cardsagainstdiscord.HumanizeError(err); cahErr != "" {
					return cahErr, nil
				}

				return "", err
			}

			return "Stopped the game", nil
		},
	}

	cmdKick := &commands.YAGCommand{
		Name:         "Kick",
		CmdCategory:  commands.CategoryFun,
		RequiredArgs: 1,
		Arguments: []*dcmd.ArgDef{
			{Name: "user", Type: dcmd.UserID},
		},
		Description: "Kicks a player from the ongoing Cards Against Humanity game in this channel.",
		RunFunc: func(data *dcmd.Data) (any, error) {
			userID := data.Args[0].Int64()
			err := p.Manager.AdminKickUser(data.Author.ID, userID)
			if err != nil {
				if cahErr := cardsagainstdiscord.HumanizeError(err); cahErr != "" {
					return cahErr, nil
				}

				return "", err
			}

			return "User removed", nil
		},
	}

	cmdPacks := &commands.YAGCommand{
		Name:         "Packs",
		CmdCategory:  commands.CategoryFun,
		RequiredArgs: 0,
		Description:  "Lists all available packs.",
		RunFunc: func(data *dcmd.Data) (any, error) {
			conf, err := BotCachedGetConfig(data.GuildData.GS.ID)
			if err != nil {
				return nil, err
			}

			customPacks, err := GuildCardPacks(data.Context(), data.GuildData.GS.ID)
			if err != nil {
				return nil, err
			}

			var resp strings.Builder
			resp.WriteString("Available packs:\n\n")
			for _, name := range BuiltinPackNames() {
				if !slices.Contains(conf.DisabledPacks, name) {
					writePackLine(&resp, conf, cardsagainstdiscord.Packs[name])
				}
			}

			if len(customPacks) > 0 {
				resp.WriteString("\nThis server's packs:\n\n")
				for _, v := range customPacks {
					writePackLine(&resp, conf, v)
				}
			}

			return resp.String(), nil
		},
	}

	container, _ := commands.CommandSystem.Root.Sub("cah")
	container.NotFound = commands.CommonContainerNotFoundHandler(container, "")
	container.Description = "Play cards against humanity!"

	container.AddCommand(cmdCreate, cmdCreate.GetTrigger())
	container.AddCommand(cmdEnd, cmdEnd.GetTrigger())
	container.AddCommand(cmdKick, cmdKick.GetTrigger())
	container.AddCommand(cmdPacks, cmdPacks.GetTrigger())
	commands.RegisterSlashCommandsContainer(container, true, func(gs *dstate.GuildSet) ([]int64, error) {
		return nil, nil
	})
}

func writePackLine(resp *strings.Builder, conf *models.CahGuildConfig, pack *cardsagainstdiscord.CardPack) {
	resp.WriteString("`" + pack.Name + "` - " + pack.Description)
	if slices.Contains(conf.DefaultPacks, pack.Name) {
		resp.WriteString(" (default)")
	}
	resp.WriteString("\n")
}

// resolveGamePacks returns the packs to play, using the defaults when none were given, or why they can't be used
func resolveGamePacks(conf *models.CahGuildConfig, customPacks []*cardsagainstdiscord.CardPack, requested []string) ([]string, string) {
	isCustom := func(name string) bool {
		return slices.ContainsFunc(customPacks, func(p *cardsagainstdiscord.CardPack) bool { return p.Name == name })
	}
	isAvailable := func(name string) bool {
		return isCustom(name) || (cardsagainstdiscord.Packs[name] != nil && !slices.Contains(conf.DisabledPacks, name))
	}

	if len(requested) == 0 {
		// skip defaults that are no longer available, such as custom packs after premium ran out
		for _, v := range conf.DefaultPacks {
			if isAvailable(v) {
				requested = append(requested, v)
			}
		}

		if len(requested) == 0 && isAvailable("main") {
			requested = []string{"main"}
		}
	}

	if slices.Contains(requested, "*") {
		var all []string
		for _, name := range BuiltinPackNames() {
			if isAvailable(name) {
				all = append(all, name)
			}
		}
		for _, v := range customPacks {
			all = append(all, v.Name)
		}

		return all, ""
	}

	for _, v := range requested {
		if cardsagainstdiscord.Packs[v] != nil && !isCustom(v) && slices.Contains(conf.DisabledPacks, v) {
			return nil, "The `" + v + "` pack is turned off on this server."
		}
	}

	// unknown packs are reported by the game manager along with suggestions
	return requested, ""
}
