package premium

import (
	"context"
	"database/sql"
	"time"

	"emperror.dev/errors"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/config"
	"github.com/jonas747/yagpdb/common/scheduledevents2"
	schEventsModels "github.com/jonas747/yagpdb/common/scheduledevents2/models"
	"github.com/jonas747/yagpdb/common/templates"
	"github.com/jonas747/yagpdb/premium/models"
	"github.com/mediocregopher/radix/v3"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

const (
	// Hash
	// Key: guild id's
	// Value: the user id's providing the premium status
	RedisKeyPremiumGuilds          = "premium_activated_guilds"
	RedisKeyPremiumGuildsTmp       = "premium_activated_guilds_tmp"
	RedisKeyPremiumGuildLastActive = "premium_guild_last_active"
)

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

// IsGuildPremium return true if the provided guild has the premium status provided to it by a user
func IsGuildPremium(guildID int64) (bool, error) {
	if confAllGuildsPremium.GetBool() {
		return true, nil
	}

	// for testing for example
	if common.RedisPool == nil {
		return false, nil
	}

	var premium bool
	err := common.RedisPool.Do(radix.FlatCmd(&premium, "HEXISTS", RedisKeyPremiumGuilds, guildID))
	return premium, errors.WithMessage(err, "IsGuildPremium")
}

type CacheKey int

const CacheKeyIsPremium CacheKey = 1

// IsGuildPremium return true if the provided guild has the premium status provided to it by a user
func IsGuildPremiumCached(guildID int64) (bool, error) {
	if confAllGuildsPremium.GetBool() {
		return true, nil
	}

	if !bot.Enabled || bot.State == nil {
		return IsGuildPremium(guildID)
	}

	gs := bot.State.Guild(true, guildID)
	if gs == nil {
		return IsGuildPremium(guildID)
	}

	v, err := gs.UserCacheFetch(CacheKeyIsPremium, func() (interface{}, error) {
		return IsGuildPremium(guildID)
	})

	if err != nil {
		return false, err
	}

	return v.(bool), nil
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

// UserPremiumSlots returns all slots for a user
func UserPremiumSlots(ctx context.Context, userID int64) (slots []*models.PremiumSlot, err error) {
	slots, err = models.PremiumSlots(qm.Where("user_id = ?", userID), qm.OrderBy("id desc")).AllG(ctx)
	return
}

var (
	PremiumSources []PremiumSource

	ErrSlotNotFound        = errors.New("premium slot not found")
	ErrGuildAlreadyPremium = errors.New("guild already assigned premium from another slot")
)

type PremiumSource interface {
	Init()
	Names() (human string, idname string)
}

func RegisterPremiumSource(source PremiumSource) {
	PremiumSources = append(PremiumSources, source)
}

func SlotExpired(ctx context.Context, slot *models.PremiumSlot) error {
	err := DetachSlotFromGuild(ctx, slot.ID, slot.UserID)
	if err != nil {
		return errors.WithMessage(err, "Detach")
	}

	// Attempt migrating the guild attached to the epxired slot to the next available slot the owner of the slot has
	tx, err := common.PQ.BeginTx(ctx, nil)
	if err != nil {
		return errors.WithMessage(err, "BeginTX")
	}

	availableSlot, err := models.PremiumSlots(qm.Where("user_id = ? AND guild_id IS NULL and permanent = true", slot.UserID), qm.For("UPDATE")).One(ctx, tx)
	if err != nil {
		tx.Rollback()

		// If there's no available slots to migrate the guild to, not much can be done
		if errors.Cause(err) == sql.ErrNoRows {
			return nil
		}

		return errors.WithMessage(err, "models.PremiumSlots")
	}

	availableSlot.AttachedAt = null.TimeFrom(time.Now())
	availableSlot.GuildID = slot.GuildID

	_, err = availableSlot.Update(ctx, tx, boil.Whitelist("attached_at", "guild_id"))
	if err != nil {
		tx.Rollback()
		return errors.WithMessage(err, "Update")
	}

	err = tx.Commit()
	if err != nil {
		return errors.WithMessage(err, "Commit")
	}

	err = common.RedisPool.Do(radix.FlatCmd(nil, "HSET", RedisKeyPremiumGuilds, slot.GuildID.Int64, slot.UserID))
	return errors.WithMessage(err, "HSET")
}

// RemovePremiumSlots removes the specifues premium slots and attempts to migrate to other permanent available ones
// THIS SHOULD BE USED INSIDE A TRANSACTION ONLY, AS OTHERWISE RACE CONDITIONS BE UPON THEE
func RemovePremiumSlots(ctx context.Context, exec boil.ContextExecutor, userID int64, slotsToRemove []int64) error {
	userSlots, err := models.PremiumSlots(qm.Where("user_id = ?", userID), qm.OrderBy("id desc"), qm.For("UPDATE")).All(ctx, exec)
	if err != nil {
		return errors.WithMessage(err, "models.PremiumSlots")
	}

	// Find the remainign free slots after the removal of the specified slots
	remainingFreeSlots := make([]*models.PremiumSlot, 0)
	for _, slot := range userSlots {
		if slot.GuildID.Valid || !slot.Permanent || SlotDurationLeft(slot) <= 0 {
			continue
		}

		for _, v := range slotsToRemove {
			if v == slot.ID {
				continue
			}
		}

		remainingFreeSlots = append(remainingFreeSlots, slot)
	}

	freeSlotsUsed := 0

	// Do the removal and migration
	for _, removing := range slotsToRemove {
		// Find the model first
		var slot *models.PremiumSlot
		for _, v := range userSlots {
			if v.ID == removing {
				slot = v
				break
			}
		}

		if slot == nil {
			continue
		}

		if slot.GuildID.Valid && freeSlotsUsed < len(remainingFreeSlots) {
			// We can migrate it
			remainingFreeSlots[freeSlotsUsed].GuildID = slot.GuildID
			remainingFreeSlots[freeSlotsUsed].AttachedAt = null.TimeFrom(time.Now())
			freeSlotsUsed++
		}

		_, err = slot.Delete(ctx, exec)
		if err != nil {
			return errors.WithMessage(err, "slot.Delete")
		}
	}

	// Update all the slots we migrated to
	for i := 0; i < freeSlotsUsed; i++ {
		_, err = remainingFreeSlots[i].Update(ctx, exec, boil.Whitelist("guild_id", "attached_at"))
		if err != nil {
			return errors.WithMessage(err, "remainingFreeSlots.Update")
		}
	}

	return nil
}

func CreatePremiumSlot(ctx context.Context, exec boil.ContextExecutor, userID int64, source, title, message string, sourceSlotID int64, duration time.Duration) (*models.PremiumSlot, error) {
	slot := &models.PremiumSlot{
		UserID:   userID,
		Source:   source,
		SourceID: sourceSlotID,

		Title:   title,
		Message: message,

		FullDuration:      int64(duration),
		Permanent:         duration <= 0,
		DurationRemaining: int64(duration),
	}

	err := slot.Insert(ctx, exec, boil.Infer())
	return slot, err
}

func FindSource(sourceID string) PremiumSource {
	for _, v := range PremiumSources {
		if _, id := v.Names(); id == sourceID {
			return v
		}
	}

	return nil
}

func SlotDurationLeft(slot *models.PremiumSlot) (duration time.Duration) {
	if slot.Permanent {
		return 0xfffffffffffffff
	}

	duration = time.Duration(slot.DurationRemaining)

	if slot.GuildID.Valid {
		duration -= time.Since(slot.AttachedAt.Time)
	}

	return duration
}

func AttachSlotToGuild(ctx context.Context, slotID int64, userID int64, guildID int64) error {
	tx, err := common.PQ.BeginTx(ctx, nil)
	if err != nil {
		return errors.WithMessage(err, "BeginTX")
	}

	_, err = tx.Exec("LOCK TABLE premium_slots IN EXCLUSIVE MODE")
	if err != nil {
		tx.Rollback()
		return errors.WithMessage(err, "Lock")
	}

	// Check if this guild is used in another slot
	n, err := models.PremiumSlots(qm.Where("guild_id = ?", guildID)).Count(ctx, tx)
	if err != nil {
		tx.Rollback()
		return errors.WithMessage(err, "PremiumSlots.Count")
	}

	if n > 0 {
		tx.Rollback()
		return ErrGuildAlreadyPremium
	}

	n, err = models.PremiumSlots(qm.Where("id = ? AND user_id = ? AND guild_id IS NULL AND (permanent OR duration_remaining > 0)", slotID, userID)).UpdateAll(
		ctx, tx, models.M{"guild_id": null.Int64From(guildID), "attached_at": time.Now()})
	if err != nil {
		tx.Rollback()
		return errors.WithMessage(err, "UpdateAll")
	}

	if n < 1 {
		tx.Rollback()
		return ErrSlotNotFound
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return errors.WithMessage(err, "Commit")
	}

	err = common.RedisPool.Do(radix.FlatCmd(nil, "HSET", RedisKeyPremiumGuilds, guildID, userID))
	if err != nil {
		return errors.WithStackIf(err)
	}

	err = scheduledevents2.ScheduleEvent("premium_guild_added", guildID, time.Now(), nil)
	if err != nil {
		return errors.WithStackIf(err)
	}
	return err
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

func DetachSlotFromGuild(ctx context.Context, slotID int64, userID int64) error {
	tx, err := common.PQ.BeginTx(ctx, nil)
	if err != nil {
		return errors.WithMessage(err, "BeginTX")
	}

	slot, err := models.PremiumSlots(qm.Where("id = ? AND user_id = ?", slotID, userID), qm.For("UPDATE")).One(ctx, tx)
	if err != nil {
		tx.Rollback()
		return errors.WithMessage(err, "PremiumSlots.One")
	}

	if slot == nil {
		tx.Rollback()
		return ErrSlotNotFound
	}

	oldGuildID := slot.GuildID.Int64

	// Update the duration before we reset the guild_id to null
	slot.DurationRemaining = int64(SlotDurationLeft(slot))
	slot.GuildID = null.Int64{}
	slot.AttachedAt = null.Time{}

	_, err = slot.Update(ctx, tx, boil.Infer())
	if err != nil {
		tx.Rollback()
		return errors.WithMessage(err, "Update")
	}

	err = tx.Commit()
	if err != nil {
		errors.WithMessage(err, "Commit")
	}

	err = common.RedisPool.Do(radix.FlatCmd(nil, "HDEL", RedisKeyPremiumGuilds, oldGuildID))
	if err != nil {
		return errors.WithStackIf(err)
	}

	err = scheduledevents2.ScheduleEvent("premium_guild_removed", oldGuildID, time.Now(), nil)
	if err != nil {
		return errors.WithStackIf(err)
	}

	return nil
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
