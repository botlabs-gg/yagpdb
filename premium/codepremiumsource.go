package premium

//go:generate sqlboiler psql

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"fmt"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/premium/models"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/util"
	"github.com/lib/pq"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
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
}

func (ps *CodePremiumSource) Names() (human string, idname string) {
	return "Redeemed code", "code"
}

func RedeemCode(ctx context.Context, code string, userID int64) error {
	tx, err := common.PQ.BeginTx(ctx, nil)
	if err != nil {
		return errors.WithMessage(err, "BeginTX")
	}

	// Query for the code model
	c, err := models.PremiumCodes(qm.Where("code = ? AND user_id IS NULL", code), qm.For("UPDATE")).One(ctx, tx)
	if err != nil {
		tx.Rollback()
		return errors.WithMessage(err, "models.PremiumCodes")
	}

	// model found, with no user attached, create the slot for it
	slot, err := CreatePremiumSlot(ctx, tx, userID, "code", "Redeemed code", c.Message, c.ID, time.Duration(c.Duration), PremiumTierPremium)
	if err != nil {
		tx.Rollback()
		return errors.WithMessage(err, "CreatePremiumSlot")
	}

	// Update the code fields
	c.UserID = null.Int64From(userID)
	c.UsedAt = null.TimeFrom(time.Now())
	c.SlotID = null.Int64From(slot.ID)

	_, err = c.Update(ctx, tx, boil.Infer())
	if err != nil {
		tx.Rollback()
		return errors.WithMessage(err, "Update")
	}

	err = tx.Commit()
	return errors.WithMessage(err, "Commit")
}

func LookupCode(ctx context.Context, code string) (*models.PremiumCode, error) {
	c, err := models.PremiumCodes(qm.Where("code = ?", code)).OneG(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrCodeNotFound
		}

		return nil, errors.WithMessage(err, "models.PremiumCodes")
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
			logger.WithError(err).Error("Code collision!")
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
		Code:      encoded,
		Message:   message,
		Permanent: duration == -1,
		Duration:  int64(duration),
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

		bot.SendDM(data.Author.ID, dm)
		return "Check yer dms", nil
	}),
}
