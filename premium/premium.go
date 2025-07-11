package premium

import (
	"fmt"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/config"
	"github.com/botlabs-gg/yagpdb/v2/common/featureflags"
	"github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2"
	schEventsModels "github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2/models"
	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/mediocregopher/radix/v3"
)

const (
	// Hash
	// Key: guild id's
	// Value: the user id's providing the premium status
	RedisKeyPremiumGuilds          = "premium_activated_guilds"
	RedisKeyPremiumGuildsTmp       = "premium_activated_guilds_tmp"
	RedisKeyPremiumGuildLastActive = "premium_guild_last_active"
)

type PremiumTier int

const (
	PremiumTierNone    PremiumTier = 0
	PremiumTierPremium PremiumTier = 1
	PremiumTierPlus    PremiumTier = 2
)

type PremiumSourceType string

const (
	PremiumSourceTypeDiscord PremiumSourceType = "discord"
	PremiumSourceTypePatreon PremiumSourceType = "patreon"
	PremiumSourceTypeCode    PremiumSourceType = "code"
)

func (p PremiumTier) String() string {
	switch p {
	case PremiumTierNone:
		return "None"
	case PremiumTierPlus:
		return "Plus"
	case PremiumTierPremium:
		return "Paid"
	}

	return "unknown"
}

var (
	confAllGuildsPremium = config.RegisterOption("yagpdb.premium.all_guilds_premium", "All servers have premium", false)
)

var logger = common.GetPluginLogger(&Plugin{})

type Plugin struct {
}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Premium",
		SysName:  "premium",
		Category: common.PluginCategoryCore,
	}
}

func RegisterPlugin() {
	common.InitSchemas("premium", DBSchemas...)
	common.RegisterPlugin(&Plugin{})

	scheduledevents2.RegisterHandler("premium_guild_added", nil, handleNewPremiumGuild)
	scheduledevents2.RegisterHandler("premium_guild_removed", nil, handleRemovedPremiumGuild)

	for _, v := range PremiumSources {
		v.Init()
	}

	templates.GuildPremiumFunc = IsGuildPremium
}

type NewPremiumGuildListener interface {
	OnNewPremiumGuild(guildID int64) error
}
type RemovedPremiumGuildListener interface {
	OnRemovedPremiumGuild(guildID int64) error
}

var (
	PremiumSources      []PremiumSource
	GuildPremiumSources []GuildPremiumSource
)

type PremiumSource interface {
	Init()
	Names() (human string, idname string)
}

type GuildPremiumSource interface {
	Name() string
	GuildPremiumDetails(guildID int64) (tier PremiumTier, humanDetails []string, err error)
	GuildPremiumTier(guildID int64) (PremiumTier, error)
	AllGuildsPremiumTiers() (map[int64]PremiumTier, error)
}

func RegisterPremiumSource(source PremiumSource) {
	PremiumSources = append(PremiumSources, source)
}

func RegisterGuildPremiumSource(source GuildPremiumSource) {
	GuildPremiumSources = append(GuildPremiumSources, source)
}

func GuildPremiumTier(guildID int64) (PremiumTier, error) {
	flags, err := featureflags.GetGuildFlags(guildID)
	if err != nil {
		return PremiumTierNone, err
	}

	if common.ContainsStringSlice(flags, FeatureFlagPremiumFull) {
		return PremiumTierPremium, nil
	}

	if common.ContainsStringSlice(flags, FeatureFlagPremiumPlus) {
		return PremiumTierPlus, nil
	}

	return PremiumTierNone, nil
}

// IsGuildPremium return true if the provided guild has the premium status or not
// This is a legacy function mostly as anything equal to and above plus is counted as premium
// if you want more granular control use the other functions
func IsGuildPremium(guildID int64) (bool, error) {
	if confAllGuildsPremium.GetBool() {
		return true, nil
	}

	// for testing for example
	if common.RedisPool == nil {
		return false, nil
	}

	return featureflags.GuildHasFlag(guildID, FeatureFlagPremiumPlus)
}

func PremiumProvidedBy(guildID int64) (int64, error) {
	if confAllGuildsPremium.GetBool() {
		return int64(common.BotUser.ID), nil
	}

	var userID int64
	err := common.RedisPool.Do(radix.FlatCmd(&userID, "HGET", RedisKeyPremiumGuilds, guildID))
	return userID, errors.WithMessage(err, "PremiumProvidedBy")
}

// AllGuildsOncePremium returns all the guilds that have has premium once, and the last time that was active
func AllGuildsOncePremium() (map[int64]time.Time, error) {
	if confAllGuildsPremium.GetBool() {
		return allGuildsOncePremiumAllPremiumEnabled()
	}

	var result []int64
	err := common.RedisPool.Do(radix.Cmd(&result, "ZRANGE", RedisKeyPremiumGuildLastActive, "0", "-1", "WITHSCORES"))
	if err != nil {
		return nil, errors.WrapIf(err, "zrange")
	}

	parsed := make(map[int64]time.Time)
	for i := 0; i < len(result); i += 2 {
		g := result[i]
		score := result[i+1]

		t := time.Unix(score, 0)
		parsed[g] = t
	}

	return parsed, nil
}

