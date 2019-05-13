package soundboard

import (
	"github.com/jonas747/dca"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/pkg/errors"
	"io"
	"sync"
	"time"
)

type PlayRequest struct {
	ChannelID      int64
	GuildID        int64
	CommandRanFrom int64
	Sound          int
}

var (
	playQueues      = make(map[int64][]*PlayRequest)
	playQueuesMutex sync.Mutex
	Silence         = []byte{0xF8, 0xFF, 0xFE}
)

// RequestPlaySound either queues up a sound to be played in an existing player or creates a new one
func RequestPlaySound(guildID int64, channelID, channelRanFrom int64, soundID int) (queued bool) {
	item := &PlayRequest{
		ChannelID:      channelID,
		GuildID:        guildID,
		Sound:          soundID,
		CommandRanFrom: channelRanFrom,
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

func runPlayer(guildID int64) {
	var lastChannel int64
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
		vc, err = playSound(vc, bot.ShardManager.SessionForGuild(guildID), item)
		if err != nil {
			logger.WithError(err).WithField("guild", guildID).Error("Failed playing sound")
			if item.CommandRanFrom != 0 {
				common.BotSession.ChannelMessageSend(item.CommandRanFrom, "Failed playing the sound: `"+err.Error()+"` make sure you put a proper audio file, and did not for example link to a youtube video.")
			}
		}
		lastChannel = item.ChannelID
	}
	if vc != nil {
		vc.Disconnect()
		vc.Close()
	}

	logger.Info("Done playing")
	// When we break out, playqueuemutex is locked
	delete(playQueues, guildID)
	playQueuesMutex.Unlock()
}

func playSound(vc *discordgo.VoiceConnection, session *discordgo.Session, req *PlayRequest) (*discordgo.VoiceConnection, error) {
	logger.Info("Playing sound ", req.Sound)

	// Open the sound and create a new decoder
	reader, err := getSoundFromBGWorker(req.Sound)
	if err != nil {
		return vc, common.ErrWithCaller(err)
	}
	defer reader.Close()

	decoder := dca.NewDecoder(reader)

	// Either use the passed voice connection, or create a new one
	if vc == nil || !vc.Ready {
		vc, err = session.GatewayManager.ChannelVoiceJoin(req.GuildID, req.ChannelID, false, true)
		if err != nil {
			if err == discordgo.ErrTimeoutWaitingForVoice {
				bot.ShardManager.SessionForGuild(req.GuildID).GatewayManager.ChannelVoiceLeave(req.GuildID)
			}
			return nil, common.ErrWithCaller(err)
		}
		<-vc.Connected
		vc.Speaking(true)
	}

	// Start by sending some frames of silence
	err = sendSilence(vc, 10)
	if err != nil {
		return vc, common.ErrWithCaller(err)
	}

	// Then play the actual sound
	for {
		frame, err := decoder.OpusFrame()
		if err != nil {
			if err != io.EOF {
				return vc, common.ErrWithCaller(err)
			}
			break
		}

		err = sendAudio(vc, frame)
		if err != nil {
			return vc, common.ErrWithCaller(err)
		}
	}

	// And finally stop with another small number of silcece frame
	err = sendSilence(vc, 5)
	if err != nil {
		return vc, common.ErrWithCaller(err)
	}

	return vc, nil
}

// Send n silence frames
func sendSilence(vc *discordgo.VoiceConnection, n int) error {
	for i := n - 1; i >= 0; i-- {
		err := sendAudio(vc, Silence)
		if err != nil {
			return err
		}
	}

	return nil
}

var (
	ErrVoiceSendTimeout = errors.New("Voice send timeout")
)

func sendAudio(vc *discordgo.VoiceConnection, frame []byte) error {
	select {
	case vc.OpusSend <- frame:
	case <-time.After(time.Second):
		return ErrVoiceSendTimeout
	}

	return nil
}
