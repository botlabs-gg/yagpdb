package premium

//go:generate sqlboiler psql

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"fmt"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/premium/models"
	"github.com/jonas747/yagpdb/stdcommands/util"
	"github.com/lib/pq"
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
	RegisterPremiumSource(&CodePremiumSource{})
}

type CodePremiumSource struct{}

func (ps *CodePremiumSource) Init() {
	_, err := common.PQ.Exec(DBSchema)
	if err != nil {
		logrus.WithError(err).Error("Failed initilizing premium code source")
	}
}

func (ps *CodePremiumSource) Names() (human string, idname string) {
	return "Redeemed code", "code"
}

func (ps *CodePremiumSource) AllUserSlots(ctx context.Context) (userSlots map[int64][]*PremiumSlot, err error) {
	allSlots, err := models.PremiumCodes(qm.Where("user_id IS NOT NULL")).AllG(ctx)
	if err != nil {
		return nil, errors.WithMessage(err, "codepremiumsource.AllUserSlots")
	}

	dst := make(map[int64][]*PremiumSlot)
	for _, slot := range allSlots {
		dst[slot.UserID.Int64] = append(dst[slot.UserID.Int64], &PremiumSlot{
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

func (ps *CodePremiumSource) SlotsForUser(ctx context.Context, userID int64) (slots []*PremiumSlot, err error) {
	codes, err := models.PremiumCodes(qm.Where("user_id = ?", userID)).AllG(ctx)
	if err != nil {
		err = errors.WithMessage(err, "codepremiumsource.SlotsForUser")
	}

	slots = make([]*PremiumSlot, 0, len(slots))
	for _, code := range codes {
		slots = append(slots, &PremiumSlot{
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

func (ps *CodePremiumSource) AttachSlot(ctx context.Context, userID int64, slotID int64, guildID int64) error {
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

func (ps *CodePremiumSource) DetachSlot(ctx context.Context, userID int64, slotID int64) error {
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
	c, err := models.PremiumCodes(qm.Where("code = ?", code)).OneG(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrCodeNotFound
		}

		return nil, errors.WithMessage(err, "LookupCode")
	}

	return c, nil
}

var (
	ErrCodeCollision = errors.New("Code collision")
)

// TryRetryGenerateCode attempts to generate codes, if it enocunters a key collision it retries, returns on all other cases
func TryRetryGenerateCode(ctx context.Context, message string, duration time.Duration) (*models.PremiumCode, error) {
	for {
		code, err := GenerateCode(ctx, message, duration)
		if err != nil && err == ErrCodeCollision {
			logrus.WithError(err).Error("Code collision!")
			continue
		}

		return code, err
	}
}

// GenerateCode generates a redeemable premium code with the specified duration (-1 for permanent) and message
func GenerateCode(ctx context.Context, message string, duration time.Duration) (*models.PremiumCode, error) {
	key := make([]byte, 16)
	_, err := rand.Read(key)
	if err != nil {
		return nil, errors.WithMessage(err, "GenerateCode")
	}

	encoded := encodeKey(key)

	model := &models.PremiumCode{
		Code:         encoded,
		Message:      message,
		Permanent:    duration == -1,
		FullDuration: int64(duration),
	}

	err = model.InsertG(ctx, boil.Infer())
	if err != nil {
		if cast, ok := errors.Cause(err).(*pq.Error); ok {
			if cast.Code == "23505" {
				return nil, ErrCodeCollision
			}
		}
	}
	return model, err
}

var keyEncoder = base32.StdEncoding.WithPadding(base32.NoPadding)

func encodeKey(rawKey []byte) string {
	str := keyEncoder.EncodeToString(rawKey)
	output := ""
	for i, r := range str {
		if i%6 == 0 && i != 0 {
			output += "-"
		}
		output += string(r)
	}

	return output
}

var cmdGenerateCode = &commands.YAGCommand{
	CmdCategory:          commands.CategoryDebug,
	HideFromCommandsPage: true,
	Name:                 "generatepremiumcode",
	Aliases:              []string{"gpc"},
	Description:          "Generates premium codes",
	HideFromHelp:         true,
	RequiredArgs:         3,
	RunInDM:              true,
	Arguments: []*dcmd.ArgDef{
		{Name: "Duration", Type: &commands.DurationArg{}},
		{Name: "NumCodes", Type: dcmd.Int},
		{Name: "Message", Type: dcmd.String},
	},
	RunFunc: util.RequireOwner(func(data *dcmd.Data) (interface{}, error) {
		numKeys := data.Args[1].Int()
		duration := data.Args[0].Value.(time.Duration)
		codes := make([]string, 0, numKeys)

		if duration <= 0 {
			duration = -1
		}

		for i := 0; i < numKeys; i++ {
			code, err := TryRetryGenerateCode(data.Context(), data.Args[2].Str(), duration)
			if err != nil {
				return nil, err
			}

			codes = append(codes, code.Code)
		}

		dm := fmt.Sprintf("Duration: `%s`, Permanent: `%t`, Message: `%s`\n```\n", duration.String(), duration == -1, data.Args[2].Str())

		for _, v := range codes {
			dm += v + "\n"
		}

		dm += "```"

		bot.SendDM(data.Msg.Author.ID, dm)
		return "Check yer dms", nil
	}),
}
