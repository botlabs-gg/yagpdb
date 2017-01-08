package soundboard

import (
	"github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/dca"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/configstore"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"sync"
	"time"
)

var (
	transcoderStop    = make(chan *sync.WaitGroup)
	transcoderOptions *dca.EncodeOptions
)

func init() {
	// Copy the standard options
	cp := *dca.StdEncodeOptions
	transcoderOptions = &cp
}

func (p *Plugin) StartBot() {
	go transcoderLoop()
}

func (p *Plugin) StopBot(wg *sync.WaitGroup) {
	logrus.Info("Stopping soundboard transcoder...")

	transcoderStop <- wg
}

func transcoderLoop() {
	ticker := time.NewTicker(time.Second)
	redisClient := common.MustGetRedisClient()
	for {
		select {
		case wg := <-transcoderStop:
			wg.Done()
			return
		case <-ticker.C:
			items := getQueue()
			for _, v := range items {
				started := time.Now()
				logrus.Println("handling queue item")
				err := handleQueueItem(redisClient, v)
				logrus.Println("done handling queue item")
				if err != nil {
					logrus.WithError(err).WithField("soundid", v).Error("Failed processing transcode queue item")
				}
				logrus.WithField("sounf", v).Info("Took ", time.Since(started).String(), " to transcode sound ")
			}
		}
	}
}

func getQueue() []string {
	files, err := ioutil.ReadDir("soundboard/queue")
	if err != nil {
		logrus.WithError(err).Error("Failed checking queue directory")
		return []string{}
	}

	out := make([]string, len(files))

	for k, v := range files {
		out[k] = v.Name()
	}

	return out
}

func handleQueueItem(client *redis.Client, item string) error {
	parsedId, err := strconv.ParseInt(item, 10, 32)
	if err != nil {
		return err
	}

	// lock it for max 10 minutes, after that something must've gone wrong
	locked, err := common.TryLockRedisKey(client, KeySoundLock(uint(parsedId)), 10*60)
	if err != nil {
		return err
	}
	if !locked {
		logrus.WithField("sound", parsedId).Warn("Sound is busy, handling it later")
		return nil
	}
	defer common.UnlockRedisKey(client, KeySoundLock(uint(parsedId)))

	var sound SoundboardSound
	err = common.SQL.Where(uint(parsedId)).First(&sound).Error
	if err != nil {
		return err
	}

	logrus.WithField("sound", sound.ID).Info("Handling queued sound ", sound.Name)
	err = transcodeSound(&sound)
	if err != nil {
		logrus.WithError(err).WithField("sound", sound.ID).Error("Failed transcodiing sound")
		common.SQL.Model(&sound).Update("Status", TranscodingStatusFailedOther)
		os.Remove(SoundFilePath(sound.ID, TranscodingStatusReady))
	} else {
		common.SQL.Model(&sound).Update("Status", TranscodingStatusReady)
	}

	configstore.InvalidateGuildCache(client, sound.GuildID, &SoundboardConfig{})
	err = os.Remove(SoundFilePath(sound.ID, TranscodingStatusQueued))
	return err
}

func transcodeSound(sound *SoundboardSound) error {
	destFile, err := os.Create(SoundFilePath(sound.ID, TranscodingStatusReady))
	if err != nil {
		return err
	}
	defer destFile.Close()

	session, err := dca.EncodeFile(SoundFilePath(sound.ID, sound.Status), transcoderOptions)
	if err != nil {
		return err
	}

	_, err = io.Copy(destFile, session)
	if err != nil {
		session.Truncate()
		return err
	}
	err = session.Error()
	return err
}
