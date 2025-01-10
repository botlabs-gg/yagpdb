package premium

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/featureflags"
	"github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2"
	"github.com/botlabs-gg/yagpdb/v2/premium/models"
	"github.com/mediocregopher/radix/v3"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

func init() {
	RegisterGuildPremiumSource(&SlotGuildPremiumSource{})
}

var _ GuildPremiumSource = (*SlotGuildPremiumSource)(nil)

type SlotGuildPremiumSource struct{}

func (s *SlotGuildPremiumSource) Name() string {
	return "User Premium Slot"
}

func (s *SlotGuildPremiumSource) GuildPremiumDetails(guildID int64) (tier PremiumTier, humanDetails []string, err error) {
	slot, err := models.PremiumSlots(qm.Where("guild_id = ?", guildID)).OneG(context.Background())
	if err != nil {
		if err == sql.ErrNoRows {
			return PremiumTierNone, nil, nil
		}

		return PremiumTierNone, nil, err
	}

	tier = PremiumTier(slot.Tier)
	humanDetails = []string{fmt.Sprintf("Premium slot provided by user with the ID of %d", slot.UserID)}
	return
}

func (s *SlotGuildPremiumSource) GuildPremiumTier(guildID int64) (PremiumTier, error) {
	slot, err := models.PremiumSlots(qm.Where("guild_id = ?", guildID)).OneG(context.Background())
	if err != nil {
		if err == sql.ErrNoRows {
			return PremiumTierNone, nil
		}

		return PremiumTierNone, err
	}

	return PremiumTier(slot.Tier), nil
}

func (s *SlotGuildPremiumSource) AllGuildsPremiumTiers() (map[int64]PremiumTier, error) {
	slots, err := models.PremiumSlots(qm.Where("guild_id IS NOT NULL")).AllG(context.Background())
	if err != nil {
		return nil, err
	}

	result := make(map[int64]PremiumTier)
	for _, slot := range slots {
		result[slot.GuildID.Int64] = PremiumTier(slot.Tier)
	}

	return result, nil
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

	err = featureflags.UpdatePluginFeatureFlags(guildID, &Plugin{})
	if err != nil {
		return errors.WithMessage(err, "failed updating plugin feature flags")
	}

	return err
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
		return errors.WithMessage(err, "Commit")
	}

	err = common.RedisPool.Do(radix.FlatCmd(nil, "HDEL", RedisKeyPremiumGuilds, oldGuildID))
	if err != nil {
		return errors.WithStackIf(err)
	}

	err = scheduledevents2.ScheduleEvent("premium_guild_removed", oldGuildID, time.Now(), nil)
	if err != nil {
		return errors.WithStackIf(err)
	}

	err = featureflags.UpdatePluginFeatureFlags(oldGuildID, &Plugin{})
	if err != nil {
		return errors.WithMessage(err, "failed updating plugin feature flags")
	}

	return nil
}

// UserPremiumSlots returns all slots for a user
func UserPremiumSlots(ctx context.Context, userID int64) (slots []*models.PremiumSlot, err error) {
	slots, err = models.PremiumSlots(qm.Where("user_id = ?", userID), qm.OrderBy("id desc")).AllG(ctx)
	return
}

// UserPremiumMarkedDeletedSlots returns all slots marked deleted for a user for a specific source
func UserPremiumMarkedDeletedSlots(ctx context.Context, userID int64, source PremiumSourceType) ([]int64, error) {
	slots, err := models.PremiumSlots(qm.Where("user_id = ? AND deletes_at IS NOT NULL AND source = ?", userID, source), qm.OrderBy("id desc")).AllG(ctx)
	if err == sql.ErrNoRows {
		return []int64{}, nil
	}
	var slotIDs []int64
	for _, slot := range slots {
		slotIDs = append(slotIDs, slot.ID)
	}
	return slotIDs, err
}

var (
	ErrSlotNotFound        = errors.New("premium slot not found")
	ErrGuildAlreadyPremium = errors.New("guild already assigned premium from another slot")
)

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

func CreatePremiumSlot(ctx context.Context, exec boil.ContextExecutor, userID int64, source PremiumSourceType, title, message string, sourceSlotID int64, duration time.Duration, tier PremiumTier) (*models.PremiumSlot, error) {
	slot := &models.PremiumSlot{
		UserID:   userID,
		Source:   string(source),
		SourceID: sourceSlotID,

		Title:   title,
		Message: message,

		FullDuration:      int64(duration),
		Permanent:         duration <= 0,
		DurationRemaining: int64(duration),
		Tier:              int(tier),
	}

	err := slot.Insert(ctx, exec, boil.Infer())
	return slot, err
}

func MarkSlotsForDeletion(ctx context.Context, exec boil.ContextExecutor, userID int64, slotsToRemove []int64) error {
	userSlots, err := models.PremiumSlots(qm.Where("user_id = ? and deletes_at IS NULL", userID), qm.OrderBy("id desc"), qm.For("UPDATE")).All(ctx, exec)
	if err != nil {
		return errors.WithMessage(err, "models.PremiumSlots")
	}
	for _, slot := range userSlots {
		for _, id := range slotsToRemove {
			if slot.ID == id {
				slot.DeletesAt = null.TimeFrom(time.Now().Add(3 * 24 * time.Hour))
				slot.Update(ctx, exec, boil.Whitelist("deletes_at"))
			}
		}
	}
	return nil
}

func CancelSlotDeletionForUser(ctx context.Context, exec boil.ContextExecutor, userID int64, slotsToUndelete []int64) error {
	userSlots, err := models.PremiumSlots(qm.Where("user_id = ? and deletes_at IS NOT NULL", userID), qm.OrderBy("id desc"), qm.For("UPDATE")).All(ctx, exec)
	if err != nil {
		return errors.WithMessage(err, "models.PremiumSlots")
	}
	for _, slot := range userSlots {
		for _, id := range slotsToUndelete {
			if slot.ID == id {
				slot.DeletesAt = null.Time{}
				slot.Update(ctx, exec, boil.Whitelist("deletes_at"))
				logger.Info("Cancelled Deletion for patreon premium slot #", slot.ID, slot.UserID)
			}
		}
	}
	return nil
}

func RemoveMarkedDeletedSlots(ctx context.Context, exec boil.ContextExecutor, source PremiumSourceType) error {
	slots, err := models.PremiumSlots(qm.Where("deletes_at IS NOT NULL AND deletes_at < ? AND source = ? ", time.Now(), source)).All(ctx, exec)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	userSlots := make(map[int64][]int64)
	for _, slot := range slots {
		userSlots[slot.UserID] = append(userSlots[slot.UserID], slot.ID)
	}
	for userID, slotIDs := range userSlots {
		err := RemovePremiumSlots(ctx, exec, userID, slotIDs)
		if err != nil {
			return err
		}
	}
	return nil
}
