package soundboard

//go:generate sqlboiler --no-hooks psql

import (
	"fmt"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/premium"
	"github.com/jonas747/yagpdb/soundboard/models"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"golang.org/x/net/context"
	"os"
)

type Plugin struct{}

func (p *Plugin) Name() string {
	return "Soundboard"
}

func RegisterPlugin() {
	_, err := common.PQ.Exec(DBSchema)
	if err != nil {
		logrus.WithError(err).Error("failed initializing soundbaord database schema, not running...")
		return
	}

	p := &Plugin{}
	common.RegisterPlugin(p)

	// Setup directories
	err = os.MkdirAll("soundboard/queue", 0755)
	if err != nil {
		if !os.IsExist(err) {
			panic(err)
		}
	}
	os.MkdirAll("soundboard/ready", 0755)
}

type TranscodingStatus int

const (
	// In the transcoding queue
	TranscodingStatusQueued TranscodingStatus = iota
	// Done transcoding and ready to be played
	TranscodingStatusReady
	// Currently transcoding
	TranscodingStatusTranscoding
	// Failed transcoding, too long
	TranscodingStatusFailedLong
	// Failed transcofing, contact support
	TranscodingStatusFailedOther
)

func (s TranscodingStatus) String() string {
	switch s {
	case TranscodingStatusQueued:
		return "Queued"
	case TranscodingStatusReady:
		return "Ready"
	case TranscodingStatusFailedLong:
		return "Failed: Too long (max 10sec)"
	case TranscodingStatusFailedOther:
		return "Failed, contact support"
	}
	return "Unknown"
}

func CanPlaySound(s *models.SoundboardSound, roles []int64) bool {
	if s.RequiredRole == "" {
		return true
	}

	for _, v := range roles {
		if discordgo.StrID(v) == s.RequiredRole {
			return true
		}
	}

	return false
}

func KeySoundLock(id int) string {
	return fmt.Sprintf("soundboard_soundlock:%d", id)
}

func SoundFilePath(id int, status TranscodingStatus) string {
	if status == TranscodingStatusReady {
		return fmt.Sprintf("soundboard/ready/%d.dca", id)
	}

	return fmt.Sprintf("soundboard/queue/%d", id)
}

const (
	MaxGuildSounds        = 50
	MaxGuildSoundsPremium = 250
)

func MaxSoundsForContext(ctx context.Context) int {
	if premium.ContextPremium(ctx) {
		return MaxGuildSoundsPremium
	}

	return MaxGuildSounds
}

func GetSoundboardSounds(guildID int64, ctx context.Context) ([]*models.SoundboardSound, error) {
	result, err := models.SoundboardSounds(qm.Where("guild_id=?", guildID)).AllG(ctx)
	return result, err
}
