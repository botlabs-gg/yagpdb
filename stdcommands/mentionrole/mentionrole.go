package mentionrole

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/scheduledevents2"
	seventsmodels "github.com/jonas747/yagpdb/common/scheduledevents2/models"
	"github.com/sirupsen/logrus"
)

type EvtData struct {
	GuildID int64
	RoleID  int64
}

func AddScheduledEventListener() {
	scheduledevents2.RegisterHandler("reset_mentionable_role", EvtData{}, handleResetMentionableRole)
	scheduledevents2.RegisterLegacyMigrater("reset_mentionable_role", handleMigrateResetMentionable)
}

func handleMigrateResetMentionable(t time.Time, data string) error {
	var parsedData EvtData
	err := json.Unmarshal([]byte(data), &parsedData)
	if err != nil {
		logrus.WithError(err).Error("Failed unmarshaling reset_mentionable_role data: ", data)
		return nil
	}

	return scheduledevents2.ScheduleEvent("reset_mentionable_role", parsedData.GuildID, t, parsedData)
}

func handleResetMentionableRole(evt *seventsmodels.ScheduledEvent, dataInterface interface{}) (retry bool, err error) {
	data := dataInterface.(*EvtData)

	gs := bot.State.Guild(true, evt.GuildID)
	if gs == nil {
		return false, nil
	}

	role := gs.RoleCopy(true, data.RoleID)

	if role == nil {
		return false, nil // Assume role was deleted or something in the meantime
	}

	_, err = common.BotSession.GuildRoleEdit(gs.ID, role.ID, role.Name, role.Color, role.Hoist, role.Permissions, false)
	return scheduledevents2.CheckDiscordErrRetry(err), err
}

var Command = &commands.YAGCommand{
	CmdCategory:     commands.CategoryTool,
	Name:            "MentionRole",
	Aliases:         []string{"mrole"},
	Description:     "Sets a role to mentionable, mentions the role, and then sets it back",
	LongDescription: "Requires the manage roles permission and the bot being above the mentioned role",
	HideFromHelp:    true,
	RequiredArgs:    1,
	Arguments: []*dcmd.ArgDef{
		{Name: "Role", Type: dcmd.String},
		{Name: "Message", Type: dcmd.String, Default: ""},
	},
	ArgSwitches: []*dcmd.ArgDef{
		&dcmd.ArgDef{Switch: "channel", Help: "Optional channel to send in", Type: dcmd.Channel},
	},
	RunFunc:            cmdFuncMentionRole,
	GuildScopeCooldown: 10,
}

func cmdFuncMentionRole(data *dcmd.Data) (interface{}, error) {
	if ok, err := bot.AdminOrPermMS(data.CS.ID, data.MS, discordgo.PermissionManageRoles); err != nil {
		return "Failed checking perms", err
	} else if !ok {
		return "You need manage roles perms to use this command", nil
	}

	roleS := data.Args[0].Str()
	role := findRoleByName(data.GS, roleS)

	//if we did not find a match yet try to match ID
	if role == nil {
		parsedNumber, parseErr := strconv.ParseInt(roleS, 10, 64)
		if parseErr == nil {
			// was a number, try looking by id
			role = data.GS.RoleCopy(true, parsedNumber)
		}
	}

	if role == nil {
		return "No role with the name or ID`" + roleS + "` found", nil
	}

	cID := data.CS.ID
	c := data.Switch("channel")
	if c.Value != nil {
		cID = c.Value.(*dstate.ChannelState).ID

		perms, err := data.GS.MemberPermissions(true, cID, data.Msg.Author.ID)
		if err != nil {
			return nil, err
		}

		if perms&discordgo.PermissionSendMessages != discordgo.PermissionSendMessages || perms&discordgo.PermissionReadMessages != discordgo.PermissionReadMessages {
			return "You do not have permissions to send messages there", nil
		}
	}

	_, err := common.BotSession.GuildRoleEdit(data.GS.ID, role.ID, role.Name, role.Color, role.Hoist, role.Permissions, true)
	if err != nil {
		return nil, err
	}

	_, err = common.BotSession.ChannelMessageSendComplex(cID, &discordgo.MessageSend{
		Content: "<@&" + discordgo.StrID(role.ID) + "> " + data.Args[1].Str(),
		AllowedMentions: discordgo.AllowedMentions{
			Roles: []int64{role.ID},
		},
	})

	if err != nil {
		return nil, err
	}

	err = scheduledevents2.ScheduleEvent("reset_mentionable_role", data.GS.ID, time.Now().Add(time.Second*10), &EvtData{
		GuildID: data.GS.ID,
		RoleID:  role.ID,
	})
	return nil, err
}

func findRoleByName(gs *dstate.GuildState, name string) *discordgo.Role {
	var role *discordgo.Role

	gs.RLock()
	defer gs.RUnlock()
	for _, r := range gs.Guild.Roles {
		if strings.EqualFold(r.Name, name) {
			role = r
			break
		}
	}

	return role
}
