package codepremiumsource

//go:generate sqlboiler psql

import (
	"context"
	"database/sql"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/premium"
	"github.com/jonas747/yagpdb/premium/codepremiumsource/models"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"time"
)

var (
	ErrCodeExpired  = errors.New("Code expired")
	ErrCodeNotFound = errors.New("Code not found")
)

func init() {
	premium.RegisterPremiumSource(&PremiumSource{})
}

type PremiumSource struct{}

func (ps *PremiumSource) Init() {
	_, err := common.PQ.Exec(DBSchema)
	if err != nil {
		logrus.WithError(err).Error("Failed initilizing premium code source")
	}
}

func (ps *PremiumSource) Names() (human string, idname string) {
	return "Redeemed code", "code"
}

func (ps *PremiumSource) AllUserSlots(ctx context.Context) (userSlots map[int64][]*premium.PremiumSlot, err error) {
	allSlots, err := models.PremiumCodes(qm.Where("user_id IS NOT NULL")).AllG(ctx)
	if err != nil {
		return nil, errors.WithMessage(err, "codepremiumsource.AllUserSlots")
	}

	dst := make(map[int64][]*premium.PremiumSlot)
	for _, slot := range allSlots {
		dst[slot.UserID.Int64] = append(dst[slot.UserID.Int64], &premium.PremiumSlot{
			ID:           slot.ID,
			Source:       "code",
			Message:      slot.Message,
			UserID:       slot.UserID.Int64,
			GuildID:      slot.GuildID.Int64,
			DurationLeft: CodeDurationLeft(slot),
			Temporary:    !slot.Permanent,
		})
	}

	return dst, nil
}

func (ps *PremiumSource) SlotsForUser(ctx context.Context, userID int64) (slots []*premium.PremiumSlot, err error) {
	codes, err := models.PremiumCodes(qm.Where("user_id = ?", userID)).AllG(ctx)
	if err != nil {
		err = errors.WithMessage(err, "codepremiumsource.SlotsForUser")
	}

	slots = make([]*premium.PremiumSlot, 0, len(slots))
	for _, code := range codes {
		slots = append(slots, &premium.PremiumSlot{
			ID:           code.ID,
			Source:       "code",
			Message:      code.Message,
			UserID:       code.UserID.Int64,
			GuildID:      code.GuildID.Int64,
			DurationLeft: CodeDurationLeft(code),
			Temporary:    !code.Permanent,
		})
	}

	return slots, err
}

func (ps *PremiumSource) AttachSlot(ctx context.Context, userID int64, slotID int64, guildID int64) error {
	tx, err := common.PQ.Begin()
	if err != nil {
		return errors.WithMessage(err, "begin.codepremiumsource.AttachSlot")
	}

	code, err := models.PremiumCodes(qm.Where("id = ? AND user_id = ? AND guild_id IS NULL", slotID, userID), qm.For("UPDATE")).One(ctx, tx)
	if err != nil {
		tx.Rollback()
		return errors.WithMessage(err, "find.codepremiumsource.AttachSlot")
	}

	if !code.Permanent {
		durLeft := CodeDurationLeft(code)
		if durLeft <= 0 {
			tx.Rollback()
			return ErrCodeExpired
		}
	}

	code.AttachedAt = null.TimeFrom(time.Now())
	code.GuildID = null.Int64From(guildID)

	_, err = code.Update(ctx, tx, boil.Infer())
	if err != nil {
		tx.Rollback()
		return errors.WithMessage(err, "update.codepremiumsource.AttachSlot")
	}

	err = tx.Commit()
	return errors.WithMessage(err, "commit.codepremiumsource.AttachSlot")
}

func (ps *PremiumSource) DetachSlot(ctx context.Context, userID int64, slotID int64) error {
	tx, err := common.PQ.Begin()
	if err != nil {
		return errors.WithMessage(err, "begin.codepremiumsource.DetachSlot")
	}

	code, err := models.PremiumCodes(qm.Where("id = ? AND user_id = ? AND guild_id IS NOT NULL", slotID, userID), qm.For("UPDATE")).One(ctx, tx)
	if err != nil {
		tx.Rollback()
		return errors.WithMessage(err, "find.codepremiumsource.DetachSlot")
	}

	if !code.Permanent {
		// Update duration left
		durUsedSinceLastAttach := time.Since(code.AttachedAt.Time)
		code.DurationUsed += int64(durUsedSinceLastAttach)
	}

	code.AttachedAt = null.Time{}
	code.GuildID = null.Int64{}

	_, err = code.Update(ctx, tx, boil.Infer())
	if err != nil {
		tx.Rollback()
		return errors.WithMessage(err, "update.codepremiumsource.DetachSlot")
	}

	err = tx.Commit()
	return errors.WithMessage(err, "commit.codepremiumsource.DetachSlot")
}

func CodeDurationLeft(code *models.PremiumCode) (duration time.Duration) {
	if code.Permanent {
		return 0xfffffffffffffff
	}

	duration = time.Duration(code.FullDuration - code.DurationUsed)

	if code.GuildID.Valid {
		duration -= time.Since(code.AttachedAt.Time)
	}

	return duration
}

func RedeemCode(ctx context.Context, code string, userID int64) error {
	tx, err := common.PQ.Begin()
	if err != nil {
		return errors.WithMessage(err, "begin.codepremiumsource.RedeemCode")
	}

	c, err := models.PremiumCodes(qm.Where("code = ? AND user_id IS NULL", code), qm.For("UPDATE")).One(ctx, tx)
	if err != nil {
		tx.Rollback()
		return errors.WithMessage(err, "find.codepremiumsource.RedeemCode")
	}

	c.UserID = null.Int64From(userID)
	c.UsedAt = null.TimeFrom(time.Now())

	_, err = c.Update(ctx, tx, boil.Infer())
	if err != nil {
		tx.Rollback()
		return errors.WithMessage(err, "update.codepremiumsource.RedeemCode")
	}

	err = tx.Commit()
	return errors.WithMessage(err, "commit.codepremiumsource.RedeemCode")
}

func LookupCode(ctx context.Context, code string) (*models.PremiumCode, error) {
	c, err := models.PremiumCodes(qm.Where("code = ", code)).OneG(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrCodeNotFound
		}

		return nil, errors.WithMessage(err, "LookupCode")
	}

	return c, nil
}
