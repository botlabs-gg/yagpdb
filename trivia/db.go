package trivia

import (
	"context"
	"database/sql"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/trivia/models"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

type TriviaUser = models.TriviaUser

func MarkAnswer(ctx context.Context, guildID, userID int64, correct bool, question *TriviaQuestion) error {
	tx, err := common.PQ.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	add := 2
	remove := 1
	switch question.Difficulty {
	case "easy":
		add = 2
		remove = 1
	case "medium":
		add = 3
		remove = 2
	case "hard":
		add = 4
		remove = 3
	}

	u, err := models.FindTriviaUser(ctx, tx, guildID, userID)
	isNew := false
	if err != nil {
		if err != sql.ErrNoRows {
			return err
		}
		u = &models.TriviaUser{
			GuildID:          guildID,
			UserID:           userID,
			Score:            0,
			CorrectAnswers:   0,
			IncorrectAnswers: 0,
			CurrentStreak:    0,
			MaxStreak:        0,
			LastPlayed:       time.Now(),
		}
		isNew = true
	}
	if correct {
		u.Score += add
		u.CorrectAnswers++
		u.CurrentStreak++
		if u.CurrentStreak > u.MaxStreak {
			u.MaxStreak = u.CurrentStreak
		}
	} else {
		u.Score -= remove
		u.IncorrectAnswers++
		u.CurrentStreak = 0
	}
	u.LastPlayed = time.Now()
	if isNew {
		err = u.Insert(ctx, tx, boil.Infer())
	} else {
		_, err = u.Update(ctx, tx, boil.Infer())
	}

	if err != nil {
		return err
	}

	return tx.Commit()
}

func GetTopTriviaUsers(guildID int64, limit, offset int, sort string) ([]*models.TriviaUser, error) {
	mods := []qm.QueryMod{
		models.TriviaUserWhere.GuildID.EQ(guildID),
		qm.Limit(limit),
		qm.Offset(offset),
	}

	switch sort {
	case "streak":
		mods = append(mods, qm.OrderBy(models.TriviaUserColumns.CurrentStreak+" DESC"))
	case "maxstreak":
		mods = append(mods, qm.OrderBy(models.TriviaUserColumns.MaxStreak+" DESC"))
	case "correct":
		mods = append(mods, qm.OrderBy(models.TriviaUserColumns.CorrectAnswers+" DESC"))
	case "incorrect":
		mods = append(mods, qm.OrderBy(models.TriviaUserColumns.IncorrectAnswers+" DESC"))
	default:
		mods = append(mods, qm.OrderBy(models.TriviaUserColumns.Score+" DESC"))
	}

	return models.TriviaUsers(mods...).AllG(context.Background())
}

func GetTriviaUser(guildID, userID int64) (*models.TriviaUser, int64, error) {
	ctx := context.Background()
	user, err := models.FindTriviaUserG(ctx, guildID, userID)
	if err != nil {
		return nil, 0, err
	}

	count, err := models.TriviaUsers(
		models.TriviaUserWhere.GuildID.EQ(guildID),
		models.TriviaUserWhere.Score.GT(user.Score),
	).CountG(ctx)
	if err != nil {
		return nil, 0, err
	}

	return user, count + 1, nil
}

func GetTriviaGuildStats(guildID int64) (maxScore, currentStreak, maxStreak, maxCorrect, maxIncorrect int, err error) {
	ctx := context.Background()

	topScoreUser, err := models.TriviaUsers(
		models.TriviaUserWhere.GuildID.EQ(guildID),
		qm.OrderBy(models.TriviaUserColumns.Score+" DESC"),
		qm.Select(models.TriviaUserColumns.Score),
	).OneG(ctx)
	if err != nil && err != sql.ErrNoRows {
		return 0, 0, 0, 0, 0, err
	}
	if topScoreUser != nil {
		maxScore = topScoreUser.Score
	}

	topCurrentStreakUser, err := models.TriviaUsers(
		models.TriviaUserWhere.GuildID.EQ(guildID),
		qm.OrderBy(models.TriviaUserColumns.CurrentStreak+" DESC"),
		qm.Select(models.TriviaUserColumns.CurrentStreak),
	).OneG(ctx)
	if err != nil && err != sql.ErrNoRows {
		return 0, 0, 0, 0, 0, err
	}
	if topCurrentStreakUser != nil {
		currentStreak = topCurrentStreakUser.CurrentStreak
	}

	topStreakUser, err := models.TriviaUsers(
		models.TriviaUserWhere.GuildID.EQ(guildID),
		qm.OrderBy(models.TriviaUserColumns.MaxStreak+" DESC"),
		qm.Select(models.TriviaUserColumns.MaxStreak),
	).OneG(ctx)
	if err != nil && err != sql.ErrNoRows {
		return 0, 0, 0, 0, 0, err
	}
	if topStreakUser != nil {
		maxStreak = topStreakUser.MaxStreak
	}

	maxCorrectUser, err := models.TriviaUsers(
		models.TriviaUserWhere.GuildID.EQ(guildID),
		qm.OrderBy(models.TriviaUserColumns.CorrectAnswers+" DESC"),
		qm.Select(models.TriviaUserColumns.CorrectAnswers),
	).OneG(ctx)

	if err != nil && err != sql.ErrNoRows {
		return 0, 0, 0, 0, 0, err
	}
	if maxCorrectUser != nil {
		maxCorrect = maxCorrectUser.CorrectAnswers
	}

	maxIncorrectUser, err := models.TriviaUsers(
		models.TriviaUserWhere.GuildID.EQ(guildID),
		qm.OrderBy(models.TriviaUserColumns.IncorrectAnswers+" DESC"),
		qm.Select(models.TriviaUserColumns.IncorrectAnswers),
	).OneG(ctx)
	if err != nil && err != sql.ErrNoRows {
		return 0, 0, 0, 0, 0, err
	}

	if maxIncorrectUser != nil {
		maxIncorrect = maxIncorrectUser.IncorrectAnswers
	}

	return maxScore, currentStreak, maxStreak, maxCorrect, maxIncorrect, nil
}

func GetTotalTriviaUsers(guildID int64) (int, error) {
	count, err := models.TriviaUsers(models.TriviaUserWhere.GuildID.EQ(guildID)).CountG(context.Background())
	return int(count), err
}

func CleanOldTriviaScores() {
	logger.Info("Cleaning up old trivia scores")
	count, err := models.TriviaUsers(
		models.TriviaUserWhere.LastPlayed.LT(time.Now().Add(-7 * 24 * time.Hour)),
	).DeleteAllG(context.Background())

	if err != nil {
		logger.WithError(err).Error("failed cleaning up old trivia scores")
		return
	}
	logger.Infof("Deleted %d old trivia scores", count)
}

func ResetTriviaLeaderboard(guildID int64) error {
	_, err := models.TriviaUsers(
		models.TriviaUserWhere.GuildID.EQ(guildID),
	).DeleteAllG(context.Background())
	return err
}
