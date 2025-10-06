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

func AttachSlotToGuild(ctx context.Context, exec boil.ContextExecutor, slotID int64, userID int64, guildID int64) error {

	n, err := models.PremiumSlots(qm.Where("id = ? AND user_id = ? AND guild_id IS NULL AND (permanent OR duration_remaining > 0)", slotID, userID)).UpdateAll(
		ctx, exec, models.M{"guild_id": null.Int64From(guildID), "attached_at": time.Now()})
	if err != nil {
		logger.Error("Error attaching slot to guild: ", err)
		logger.Error("Slot ID: ", slotID)
		logger.Error("User ID: ", userID)
		logger.Error("Guild ID: ", guildID)
		// If another transaction attached a different slot to this guild concurrently,
		// the partial unique index on guild_id will raise a unique violation.
		if common.ErrPQIsUniqueViolation(err) {
			return ErrGuildAlreadyPremium
		}
		return errors.WithMessage(err, "UpdateAll")
	}

	if n < 1 {
		return ErrSlotNotFound
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

func DetachSlotFromGuild(ctx context.Context, exec boil.ContextExecutor, slotID int64, userID int64) error {

	slot, err := models.PremiumSlots(qm.Where("id = ? AND user_id = ?", slotID, userID), qm.For("UPDATE")).One(ctx, exec)
	if err != nil {
		return errors.WithMessage(err, "PremiumSlots.One")
	}

	if slot == nil {
		return ErrSlotNotFound
	}

	if slot.GuildID.Int64 == 0 {
		return nil
	}

	oldGuildID := slot.GuildID.Int64

	// Update the duration before we reset the guild_id to null
	slot.DurationRemaining = int64(SlotDurationLeft(slot))
	slot.GuildID = null.Int64{}
	slot.AttachedAt = null.Time{}

	_, err = slot.Update(ctx, exec, boil.Infer())
	if err != nil {
		return errors.WithMessage(err, "Update")
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
	slots, err = models.PremiumSlots(qm.Where("user_id = ?", userID), qm.OrderBy("id asc")).AllG(ctx)
	return
}

// UserPremiumMarkedDeletedSlots returns all slots marked deleted for a user for a specific source
func UserPremiumMarkedDeletedSlots(ctx context.Context, tx boil.ContextExecutor, userID int64, limit int, source PremiumSourceType) ([]int64, error) {
	slots, err := models.PremiumSlots(qm.Where("user_id = ? AND deletes_at IS NOT NULL AND source = ?", userID, source), qm.OrderBy("id desc"), qm.Limit(limit)).All(ctx, tx)
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

// RemovePremiumSlots removes the specifues premium slots and attempts to migrate to other permanent available ones
// THIS SHOULD BE USED INSIDE A TRANSACTION ONLY, AS OTHERWISE RACE CONDITIONS BE UPON THEE
func RemovePremiumSlots(ctx context.Context, exec boil.ContextExecutor, userID int64, slotsToRemove []int64) error {
	userSlots, err := models.PremiumSlots(qm.Where("user_id = ?", userID), qm.OrderBy("id desc"), qm.For("UPDATE")).All(ctx, exec)
	if err != nil {
		return errors.WithMessage(err, "models.PremiumSlots")
	}

	// Find the remainign free slots after the removal of the specified slots
	remainingFreeSlots := make([]*models.PremiumSlot, 0)
	toRemove := make(map[int64]struct{}, len(slotsToRemove))
	for _, id := range slotsToRemove {
		toRemove[id] = struct{}{}
	}
	for _, slot := range userSlots {
		if slot.GuildID.Valid || !slot.Permanent || SlotDurationLeft(slot) <= 0 {
			continue
		}
		if _, isRemoving := toRemove[slot.ID]; isRemoving {
			continue
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

		if slot.GuildID.Valid {
			if freeSlotsUsed < len(remainingFreeSlots) {
				// We can migrate it
				remainingFreeSlots[freeSlotsUsed].GuildID = slot.GuildID
				remainingFreeSlots[freeSlotsUsed].AttachedAt = null.TimeFrom(time.Now())
				freeSlotsUsed++
			} else {
				//else we detach the slot from the guild
				err = DetachSlotFromGuild(ctx, exec, slot.ID, slot.UserID)
				if err != nil {
					return errors.WithMessage(err, "DetachSlotFromGuild")
				}
			}
		}

		if slot.Source == string(PremiumSourceTypeCode) {
			_, err = models.PremiumCodes(qm.Where("slot_id = ?", slot.ID)).DeleteAll(ctx, exec)
			if err != nil {
				return errors.WithMessage(err, "PremiumSlots.DeleteAll")
			}
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

func RemoveMarkedDeletedSlotsForUser(ctx context.Context, exec boil.ContextExecutor, userID int64, source PremiumSourceType) error {
	slots, err := models.PremiumSlots(qm.Where("deletes_at IS NOT NULL AND deletes_at < ? AND user_id = ? AND source = ? ", time.Now(), userID, source)).All(ctx, exec)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if len(slots) == 0 {
		return nil
	}
	slotIDs := make([]int64, 0)
	for _, slot := range slots {
		slotIDs = append(slotIDs, slot.ID)
	}
	logger.Infof("Removing %d marked deleted slots for user %d and source %s", len(slots), userID, source)
	return RemovePremiumSlots(ctx, exec, userID, slotIDs)
}
