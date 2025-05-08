package notes

import (
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
)

var logger = common.GetPluginLogger(&Plugin{})

var _ bot.BotInitHandler = (*Plugin)(nil)
var _ commands.CommandProvider = (*Plugin)(nil)

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(p, cmds...)
}

func (p *Plugin) BotInit() {
	eventsystem.AddHandlerAsyncLast(p, handleInteractionCreate, eventsystem.EventInteractionCreate)
}

var notPermittedResp = &discordgo.InteractionResponse{
	Type: discordgo.InteractionResponseChannelMessageWithSource,
	Data: &discordgo.InteractionResponseData{
		Components: []discordgo.TopLevelComponent{
			discordgo.TextDisplay{
				Content: "You are no longer permitted to use notes on this server. Please contact your server admin.",
			},
		},
		Flags: discordgo.MessageFlagsEphemeral | discordgo.MessageFlagsIsComponentsV2,
	},
}

func handleInteractionCreate(evt *eventsystem.EventData) (retry bool, err error) {
	ic := evt.InteractionCreate()
	if ic.GuildID == 0 {
		return
	}
	if ic.Member == nil {
		return
	}
	if ic.ChannelID == 0 {
		return
	}

	if evt.GS == nil {
		evt.GS = &dstate.GuildSet{}
		evt.GS.ID = ic.GuildID
	}

	var hasPerms bool
	ms := dstate.MemberStateFromMember(ic.Member)
	for _, p := range requiredPerms {
		hasPerms, err = bot.AdminOrPermMS(ic.GuildID, ic.ChannelID, ms, p)
		if hasPerms || err != nil {
			break
		}
	}

	if ic.Type == discordgo.InteractionMessageComponent {
		stripped, ok := strings.CutPrefix(ic.MessageComponentData().CustomID, "notes_")
		if !ok {
			return false, nil
		}
		resp := notPermittedResp
		if hasPerms {
			resp, err = handleNoteButton(evt, stripped)
			if err != nil {
				return false, err
			}
		}
		return false, common.BotSession.CreateInteractionResponse(ic.ID, ic.Token, resp)
	} else if ic.Type == discordgo.InteractionModalSubmit {
		stripped, ok := strings.CutPrefix(ic.ModalSubmitData().CustomID, "notes_")
		if !ok {
			return false, nil
		}
		resp := notPermittedResp
		if hasPerms {
			newVal := ic.ModalSubmitData().Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value
			resp, err = handleNoteModal(evt, stripped, newVal)
			if err != nil {
				return false, err
			}
		}
		return false, common.BotSession.CreateInteractionResponse(ic.ID, ic.Token, resp)
	}
	return false, nil
}

func errResponse(err error) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Components: []discordgo.TopLevelComponent{
				discordgo.TextDisplay{
					Content: err.Error(),
				},
			},
			Flags: discordgo.MessageFlagsEphemeral | discordgo.MessageFlagsIsComponentsV2,
		},
	}
}

func handleNoteButton(evt *eventsystem.EventData, strippedID string) (*discordgo.InteractionResponse, error) {
	action := parseCustomID(strippedID)
	notes, err := getNotes(evt.Context(), evt.GS.ID, action.userID)
	if err != nil {
		return nil, err
	}
	switch action.actionType {
	case noteActionTypeNew:
		return createModal(notes, nil), nil
	case noteActionTypeEdit:
		return createModal(notes, &action.index), nil
	case noteActionTypeDelete:
		err = notes.delete(action.index)
		if err != nil {
			return errResponse(err), nil
		}

		err = notes.save(evt.Context())
		if err != nil {
			return nil, err
		}

		m := createMessage(notes)
		return &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Components: m.Components,
				Flags:      m.Flags & discordgo.MessageFlagsIsComponentsV2,
			},
		}, nil
	}
	return nil, nil
}

func handleNoteModal(evt *eventsystem.EventData, strippedID, newVal string) (*discordgo.InteractionResponse, error) {
	ic := evt.InteractionCreate()
	action := parseCustomID(strippedID)
	notes, err := getNotes(evt.Context(), evt.GS.ID, action.userID)
	if err != nil {
		return nil, err
	}
	switch action.actionType {
	case noteActionTypeNew:
		err = notes.add(newVal, ic.Member.User)
		if err != nil {
			return errResponse(err), nil
		}
	case noteActionTypeEdit:
		err = notes.edit(action.index, newVal, ic.Member.User)
		if err != nil {
			return errResponse(err), nil
		}
	}

	err = notes.save(evt.Context())
	if err != nil {
		return nil, err
	}
	m := createMessage(notes)
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Components: m.Components,
			Flags:      m.Flags & discordgo.MessageFlagsIsComponentsV2,
		},
	}, nil
}
