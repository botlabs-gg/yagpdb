package customcommands

import (
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/customcommands/models"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
)

func ExecuteCustomCommandFromModal(cc *models.CustomCommand, gs *dstate.GuildSet, cs *dstate.ChannelState, cmdArgs []string, stripped string, interaction *templates.CustomCommandInteraction) error {
	ms := dstate.MemberStateFromMember(interaction.Member)
	tmplCtx := templates.NewContext(gs, cs, ms)
	tmplCtx.CurrentFrame.Interaction = interaction

	tmplCtx.Data["Interaction"] = interaction
	tmplCtx.Data["InteractionData"] = interaction.ModalSubmitData()
	modalCustomID := strings.TrimPrefix(interaction.ModalSubmitData().CustomID, templates.TemplateCustomIDPrefix)
	tmplCtx.Data["CustomID"] = modalCustomID
	tmplCtx.Data["Cmd"] = cmdArgs[0]
	if len(cmdArgs) > 1 {
		tmplCtx.Data["CmdArgs"] = cmdArgs[1:]
	} else {
		tmplCtx.Data["CmdArgs"] = []string{}
	}
	tmplCtx.Data["StrippedID"] = stripped
	tmplCtx.Data["StrippedMsg"] = stripped
	tmplCtx.Data["IsModal"] = true
	cmdValues := []any{}

	modalValues := templates.SDict{}
	for i := 0; i < len(interaction.ModalSubmitData().Components); i++ {
		switch comp := interaction.ModalSubmitData().Components[i].(type) {
		case *discordgo.ActionsRow:
			for j := 0; j < len(comp.Components); j++ {
				field, ok := comp.Components[j].(*discordgo.TextInput)
				if ok {
					cmdValues = append(cmdValues, field.Value)
				}
				cID, _ := strings.CutPrefix(field.CustomID, templates.TemplateCustomIDPrefix)
				modalValues.Set(cID, templates.SDict{
					"type":      field.Type(),
					"value":     field.Value,
					"custom_id": cID,
				})
			}
		case *discordgo.Label:
			if t, ok := comp.Component.(*discordgo.TextInput); ok {
				cID, _ := strings.CutPrefix(t.CustomID, templates.TemplateCustomIDPrefix)
				cmdValues = append(cmdValues, t.Value)
				modalValues.Set(cID, templates.SDict{
					"type":      t.Type(),
					"value":     t.Value,
					"custom_id": cID,
				})
			} else if sm, ok := comp.Component.(*discordgo.SelectMenu); ok {
				cID, _ := strings.CutPrefix(sm.CustomID, templates.TemplateCustomIDPrefix)
				cmdValues = append(cmdValues, sm.Values)
				modalValues.Set(cID, templates.SDict{
					"type":      sm.Type(),
					"value":     sm.Values,
					"custom_id": cID,
				})
			}
		}
	}
	tmplCtx.Data["Values"] = cmdValues
	tmplCtx.Data["ModalValues"] = modalValues
	msg := interaction.Message
	msg.Member = ms.DgoMember()
	msg.Author = msg.Member.User
	tmplCtx.Msg = msg

	tmplCtx.Data["Message"] = msg

	return ExecuteCustomCommand(cc, tmplCtx)
}

func CheckMatchModal(cmd *models.CustomCommand, cID string) (match bool, stripped string, args []string) {

	if cmd.TriggerType != int(CommandTriggerModal) {
		return false, "", nil
	}

	cmdMatch := "(?m)"
	if !cmd.TextTriggerCaseSensitive {
		cmdMatch += "(?i)"
	}
	cmdMatch += cmd.TextTrigger

	match, stripped, args = matchRegexSplitArgs(cmdMatch, cID)
	return
}
