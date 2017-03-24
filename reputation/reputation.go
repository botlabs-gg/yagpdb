package reputation

//go:generate sqlboiler -w "reputation_configs,reputation_users,reputation_log" postgres

import (
	"database/sql"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/reputation/models"
	"github.com/jonas747/yagpdb/web"
	"github.com/pkg/errors"
	"strconv"
)

func KeyCooldown(guildID, userID string) string {
	return "reputation_cooldown:" + guildID + ":" + userID
}

type Plugin struct{}

func RegisterPlugin() {
	plugin := &Plugin{}
	bot.RegisterPlugin(plugin)
	web.RegisterPlugin(plugin)
}

func (p *Plugin) Name() string {
	return "Reputation"
}

func DefaultConfig(guildID string) *models.ReputationConfig {
	return &models.ReputationConfig{
		GuildID:       common.MustParseInt(guildID),
		PointsName:    "Rep",
		Enabled:       false,
		Cooldown:      120,
		MaxGiveAmount: 10,
	}
}

var (
	ErrUserNotFound = errors.New("User not found")
)

func GetUserStats(client *redis.Client, guildID, userID string) (score int64, rank int, err error) {

	user, err := models.FindReputationUserG(common.MustParseInt(userID), common.MustParseInt(guildID))
	if err != nil {
		if err == sql.ErrNoRows {
			err = ErrUserNotFound
		}
		return
	}

	score = user.Points
	return
}

type UserError string

func (b UserError) Error() string {
	return string(b)
}

var (
	ErrMissingRequiredGiveRole    = UserError("Missing the required role to give points")
	ErrMissingRequiredReceiveRole = UserError("Missing the required role to receive points")

	ErrBlacklistedGive    = UserError("Blaclisted from giving points")
	ErrBlacklistedReceive = UserError("Blacklisted from receiving points")
	ErrCooldown           = UserError("You're still on cooldown")
)

func ModifyRep(conf *models.ReputationConfig, redisClient *redis.Client, guildID string, sender, receiver *discordgo.Member, amount int64) (newAmount int64, err error) {
	if conf == nil {
		conf, err = GetConfig(guildID)
		if err != nil {
			return
		}
	}

	if err = CanModifyRep(conf, sender, receiver); err != nil {
		return
	}

	if amount > conf.MaxGiveAmount {
		err = UserError(fmt.Sprintf("Too big amount (max %d)", conf.MaxGiveAmount))
		return
	} else if amount < -conf.MaxGiveAmount {
		err = UserError(fmt.Sprintf("Too small amount (min -%d)", conf.MaxGiveAmount))
		return
	}

	ok, err := CheckSetCooldown(conf, redisClient, sender.User.ID)
	if err != nil || !ok {
		if err == nil {
			err = ErrCooldown
		}
		return
	}

	newAmount, err = insertUpdateUser(common.MustParseInt(guildID), common.MustParseInt(receiver.User.ID), amount)
	if err != nil {
		// Clear the cooldown since it failed updating the rep
		ClearCooldown(redisClient, guildID, sender.User.ID)
		return
	}

	// Add the log element
	entry := &models.ReputationLog{
		GuildID:        common.MustParseInt(guildID),
		SenderID:       common.MustParseInt(sender.User.ID),
		ReceiverID:     common.MustParseInt(sender.User.ID),
		SetFixedAmount: false,
		Amount:         amount,
	}
	err = entry.InsertG()
	if err != nil {
		err = errors.WithMessage(err, "ModifyRep log entry.Isert")
	}

	return
}

func insertUpdateUser(guildID, userID int64, amount int64) (newAmount int64, err error) {

	user := &models.ReputationUser{
		GuildID: guildID,
		UserID:  userID,
		Points:  amount,
	}

	// First try inserting a new user
	err = user.InsertG()
	if err == nil {
		logrus.Debug("Inserted")
		return amount, nil
	}

	// Update
	r, err := common.PQ.Exec("UPDATE reputation_users SET points = points + $1 WHERE user_id = $2 AND guild_id = $3", amount, userID, guildID)
	rows, _ := r.RowsAffected()
	logrus.Println("Rows: ", rows, userID, guildID)
	return
}

// Returns a user error if the sender can not modify the rep of receiver
// Admins are always able to modify the rep of everyone
func CanModifyRep(conf *models.ReputationConfig, sender, receiver *discordgo.Member) error {
	if conf.AdminRole.String != "" && common.FindStringSlice(sender.Roles, conf.AdminRole.String) {
		return nil
	}

	if conf.RequiredGiveRole.String != "" && !common.FindStringSlice(sender.Roles, conf.RequiredGiveRole.String) {
		return ErrMissingRequiredGiveRole
	}

	if conf.RequiredReceiveRole.String != "" && !common.FindStringSlice(receiver.Roles, conf.RequiredReceiveRole.String) {
		return ErrMissingRequiredReceiveRole
	}

	if conf.BlacklistedGiveRole.String != "" && common.FindStringSlice(sender.Roles, conf.BlacklistedGiveRole.String) {
		return ErrBlacklistedGive
	}

	if conf.BlacklistedReceiveRole.String != "" && common.FindStringSlice(sender.Roles, conf.BlacklistedReceiveRole.String) {
		return ErrBlacklistedReceive
	}

	return nil
}

func CheckSetCooldown(conf *models.ReputationConfig, redisClient *redis.Client, senderID string) (bool, error) {
	if conf.Cooldown < 1 {
		return true, nil
	}

	reply := redisClient.Cmd("SET", KeyCooldown(strconv.FormatInt(conf.GuildID, 10), senderID), true, "EX", conf.Cooldown, "NX")
	if reply.Type == redis.NilReply {
		return false, nil
	}
	if reply.Err != nil {
		return false, common.ErrWithCaller(reply.Err)
	}

	return true, nil
}

func ClearCooldown(redisClient *redis.Client, guildID, senderID string) error {
	return redisClient.Cmd("DEL", KeyCooldown(guildID, senderID)).Err
}

func GetConfig(guildID string) (*models.ReputationConfig, error) {
	conf, err := models.FindReputationConfigG(common.MustParseInt(guildID))
	if err != nil {
		if err == sql.ErrNoRows {
			return DefaultConfig(guildID), nil
		}
		return nil, errors.Wrap(err, "GetConfig")
	}

	return conf, nil
}
