package soundboard

import (
	"io"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dca"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
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

	players   = make(map[int64]*Player)
	playersmu = sync.NewCond(&sync.Mutex{})
)

// RequestPlaySound either queues up a sound to be played in an existing player or creates a new one
func RequestPlaySound(guildID int64, channelID, channelRanFrom int64, soundID int) (queued bool) {
	item := &PlayRequest{
		ChannelID:      channelID,
		GuildID:        guildID,
		Sound:          soundID,
		CommandRanFrom: channelRanFrom,
	}

	playersmu.L.Lock()
	if p, ok := players[guildID]; ok {
		// add to existing player queue
		p.queue = append(p.queue, item)
		queued = true
	} else {
		// create new player
		p = &Player{
			ChannelID: channelID,
			GuildID:   guildID,
			queue:     []*PlayRequest{item},
		}
		players[guildID] = p
		go p.Run()
	}

	playersmu.L.Unlock()

	// wake up all players to recheck their queues
	playersmu.Broadcast()

	return
}

func resetPlayerServer(guildID int64) string {
	playersmu.L.Lock()

	if p, ok := players[guildID]; ok {
		p.stop = true
		playersmu.L.Unlock()
		playersmu.Broadcast()
		return ""
	}
	playersmu.L.Unlock()

	return "No active Player, nothing to reset."
}

// Player represends a voice connection playing a soundbaord file (or waiting for one)
type Player struct {
	GuildID int64

	// below fields are safe to access with playersmu
	ChannelID    int64
	queue        []*PlayRequest
	timeLastPlay time.Time
	playing      bool
	stop         bool

	// below fields are only safe to deal with in the main run goroutine
	vc *discordgo.VoiceConnection
}

// Run runs the main player goroutine
func (p *Player) Run() {
	go p.checkIdleTooLong()

	for {
		p.waitForNextElement()
		if p.stop {
			playersmu.L.Unlock()
			return
		}

		item := p.queue[0]
		p.queue = p.queue[1:]

		// check if we need to change voice channel
		changeChannel := false
		if p.ChannelID != item.ChannelID {
			changeChannel = true
			p.ChannelID = item.ChannelID
		}

		p.playing = true
		p.timeLastPlay = time.Now()
		playersmu.L.Unlock()

		var err error
		p.vc, err = playSound(p, p.vc, bot.ShardManager.SessionForGuild(p.GuildID), item, changeChannel)
		if err != nil {
			logger.WithError(err).WithField("guild", p.GuildID).Error("Failed playing sound")
			if item.CommandRanFrom != 0 {
				common.BotSession.ChannelMessageSend(item.CommandRanFrom, "Failed playing the sound: `"+err.Error()+"` make sure you put a proper audio file, and did not for example link to a youtube video.")
			}
		}
	}
}

// cond.L is locked when this returns
func (p *Player) waitForNextElement() {
	playersmu.L.Lock()
	p.playing = false
	for {
		if p.stop {
			p.exit()
			return
		}

		if len(p.queue) < 1 {
			playersmu.Wait()
			continue
		}

		break
	}
}

func (p *Player) exit() {
	if p.vc != nil {
		p.vc.Disconnect()
	}
	if len(p.queue) > 0 {
		p.queue = nil
	}
	delete(players, p.GuildID)
}

func (p *Player) checkIdleTooLong() {
	t := time.NewTicker(time.Second * 10)
	defer t.Stop()
	for {
		<-t.C

		playersmu.L.Lock()
		if p.stop {
			playersmu.L.Unlock()
			return
		}
		if time.Since(p.timeLastPlay) > time.Minute && len(p.queue) < 1 && !p.playing {
			p.stop = true
			playersmu.Broadcast()
			playersmu.L.Unlock()
			return
		}

		playersmu.L.Unlock()
	}
}

func playSound(p *Player, vc *discordgo.VoiceConnection, session *discordgo.Session, req *PlayRequest, changeChannel bool) (*discordgo.VoiceConnection, error) {
	logger.Info("Playing sound ", req.Sound)

	// Open the sound and create a new decoder
	reader, err := getSoundFromBGWorker(req.Sound)
	if err != nil {
		return vc, common.ErrWithCaller(err)
	}
	defer reader.Close()

	decoder := dca.NewDecoder(reader)

	// Either use the passed voice connection, or create a new one
	if changeChannel || vc == nil || !vc.Ready {
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
	err = sendSilence(vc, 3)
	if err != nil {
		return vc, common.ErrWithCaller(err)
	}

	// Then play the actual sound
	for {
		playersmu.L.Lock()
		if p.stop {
			playersmu.L.Unlock()
			return vc, nil
		}
		playersmu.L.Unlock()

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
