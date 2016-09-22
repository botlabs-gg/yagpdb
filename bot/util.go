package bot

import (
	"errors"
	"github.com/jonas747/discordgo"
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

func GetGuildMember(s *discordgo.Session, gID string, uID string) (*discordgo.Member, error) {
	member, err := s.State.Member(gID, uID)
	if err == nil {
		return member, nil
	}

	// If it has been 2 minutes since startup try the api
	// This is currently a workaround
	if time.Since(Started) < time.Minute*2 {
		return nil, ErrStartingUp
	}

	member, err = s.GuildMember(gID, uID)
	if err == nil {
		member.GuildID = gID
		s.State.MemberAdd(member)
	}
	return member, err
}
