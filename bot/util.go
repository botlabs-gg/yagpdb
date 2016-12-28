package bot

import (
	"errors"
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/patrickmn/go-cache"
	"time"
)

var (
	Cache = cache.New(time.Minute, time.Minute)
)

func GetCreatePrivateChannel(s *discordgo.Session, user string) (*discordgo.Channel, error) {

	State.RLock()
	defer State.RUnlock()
	for _, c := range State.PrivateChannels {
		if c.Recipient().ID == user {
			return c.Copy(true, false), nil
		}
	}

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

func GetMember(guildID, userID string) *discordgo.Member {
	gs := State.Guild(true, guildID)
	if gs == nil {
		return nil
	}

	cop := gs.MemberCopy(true, userID, true)
	if cop != nil {
		return cop
	}

	member, err := common.BotSession.GuildMember(guildID, userID)
	if err != nil {
		logrus.WithError(err).WithField("guild", guildID).Error("Failed retrieving guild member")
		return nil
	}

	gs.MemberAddUpdate(true, member)

	return member
}
