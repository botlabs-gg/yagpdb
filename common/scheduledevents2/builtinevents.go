package scheduledevents2

import (
	"context"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2/models"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

type DeleteMessagesEvent struct {
	GuildID   int64
	ChannelID int64
	Messages  []int64
}

func registerBuiltinEvents() {
	RegisterHandler("delete_messages", DeleteMessagesEvent{}, handleDeleteMessagesEvent)
	RegisterHandler("std_remove_member_role", RmoveRoleData{}, handleRemoveMemberRole)
	RegisterHandler("std_add_member_role", AddRoleData{}, handleAddMemberRole)
}

func ScheduleDeleteMessages(guildID, channelID int64, when time.Time, messages ...int64) error {
	msgs := messages

	if len(messages) > 100 {
		msgs = messages[:100]
	}

	err := ScheduleEvent("delete_messages", guildID, when, &DeleteMessagesEvent{
		GuildID:   guildID,
		ChannelID: channelID,
		Messages:  msgs,
	})

	if err != nil {
		return err
	}

	if len(messages) > 100 {
		return ScheduleDeleteMessages(guildID, channelID, when, messages[100:]...)
	}

	return nil
}

func handleDeleteMessagesEvent(evt *models.ScheduledEvent, data interface{}) (retry bool, err error) {
	dataCast := data.(*DeleteMessagesEvent)

	bot.MessageDeleteQueue.DeleteMessages(dataCast.GuildID, dataCast.ChannelID, dataCast.Messages...)
	return false, nil
}

type RmoveRoleData struct {
	GuildID int64 `json:"guild_id"`
	UserID  int64 `json:"user_id"`
	RoleID  int64 `json:"role_id"`
}

func ScheduleRemoveRole(ctx context.Context, guildID, userID, roleID int64, when time.Time) error {
	// remove existing role removal events for this role, this may not be the desired outcome in all cases, but for now it is like this
	_, err := models.ScheduledEvents(qm.Where("event_name='std_remove_member_role' AND  guild_id = ? AND (data->>'user_id')::bigint = ? AND (data->>'role_id')::bigint = ? AND processed = false", guildID, userID, roleID)).DeleteAll(ctx, common.PQ)
	if err != nil {
		return err
	}

	// add the scheduled event for it
	err = ScheduleEvent("std_remove_member_role", guildID, when, &RmoveRoleData{
		GuildID: guildID,
		UserID:  userID,
		RoleID:  roleID,
	})

	if err != nil {
		return err
	}

	return nil
}

func CancelRemoveRole(ctx context.Context, guildID, userID, roleID int64) error {
	_, err := models.ScheduledEvents(qm.Where("event_name='std_remove_member_role' AND  guild_id = ? AND (data->>'user_id')::bigint = ? AND (data->>'role_id')::bigint = ? AND processed = false",
		guildID, userID, roleID)).DeleteAll(ctx, common.PQ)

	return err
}

func handleRemoveMemberRole(evt *models.ScheduledEvent, data interface{}) (retry bool, err error) {
	dataCast := data.(*RmoveRoleData)
	err = common.BotSession.GuildMemberRoleRemove(dataCast.GuildID, dataCast.UserID, dataCast.RoleID)
	if err != nil {
		return CheckDiscordErrRetry(err), err
	}

	return CheckDiscordErrRetry(err), err
}

type AddRoleData struct {
	UserID int64 `json:"user_id"`
	RoleID int64 `json:"role_id"`
}

func ScheduleAddRole(ctx context.Context, guildID, userID, roleID int64, when time.Time) error {
	// remove existing role addition events for this role, this may not be the desired outcome in all cases, but for now it is like this
	err := CancelAddRole(ctx, guildID, userID, roleID)
	if err != nil {
		return err
	}

	// add the scheduled event for it
	err = ScheduleEvent("std_add_member_role", guildID, when, &AddRoleData{
		UserID: userID,
		RoleID: roleID,
	})

	if err != nil {
		return err
	}

	return nil
}

func CancelAddRole(ctx context.Context, guildID, userID, roleID int64) error {
	_, err := models.ScheduledEvents(qm.Where("event_name='std_add_member_role' AND  guild_id = ? AND (data->>'user_id')::bigint = ? AND (data->>'role_id')::bigint = ? AND processed = false",
		guildID, userID, roleID)).DeleteAll(ctx, common.PQ)

	return err
}

func handleAddMemberRole(evt *models.ScheduledEvent, data interface{}) (retry bool, err error) {
	dataCast := data.(*AddRoleData)
	err = common.BotSession.GuildMemberRoleAdd(evt.GuildID, dataCast.UserID, dataCast.RoleID)
	if err != nil {
		return CheckDiscordErrRetry(err), err
	}

	return CheckDiscordErrRetry(err), err
}
