package soundboard

import (
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/dca"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"io"
	"os"
	"sync"
	"time"
)

type PlayRequest struct {
	ChannelID string
	GuildID   string
	Sound     uint
}

var (
	playQueues      = make(map[string][]*PlayRequest)
	playQueuesMutex sync.Mutex
)

func RequestPlaySound(guildID string, channelID string, soundID uint) (queued bool) {
	item := &PlayRequest{
		ChannelID: channelID,
		GuildID:   guildID,
		Sound:     soundID,
	}

	// If there is a queue setup there is alaso a player running, so just add it to the queue then
	playQueuesMutex.Lock()
	if queue, ok := playQueues[guildID]; ok {
		playQueues[guildID] = append(queue, item)
		queued = true
	} else {
		playQueues[guildID] = []*PlayRequest{item}
		go runPlayer(guildID)
	}
	playQueuesMutex.Unlock()
	return
}

func runPlayer(guildID string) {
	lastChannel := ""
	var vc *discordgo.VoiceConnection
	for {
		playQueuesMutex.Lock()
		var item *PlayRequest

		// Get the next item in the queue or quit life
		if queue, ok := playQueues[guildID]; ok && len(queue) > 0 {
			item = queue[0]
			playQueues[guildID] = queue[1:]
		} else {
			break
		}

		playQueuesMutex.Unlock()
		// Should probably to changechannel but eh..
		if lastChannel != item.ChannelID && vc != nil {
			vc.Disconnect()
			vc.Close()
			vc = nil
		}

		var err error
		vc, err = playSound(vc, common.BotSession, item)
		if err != nil {
			logrus.WithError(err).WithField("guild", guildID).Error("Failed playing sound")
		}
		lastChannel = item.ChannelID
	}
	if vc != nil {
		vc.Disconnect()
		vc.Close()
	}

	logrus.Info("Done playing")
	// When we break out, playqueuemutex is locked
	delete(playQueues, guildID)
	playQueuesMutex.Unlock()
}

func playSound(vc *discordgo.VoiceConnection, session *discordgo.Session, req *PlayRequest) (*discordgo.VoiceConnection, error) {
	logrus.Info("Playing sound ", req.Sound)
	file, err := os.Open(SoundFilePath(req.Sound, TranscodingStatusReady))
	if err != nil {
		return vc, err
	}
	defer file.Close()
	decoder := dca.NewDecoder(file)

	if vc == nil || !vc.Ready {
		vc, err = session.ChannelVoiceJoin(req.GuildID, req.ChannelID, false, true)
		if err != nil {
			return nil, err
		}
		<-vc.Connected
		vc.Speaking(true)
	}

	for {
		frame, err := decoder.OpusFrame()
		if err != nil {
			if err != io.EOF {
				return vc, err
			}
			return vc, nil
		}
		select {
		case vc.OpusSend <- frame:
		case <-time.After(time.Second):
			return vc, nil
		}
	}

	return vc, nil
}
