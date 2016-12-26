package bot

import (
	"errors"
	"github.com/jonas747/discordgo"
	"github.com/patrickmn/go-cache"
	"time"
)

var (
	Cache = cache.New(time.Minute, time.Minute)
)

func GetCreatePrivateChannel(s *discordgo.Session, user string) (*discordgo.Channel, error) {
	// for _, v := range s.State.PrivateChannels {
	// 	if v.Recipient.ID == user {
	// 		return v, nil
	// 	}
	// }

	channel, err := s.UserChannelCreate(user)
	if err != nil {
		return nil, err
	}

	return channel, nil
}

func SendDM(s *discordgo.Session, user string, msg string) error {
	channel, err := GetCreatePrivateChannel(s, user)
	if err != nil {
		return err
	}

	_, err = s.ChannelMessageSend(channel.ID, msg)
	return err
}

var (
	ErrStartingUp = errors.New("Starting up, caches are being filled...")
)
