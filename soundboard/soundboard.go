package soundboard

import (
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/configstore"
	"github.com/jonas747/yagpdb/web"
	"golang.org/x/net/context"
	"os"
)

type Plugin struct{}

// GetGuildConfig returns a GuildConfig item from db
func (p *Plugin) GetGuildConfig(ctx context.Context, guildID string, dest configstore.GuildConfig) (err error) {
	cast := dest.(*SoundboardConfig)

	err = common.GORM.Where(common.MustParseInt(guildID)).First(cast).Error
	if err != nil {
		// Return default config if not found
		if err == gorm.ErrRecordNotFound {
			*cast = SoundboardConfig{
				GuildConfigModel: configstore.GuildConfigModel{
					GuildID: common.MustParseInt(guildID),
				},
			}
		} else {
			return err
		}
	}

	err = common.GORM.Where("guild_id = ?", guildID).Find(&cast.Sounds).Error
	return err
}

// SetGuildConfig saves the GuildConfig struct
func (p *Plugin) SetGuildConfig(ctx context.Context, conf configstore.GuildConfig) error {
	return common.GORM.Save(conf).Error
}

// SetIfLatest saves it only if the passedLatest time is the latest version
func (p *Plugin) SetIfLatest(ctx context.Context, conf configstore.GuildConfig) (updated bool, err error) {
	err = p.SetGuildConfig(ctx, conf)
	return true, err
}

func (p *Plugin) Name() string {
	return "Soundboard"
}

func RegisterPlugin() {
	p := &Plugin{}

	web.RegisterPlugin(p)
	bot.RegisterPlugin(p)

	configstore.RegisterConfig(p, &SoundboardConfig{})
	common.GORM.AutoMigrate(SoundboardConfig{}, SoundboardSound{})

	// Setup directories
	err := os.MkdirAll("soundboard/queue", 0755)
	if err != nil {
		if !os.IsExist(err) {
			panic(err)
		}
	}
	os.MkdirAll("soundboard/ready", 0755)
}

type SoundboardConfig struct {
	configstore.GuildConfigModel

	Sounds []*SoundboardSound `gorm:"ForeignKey:GuildID;AssociationForeignKey:GuildID"`
}

func (sc *SoundboardConfig) GetName() string {
	return "soundboard"
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
		return "Qeued"
	case TranscodingStatusReady:
		return "Ready"
	case TranscodingStatusFailedLong:
		return "Failed: Too long (max 10sec)"
	case TranscodingStatusFailedOther:
		return "Failed, contact support"
	}
	return "Unknown"
}

type SoundboardSound struct {
	common.SmallModel

	GuildID      int64  `gorm:"index"`
	RequiredRole string `valid:"role,true"`
	Name         string `valid:",1,100"`
	Status       TranscodingStatus
}

func (s *SoundboardSound) CanPlay(roles []string) bool {
	if s.RequiredRole == "" {
		return true
	}

	for _, v := range roles {
		if v == s.RequiredRole {
			return true
		}
	}

	return false
}

func KeySoundLock(id uint) string {
	return fmt.Sprintf("soundboard_soundlock:%d", id)
}

func SoundFilePath(id uint, status TranscodingStatus) string {
	if status == TranscodingStatusReady {
		return fmt.Sprintf("soundboard/ready/%d.dca", id)
	}

	return fmt.Sprintf("soundboard/queue/%d", id)
}
