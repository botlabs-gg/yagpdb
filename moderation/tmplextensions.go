package moderation

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/logs"
	"github.com/botlabs-gg/yagpdb/v2/moderation/models"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

func init() {
	templates.RegisterSetupFunc(func(ctx *templates.Context) {
		ctx.ContextFuncs["getWarnings"] = tmplGetWarnings(ctx)
	})
}

// Needed to maintain backward compatibility with previous implementation of
// getWarnings using gorm, in which LogsLink was marked as a string instead of a
// null.String.
type TemplatesWarning struct {
	ID        int
	CreatedAt time.Time
	UpdatedAt time.Time

	GuildID int64
	UserID  int64

	AuthorID              string
	AuthorUsernameDiscrim string

	Message  string
	LogsLink string
}

func templatesWarningFromModel(model *models.ModerationWarning) *TemplatesWarning {
	var logsLink string
	if model.LogsLink.Valid {
		logsLink = model.LogsLink.String
	}

	userID, _ := discordgo.ParseID(model.UserID)
	return &TemplatesWarning{
		ID:        model.ID,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,

		GuildID: model.GuildID,
		UserID:  userID,

		AuthorID:              model.AuthorID,
		AuthorUsernameDiscrim: model.AuthorUsernameDiscrim,

		Message:  model.Message,
		LogsLink: logsLink,
	}
}

// getWarnings returns a slice of all warnings the target user has.
func tmplGetWarnings(ctx *templates.Context) interface{} {
	return func(target interface{}) ([]*TemplatesWarning, error) {
		if ctx.IncreaseCheckCallCounterPremium("cc_moderation", 5, 10) {
			return nil, templates.ErrTooManyCalls
		}

		targetID := templates.TargetUserID(target)
		if targetID == 0 {
			return nil, fmt.Errorf("could not convert %T to a user ID", target)
		}

		warns, err := models.ModerationWarnings(
			models.ModerationWarningWhere.UserID.EQ(discordgo.StrID(targetID)),
			models.ModerationWarningWhere.GuildID.EQ(ctx.GS.ID),

			qm.OrderBy("id DESC"),
		).AllG(context.Background())
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}

		out := make([]*TemplatesWarning, len(warns))
		for i, w := range warns {
			out[i] = templatesWarningFromModel(w)
		}
		return stripExpiredLogLinks(out), nil
	}
}

// stripExpiredLogLinks clears the LogLink field for warnings whose logs have
// expired in-place and returns the input slice.
func stripExpiredLogLinks(warns []*TemplatesWarning) []*TemplatesWarning {
	if !logs.ConfEnableMessageLogPurge.GetBool() {
		return warns
	}

	const logStorageDuration = 30 * 24 * time.Hour // TODO: does this constant already exist elsewhere?

	for _, entry := range warns {
		if entry.LogsLink != "" && time.Since(entry.CreatedAt) > logStorageDuration {
			entry.LogsLink = ""
		}
	}
	return warns
}
