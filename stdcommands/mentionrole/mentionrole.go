package mentionrole

import (
	"encoding/json"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/scheduledevents"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"strings"
	"time"
)

type EvtData struct {
	GuildID int64
	RoleID  int64
}

func AddScheduledEventListener() {
	scheduledevents.RegisterEventHandler("reset_mentionable_role", func(data string) error {
		var parsedData EvtData
		err := json.Unmarshal([]byte(data), &parsedData)
		if err != nil {
			logrus.WithError(err).Error("Failed unmarshaling reset_mentionable_role data: ", data)
			return nil
		}

		gs := bot.State.Guild(true, parsedData.GuildID)
		if gs == nil {
			return nil
		}

		var role *discordgo.Role
		gs.RLock()
		defer gs.RUnlock()
		for _, r := range gs.Guild.Roles {
			if r.ID == parsedData.RoleID {
				role = r
				break
			}
		}

		if role == nil {
			return nil // Assume role was deleted or something in the meantime
		}

		_, err = common.BotSession.GuildRoleEdit(gs.ID, role.ID, role.Name, role.Color, role.Hoist, role.Permissions, false)
		if err != nil {
			if cast, ok := errors.Cause(err).(*discordgo.RESTError); ok && cast.Message != nil && cast.Message.Code != 0 {
				return nil // Discord api ok, something else went wrong. do not reschedule
			}

			return err
		}

		return nil
	})
}

var Command = &commands.YAGCommand{
	CmdCategory:     commands.CategoryTool,
	Name:            "MentionRole",
	Aliases:         []string{"mrole"},
	Description:     "Sets a role to mentionable, mentions the role, and then sets it back",
	LongDescription: "Requires the manage roles permission and the bot being above the mentioned role",
	RequiredArgs:    1,
	Arguments: []*dcmd.ArgDef{
		{Name: "Role", Type: dcmd.String},
		{Name: "Message", Type: dcmd.String, Default: ""},
	},
	RunFunc: cmdFuncMentionRole,
}

func cmdFuncMentionRole(data *dcmd.Data) (interface{}, error) {
	if ok, err := bot.AdminOrPerm(discordgo.PermissionManageRoles, data.Msg.Author.ID, data.CS.ID); err != nil {
		return "Failed checking perms", err
	} else if !ok {
		return "You need manage server perms to use this commmand", nil
	}

	var role *discordgo.Role
	data.GS.RLock()
	defer data.GS.RUnlock()
	for _, r := range data.GS.Guild.Roles {
		if strings.EqualFold(r.Name, data.Args[0].Str()) {
			role = r
			break
		}
	}

	if role == nil {
		return "No role with the name `" + data.Args[0].Str() + "` found", nil
	}

	_, err := common.BotSession.GuildRoleEdit(data.GS.ID, role.ID, role.Name, role.Color, role.Hoist, role.Permissions, true)
	if err != nil {
		return nil, err
	}

	_, err = common.BotSession.ChannelMessageSend(data.CS.ID, "<@&"+discordgo.StrID(role.ID)+"> "+data.Args[1].Str())

	scheduledData, err := json.Marshal(EvtData{
		GuildID: data.GS.ID,
		RoleID:  role.ID,
	})
	if err != nil {
		return nil, err
	}

	scheduledevents.ScheduleEvent("reset_mentionable_role", string(scheduledData), time.Now().Add(time.Minute))

	return "", err
}
