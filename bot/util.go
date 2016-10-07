package bot

import (
	"errors"
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"time"
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

var (
	ErrStartingUp = errors.New("Starting up, caches are being filled...")
)

var (
	LoadGuildMembersQueue = make(chan string)
)

func guildMembersRequester() {
	for {
		g := <-LoadGuildMembersQueue

		err := common.BotSession.RequestGuildMembers(g, "", 0)
		if err != nil {
			// Put it back into the queue if an error occured
			logrus.WithError(err).WithField("guild", g).Error("Failed requesting guild members")
			go func() {
				LoadGuildMembersQueue <- g
			}()
		}

		time.Sleep(time.Second)
	}
}
