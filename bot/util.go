package bot

import (
	"github.com/jonas747/discordgo"
)

func GetCreatePrivateChannel(s *discordgo.Session, user string) (*discordgo.Channel, error) {
	for _, v := range s.State.PrivateChannels {
		if v.Recipient.ID == user {
			return v, nil
		}
	}

	channel, err := s.UserChannelCreate(user)
	if err != nil {
		return nil, err
	}

	return channel, nil
}
