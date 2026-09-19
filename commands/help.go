package commands

import (
	"fmt"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/bot/paginatedmessages"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/web"
)

var cmdHelp = &YAGCommand{
	Name:        "Help",
	Aliases:     []string{"commands", "h", "how", "command"},
	Description: "Shows help about all or one specific command",
	CmdCategory: CategoryGeneral,
	RunInDM:     true,

	SlashCommandEnabled: true,
	DefaultEnabled:      true,
	Arguments: []*dcmd.ArgDef{
		{Name: "command", Type: dcmd.String},
	},

	RunFunc: cmdFuncHelp,
}

func CmdNotFound(search string) string {
	return "Couldn't find that command."
}

func cmdFuncHelp(data *dcmd.Data) (interface{}, error) {
	target := data.Args[0].Str()

	// Send the targetted help in the channel it was requested in
	resp := dcmd.GenerateTargettedHelp(target, data, data.ContainerChain[0], helpFormatter)
	for _, v := range resp {
		ensureEmbedLimits(v)
	}

	if target != "" {
		if len(resp) != 1 {
			// Send command not found in same channel
			return CmdNotFound(target), nil
		}

		// see if we can find the permissions the command needs and add that info to the help message
		cmd, _ := data.ContainerChain[0].AbsFindCommand(target)
		if cmd == nil {
			return resp, nil
		}

		yc, ok := cmd.Command.(*YAGCommand)
		if !ok {
			return resp, nil
		}

		if len(yc.RequireDiscordPerms) == 0 && yc.RequiredDiscordPermsHelp == "" {
			return resp, nil
		}

		requiredPerms := yc.RequiredDiscordPermsHelp
		if requiredPerms == "" {
			humanizedPerms := make([]string, 0, len(yc.RequireDiscordPerms))
			for _, v := range yc.RequireDiscordPerms {
				h := common.HumanizePermissions(v)
				if len(h) == 1 {
					humanizedPerms = append(humanizedPerms, h[0])
				} else {
					joined := strings.Join(h, " and ")
					humanizedPerms = append(humanizedPerms, "("+joined+")")
				}
			}
			requiredPerms = strings.Join(humanizedPerms, " or ")
		}

		embed := resp[0]
		embed.Footer = &discordgo.MessageEmbedFooter{
			Text: "Required permissions: " + requiredPerms,
		}
		return embed, nil
	}

	return createInteractiveHelp(data, resp)
}

func createInteractiveHelp(data *dcmd.Data, helpEmbeds []*discordgo.MessageEmbed) (interface{}, error) {
	// prepend a introductionairy first page
	firstPage := &discordgo.MessageEmbed{
		Title: "YAGPDB Help",
		Description: fmt.Sprintf(`YAGPDB is an open-source multipurpose discord bot that is configured through the web interface at %s.
For more in depth help and information you should visit https://help.yagpdb.xyz/ as this command only shows information about commands.

Use the buttons below to change pages, or `+"`/help <command>`"+` for details on one command.`, web.BaseURL()),
	}

	var pageLayout strings.Builder
	for i, v := range helpEmbeds {
		pageLayout.WriteString(fmt.Sprintf("**Page %d**: %s\n", i+2, v.Title))
	}
	firstPage.Fields = []*discordgo.MessageEmbedField{
		{Name: "Help pages", Value: pageLayout.String()},
	}

	helpEmbeds = append([]*discordgo.MessageEmbed{firstPage}, helpEmbeds...)

	// Answer where it was asked. Sending this to dms meant a mention or slash
	// invocation in a channel looked like it had done nothing.
	guildID := int64(0)
	if data.GuildData != nil {
		guildID = data.GuildData.GS.ID
	}

	return paginatedmessages.NewPaginatedResponse(guildID, data.ChannelID, 1, len(helpEmbeds), func(p *paginatedmessages.PaginatedMessage, page int) (*discordgo.MessageEmbed, error) {
		embed := helpEmbeds[page-1]
		return embed, nil
	}), nil
}