func allGuildsOncePremiumAllPremiumEnabled() (map[int64]time.Time, error) {
	var listedServers []int64
	err := common.RedisPool.Do(radix.Cmd(&listedServers, "SMEMBERS", "connected_guilds"))
	if err != nil {
		return nil, err
	}

	results := make(map[int64]time.Time)
	for _, v := range listedServers {
		results[v] = time.Now()
	}

	return results, nil

}

func FindSource(sourceID string) PremiumSource {
	for _, v := range PremiumSources {
		if _, id := v.Names(); id == sourceID {
			return v
		}
	}

	return nil
}

func handleNewPremiumGuild(evt *schEventsModels.ScheduledEvent, data interface{}) (retry bool, err error) {
	for _, v := range common.Plugins {
		if cast, ok := v.(NewPremiumGuildListener); ok {
			err := cast.OnNewPremiumGuild(evt.GuildID)
			if err != nil {
				return scheduledevents2.CheckDiscordErrRetry(err), err
			}
		}
	}

	return false, nil
}

func handleRemovedPremiumGuild(evt *schEventsModels.ScheduledEvent, data interface{}) (retry bool, err error) {
	for _, v := range common.Plugins {
		if cast, ok := v.(RemovedPremiumGuildListener); ok {
			err := cast.OnRemovedPremiumGuild(evt.GuildID)
			if err != nil {
				return scheduledevents2.CheckDiscordErrRetry(err), err
			}
		}
	}

	return false, nil
}

var _ featureflags.PluginWithFeatureFlags = (*Plugin)(nil)
var _ featureflags.PluginWithBatchFeatureFlags = (*Plugin)(nil)

const (
	FeatureFlagPremiumPlus = "premium_plus"
	FeatureFlagPremiumFull = "premium_full"
)

func (p *Plugin) UpdateFeatureFlags(guildID int64) ([]string, error) {
	highestTier := PremiumTierNone

	for _, v := range GuildPremiumSources {
		tier, err := v.GuildPremiumTier(guildID)
		if err != nil {
			return nil, errors.WithMessage(err, "GuildPremiumTier")
		}

		if tier == PremiumTierPremium {
			highestTier = tier
		} else if highestTier == PremiumTierNone {
			highestTier = tier
		}
	}

	return tierFlags(highestTier), nil
}

func (p *Plugin) AllFeatureFlags() []string {
	return []string{
		FeatureFlagPremiumFull, // set if this server has the highest premium tier
		FeatureFlagPremiumPlus, // set if this server has the lower premium tier
	}
}

func (p *Plugin) UpdateFeatureFlagsBatch() (map[int64][]string, error) {
	// fetch all the guild premium tiers and sort the highest one in each
	highestTiers := make(map[int64]PremiumTier)

	for _, v := range GuildPremiumSources {
		guilds, err := v.AllGuildsPremiumTiers()
		if err != nil {
			return nil, errors.WithMessage(err, "AllGuildsPremiumTiers")
		}

		for guildID, tier := range guilds {
			if tier == PremiumTierNone {
				continue
			}

			if current, ok := highestTiers[guildID]; ok {
				if tier == PremiumTierPremium {
					highestTiers[guildID] = tier
				} else if current == PremiumTierNone {
					highestTiers[guildID] = tier
				}
			} else {
				highestTiers[guildID] = tier
			}
		}
	}

	result := make(map[int64][]string)
	for guildID, tier := range highestTiers {
		guildFlags := tierFlags(tier)
		result[guildID] = guildFlags
	}

	return result, nil
}

func tierFlags(tier PremiumTier) []string {
	switch tier {
	case PremiumTierPremium:
		return []string{FeatureFlagPremiumFull, FeatureFlagPremiumPlus}
	case PremiumTierPlus:
		return []string{FeatureFlagPremiumPlus}
	}

	return nil
}

func SendPremiumDM(userID int64, source PremiumSourceType, numSlots int) {
	confSendPatreonPremiumDM := config.RegisterOption("yagpdb.premium.send_patreon_dm", "Send DMs to users when they receive premium slots", false)
	if !confSendPatreonPremiumDM.GetBool() && source == PremiumSourceTypePatreon {
		return
	}
	logger.Infof("Sending premium DM to user: %d for %d slots via %s subscription", userID, numSlots, string(source))
	embed := &discordgo.MessageEmbed{}
	embed.Title = "You have new Premium Slots!"
	embed.Description = fmt.Sprintf("You have received %d new premium slots from a %s subscription!\n\n[Assign them to a server here.](https://%s/premium)", numSlots, string(source), common.ConfHost.GetString())
	err := bot.SendDMEmbed(userID, embed)
	if err != nil {
		logger.WithError(err).Error("Failed sending premium DM")
	}
}
