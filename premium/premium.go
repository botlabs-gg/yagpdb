package premium

import (
	"context"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/premium/models"
	"github.com/mediocregopher/radix.v3"
	"github.com/pkg/errors"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"strconv"
	"time"
)

const (
	// Hash
	// Key: guild id's
	// Value: the user id's providing the premium status
	RedisKeyPremiumGuilds    = "premium_activated_guilds"
	RedisKeyPremiumGuildsTmp = "premium_activated_guilds_tmp"
)

type Plugin struct {
}

func (p *Plugin) Name() string {
	return "premium"
}

func RegisterPlugin() {
	common.RegisterPlugin(&Plugin{})

	for _, v := range PremiumSources {
		v.Init()
	}
}

// IsGuildPremium return true if the provided guild has the premium status provided to it by a user
func IsGuildPremium(guildID int64) (bool, error) {
	var premium bool
	err := common.RedisPool.Do(radix.FlatCmd(&premium, "HEXISTS", RedisKeyPremiumGuilds, guildID))
	return premium, errors.WithMessage(err, "IsGuildPremium")
}

// UserPremiumSlots returns all slots for a user
func UserPremiumSlots(ctx context.Context, userID int64) (slots []*models.PremiumSlot, err error) {
	slots, err = models.PremiumSlots(qm.Where("user_id = ?", userID), qm.OrderBy("id desc")).AllG(ctx)
	return
}

var (
	PremiumSources []PremiumSource

	ErrSlotNotFound = errors.New("premium slot not found")
)

type PremiumSource interface {
	Init()
	Names() (human string, idname string)
}

func RegisterPremiumSource(source PremiumSource) {
	PremiumSources = append(PremiumSources, source)
}

func SlotExpired(slot *models.PremiumSlot) {
	// TODO
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
	n, err := models.PremiumSlots(qm.Where("id = ? AND user_id = ? AND guild_id IS NULL AND (permanent OR duration_remaining > 0)", slotID, userID)).UpdateAll(
		ctx, common.PQ, models.M{"guild_id": null.Int64From(guildID), "attached_at": time.Now()})
	if err != nil {
		return errors.WithMessage(err, "UpdateAll")
	}

	if n < 1 {
		return ErrSlotNotFound
	}

	common.RedisPool.Do(radix.FlatCmd(nil, "HSET", strconv.FormatInt(guildID, 10), userID))

	return nil
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
	return errors.WithMessage(err, "Commit")
}
