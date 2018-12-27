package scheduledevents2

import (
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common/scheduledevents2/models"
	"time"
)

type DeleteMessagesEvent struct {
	GuildID   int64
	ChannelID int64
	Messages  []int64
}

func init() {
	RegisterHandler("delete_messages", DeleteMessagesEvent{}, handleDeleteMessagesEvent)
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

	bot.MessageDeleteQueue.DeleteMessages(dataCast.ChannelID, dataCast.Messages...)
	return false, nil
}
