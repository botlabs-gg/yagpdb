package mentionrole

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
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
		return "You need manage server perms to use this command", nil
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
	if err != nil {
		return nil, err
	}

	err = scheduledevents2.ScheduleEvent("reset_mentionable_role", data.GS.ID, time.Now().Add(time.Second*10), &EvtData{
		GuildID: data.GS.ID,
		RoleID:  role.ID,
	})
	return nil, err
}
