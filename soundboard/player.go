package soundboard

import (
	"io"
	"runtime/debug"
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

const maxQueuedSounds = 25

var ErrQueueFull = errors.New("the soundboard queue for this server is full, try again once some sounds have played")

// RequestPlaySound either queues up a sound to be played in an existing player or creates a new one
func RequestPlaySound(guildID int64, channelID, channelRanFrom int64, soundID int) (queued bool, err error) {
	item := &PlayRequest{
		ChannelID:      channelID,
		GuildID:        guildID,
		Sound:          soundID,
		CommandRanFrom: channelRanFrom,
	}

	playersmu.L.Lock()
	if p, ok := players[guildID]; ok {
		if len(p.queue)+len(p.afterStop) >= maxQueuedSounds {
			playersmu.L.Unlock()
			return false, ErrQueueFull
		}

		if p.stop {
			// the player is shutting down, a new one picks this up once it has left voice
			p.afterStop = append(p.afterStop, item)
		} else {
			p.queue = append(p.queue, item)
		}
		queued = true
	} else {
		startPlayer(guildID, []*PlayRequest{item})
	}

	playersmu.L.Unlock()

	// wake up all players to recheck their queues
	playersmu.Broadcast()

	return queued, nil
}

// startPlayer must be called with playersmu.L held
func startPlayer(guildID int64, queue []*PlayRequest) {
	p := &Player{
		ChannelID: queue[0].ChannelID,
		GuildID:   guildID,
		queue:     queue,
	}
	players[guildID] = p
	go p.Run()
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
	afterStop    []*PlayRequest // requests made while shutting down

	// below fields are only safe to deal with in the main run goroutine
	vc *discordgo.VoiceConnection
}

// Run runs the main player goroutine
func (p *Player) Run() {
	go p.checkIdleTooLong()

	for {
		p.waitForNextElement()
		if p.stop {
			p.shutdown()
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
		p.vc, err = playSoundRecovered(p, p.vc, bot.ShardManager.SessionForGuild(p.GuildID), item, changeChannel)
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
	for !p.stop && len(p.queue) < 1 {
		playersmu.Wait()
	}
}

// shutdown is called with playersmu.L held and returns with it released. It leaves voice without holding the lock,
// which every server's player shares, and keeps this player registered until then so a new one can't join voice
// in the same server while the old connection is still disconnecting.
func (p *Player) shutdown() {
	vc := p.vc
	p.vc = nil
	p.queue = nil
	playersmu.L.Unlock()

	if vc != nil {
		vc.Disconnect()
	}

	playersmu.L.Lock()
	delete(players, p.GuildID)
	if len(p.afterStop) > 0 {
		startPlayer(p.GuildID, p.afterStop)
		p.afterStop = nil
	}
	playersmu.L.Unlock()
}

// playSoundRecovered turns a panic while playing into an error. playSound never holds playersmu, so recovering
// here can't leave the lock every server's player shares locked.
func playSoundRecovered(p *Player, vc *discordgo.VoiceConnection, session *discordgo.Session, req *PlayRequest, changeChannel bool) (retVC *discordgo.VoiceConnection, err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.WithField("guild", req.GuildID).Warnf("recovered from panic playing sound %d: %v\n%s", req.Sound, r, debug.Stack())
			retVC, err = vc, errors.Errorf("something went wrong playing the sound")
		}
	}()
	return playSound(p, vc, session, req, changeChannel)
}

func (p *Player) checkIdleTooLong() {
	defer func() {
		if r := recover(); r != nil {
			logger.WithField("guild", p.GuildID).Warnf("recovered from panic in soundboard idle checker: %v\n%s", r, debug.Stack())
		}
	}()

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
		select {
		case <-vc.Connected:
		case <-time.After(voiceConnectTimeout):
			session.GatewayManager.ChannelVoiceLeave(req.GuildID)
			return nil, ErrVoiceConnectTimeout
		}
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

const voiceConnectTimeout = 10 * time.Second

var (
	ErrVoiceSendTimeout    = errors.New("Voice send timeout")
	ErrVoiceConnectTimeout = errors.New("timed out connecting to the voice channel")
)

func sendAudio(vc *discordgo.VoiceConnection, frame []byte) error {
	select {
	case vc.OpusSend <- frame:
	case <-time.After(time.Second):
		return ErrVoiceSendTimeout
	}

	return nil
}
