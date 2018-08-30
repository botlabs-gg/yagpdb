package premium

import (
	"context"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix.v3"
	"github.com/pkg/errors"
	"strconv"
	"time"
)

const (
	// Hash
	// Key: guild id's
	// Value: the user id's providing the premium status
	RedisKeyPremiumGuilds = "premium_activated_guilds"

	// Set
	// values: user id's
	RedisKeyPremiumUsers = "premium_users"
)

// String/Byte array
// Json object of []PremiumSlots, the premium slots this user has
func RedisKeyPremiumUser(userID int64) string { return "premium_user:" + strconv.FormatInt(userID, 10) }

type PremiumSlot struct {
	ID      int64  `json:"id"` // The slot id may be the same as others if the source is different, as its a per source thing
	Source  string `json:"source"`
	Message string `json:"message"` // An optinal message for this slot, maybe it was from a giveaway or something

	GuildID int64 `json:"guild_id"` // If this slot is
	UserID  int64 `json:"user_id"`

	Temporary    bool          `json:"temporary"`  // Wether this slot is fixed monthly (eg patreon slots) or something thats given from say giveaways
	DurationLeft time.Duration `json:"hours_left"` // How much time is left of this slot, if temporary
}

// IsGuildPremium return true if the provided guild has the premium status provided to it by a user
func IsGuildPremium(guildID int64) (bool, error) {
	var premium bool
	err := common.RedisPool.Do(radix.FlatCmd(&premium, "HEXISTS", RedisKeyPremiumGuilds, guildID))
	return premium, errors.WithMessage(err, "IsGuildPremium")
}

// UserPremiumSlots returns all slots for a user
func UserPremiumSlots(userID int64) (slots []*PremiumSlot, err error) {
	err = common.GetRedisJson(RedisKeyPremiumUser(userID), &slots)
	err = errors.WithMessage(err, "UserPremiumSlots")
	return
}

func AllPremiumUsers() (users []int64, err error) {
	err = common.RedisPool.Do(radix.Cmd(&users, "SMEMBERS", RedisKeyPremiumUsers))
	return users, errors.WithMessage(err, "AllPremiumUsers")
}

var (
	PremiumSources []PremiumSource
)

type PremiumSource interface {
	Init()
	Names() (human string, idname string)
	AllUserSlots(ctx context.Context) (userSlots map[int64][]*PremiumSlot, err error)
	SlotsForUser(ctx context.Context, userID int64) (slots []*PremiumSlot, err error)
	AttachSlot(ctx context.Context, userID int64, slotID int64, guildID int64) (err error)
	DetachSlot(ctx context.Context, userID int64, slotID int64) (err error)
}

func RegisterPremiumSource(source PremiumSource) {
	PremiumSources = append(PremiumSources, source)
}

func SlotExpired(userID, guildID int64) {
	// TODO
}
