package commands

import (
	"fmt"
	"strings"

	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot/paginatedmessages"
	"github.com/jonas747/yagpdb/common"
)

var cmdHelp = &YAGCommand{
	Name:        "Pomoc",
	Aliases:     []string{"commands", "h", "how", "command", "help", "komendy"},
	Description: "Pokazuje pomoc bota albo jednej komendy",
	CmdCategory: CategoryGeneral,
	RunInDM:     true,
	Arguments: []*dcmd.ArgDef{
		&dcmd.ArgDef{Name: "komenda", Type: dcmd.String},
	},

	RunFunc:  cmdFuncHelp,
	Cooldown: 10,
}

func CmdNotFound(search string) string {
	return fmt.Sprintf("Nie znaleziono komendy %q", search)
}

func cmdFuncHelp(data *dcmd.Data) (interface{}, error) {
	target := data.Args[0].Str()

	var resp []*discordgo.MessageEmbed

	// Send the targetted help in the channel it was requested in
	resp = dcmd.GenerateTargettedHelp(target, data, data.ContainerChain[0], &dcmd.StdHelpFormatter{})
	for _, v := range resp {
		ensureEmbedLimits(v)
	}

	if target != "" {
		if len(resp) != 1 {
			// Send command not found in same channel
			return CmdNotFound(target), nil
		}

		// Send short help in same channel
		return resp, nil
	}

	// Send full help in DM
	ir, err := createInteractiveHelp(data.Msg.Author.ID, resp)
	if ir != nil || err != nil {
		return ir, err
	}

	if data.Source == dcmd.DMSource {
		return nil, nil
	}

	return "Sprawdź prywatne wiadomości!", nil
}

func createInteractiveHelp(userID int64, helpEmbeds []*discordgo.MessageEmbed) (interface{}, error) {
	channel, err := common.BotSession.UserChannelCreate(userID)
	if err != nil {
		return "Coś poszło nie tak, może masz wyłączone wiadomości? Tu masz angielski link do wszystkich komend: <https://docs.yagpdb.xyz/commands>", err
	}

	// prepend a introductionairy first page
	firstPage := &discordgo.MessageEmbed{
		Title: "Pomoc bota Policjant",
		Description: `Policjant jest polską wersją bota YAGPDB.xyz napisaną na potrzeby serwera Robuxianie http://robuxianie.pl/
Żeby zobaczyć pomoc (w języku angielskim) wejdź na https://docs.yagpdb.xyz/. Ta komenda tylko daje informacje na temat komend.
		
		
**Użyj emoji poniżej żeby zmienić stronę**`,
	}

	var pageLayout strings.Builder
	for i, v := range helpEmbeds {
		pageLayout.WriteString(fmt.Sprintf("**Strona %d**: %s\n", i+2, v.Title))
	}
	firstPage.Fields = []*discordgo.MessageEmbedField{
		{Name: "Strony:", Value: pageLayout.String()},
	}

	helpEmbeds = append([]*discordgo.MessageEmbed{firstPage}, helpEmbeds...)

	_, err = paginatedmessages.CreatePaginatedMessage(0, channel.ID, 1, len(helpEmbeds), func(p *paginatedmessages.PaginatedMessage, page int) (*discordgo.MessageEmbed, error) {
		embed := helpEmbeds[page-1]
		return embed, nil
	})
	if err != nil {
		return "Coś poszło nie tak, upewnij się że nie zablokowałeś bota!", err

	}

	return nil, nil
}
