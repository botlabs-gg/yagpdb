package soundboard

//go:generate sqlboiler --no-hooks psql

import (
	"fmt"
	"os"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/soundboard/models"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"golang.org/x/net/context"
)

type Plugin struct{}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Soundboard",
		SysName:  "soundboard",
		Category: common.PluginCategoryMisc,
	}
}

var logger = common.GetPluginLogger(&Plugin{})

func RegisterPlugin() {
	common.InitSchemas("soundboard", DBSchemas...)

	p := &Plugin{}
	common.RegisterPlugin(p)

	// Setup directories
	err := os.MkdirAll("soundboard/queue", 0755)
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
	if len(s.RequiredRoles) > 0 && !common.ContainsInt64SliceOneOf(roles, s.RequiredRoles) {
		return false
	}

	if common.ContainsInt64SliceOneOf(roles, s.BlacklistedRoles) {
		return false
	}

	return true
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
