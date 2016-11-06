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

// var (
// 	LoadGuildMembersQueue = make(chan string)
// )

// func guildMembersRequester() {

// 	client, err := common.RedisPool.Get()
// 	if err != nil {
// 		panic(err)
// 	}

// 	for {
// 		g := <-LoadGuildMembersQueue

// 		// Reset this stat
// 		err := client.Cmd("SET", "guild_stats_num_members:"+g, 0).Err
// 		if err != nil {
// 			logrus.WithError(err).Error("Failed resetting guild members stat")
// 		}

// 		err = common.BotSession.RequestGuildMembers(g, "", 0)
// 		if err != nil {
// 			// Put it back into the queue if an error occured
// 			logrus.WithError(err).WithField("guild", g).Error("Failed requesting guild members")
// 			go func() {
// 				LoadGuildMembersQueue <- g
// 			}()
// 		}

// 		time.Sleep(time.Second)
// 	}
// }
