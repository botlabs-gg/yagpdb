package premium

import (
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix.v3"
	"github.com/pkg/errors"
	"strconv"
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

// Hash
// The key is either a guild the user proides premium status to or "available", with the value of that being the total number of premium slots this user has available
func RedisKeyPremiumUser(userID int64) string { return "premium_user" }

// IsGuildPremium return true if the provided guild has the premium status provided to it by a user
func IsGuildPremium(guildID int64) (bool, error) {
	var premium bool
	err := common.RedisPool.Do(radix.FlatCmd(&premium, "HEXISTS", RedisKeyPremiumGuilds, guildID))
	return premium, errors.WithMessage(err, "IsGuildPremium")
}

// UserPremiumStat returns slots used and total available slots
func UserPremiumStat(userID int64) (used []int64, available int, err error) {
	var usermap map[string]string
	err = common.RedisPool.Do(radix.FlatCmd(&premium, "HGETALL", RedisKeyPremiumUser, guildID))
	if err != nil {
		err = errors.WithMessage(err, "UserPremiumStat")
		return
	}

	if v, ok := usermap["available"]; ok {
		available, _ = strconv.ParseInt(v, 10, 32)
	}

	used = make([]int64, 0, len(usermap))
	for k, _ := range usermap {
		if k != "available" {
			parsed, _ := strconv.ParseInt(k, 10, 64)
			used = append(used, parsed)
		}
	}

	return
}
