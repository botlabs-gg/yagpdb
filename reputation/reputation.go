package reputation

//go:generate sqlboiler --no-hooks -w "reputation_configs,reputation_users,reputation_log" postgres
//go:generate esc -o assets_gen.go -pkg reputation -ignore ".go" assets/

import (
	"database/sql"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/botrest"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/reputation/models"
	"github.com/mediocregopher/radix.v2/redis"
	"github.com/pkg/errors"
	"strconv"
)

func KeyCooldown(guildID, userID string) string {
	return "reputation_cooldown:" + guildID + ":" + userID
}

type Plugin struct{}

func RegisterPlugin() {
	plugin := &Plugin{}
	common.ValidateSQLSchema(FSMustString(false, "/assets/schema.sql"))
	_, err := common.PQ.Exec(FSMustString(false, "/assets/schema.sql"))
	if err != nil {
		panic(errors.WithMessage(err, "Failed upating reputation schema"))
	}

	common.RegisterPlugin(plugin)
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
		MaxGiveAmount: 1,
	}
}

var (
	ErrUserNotFound = errors.New("User not found")
)

func GetUserStats(guildID, userID string) (score int64, rank int, err error) {

	const query = `SELECT points, position FROM
(
	SELECT user_id, points,
	DENSE_RANK() OVER(ORDER BY points DESC) AS position
	FROM reputation_users WHERE guild_id = $1
) AS w
WHERE user_id = $2`

	row := common.PQ.QueryRow(query, guildID, userID)
	err = row.Scan(&score, &rank)
	if err != nil {
		if err == sql.ErrNoRows {
			err = ErrUserNotFound
		}
	}
	return
}

type RankEntry struct {
	Rank   int   `json:"rank"`
	UserID int64 `json:"user_id"`
	Points int64 `json:"points"`
}

func TopUsers(guildID string, offset, limit int) ([]*RankEntry, error) {
	const query = `SELECT points, position, user_id FROM
(
	SELECT user_id, points,
	DENSE_RANK() OVER(ORDER BY points DESC) AS position
	FROM reputation_users WHERE guild_id = $1
) AS w
ORDER BY points desc
LIMIT $2 OFFSET $3`

	rows, err := common.PQ.Query(query, guildID, limit, offset)
	if err != nil {
		if err == sql.ErrNoRows {
			return []*RankEntry{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	result := make([]*RankEntry, 0, limit)
	for rows.Next() {
		var rank int
		var userID int64
		var points int64
		err = rows.Scan(&points, &rank, &userID)
		if err != nil {
			return nil, err
		}

		result = append(result, &RankEntry{
			Rank:   rank,
			UserID: userID,
			Points: points,
		})
	}

	return result, nil
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

func ModifyRep(conf *models.ReputationConfig, redisClient *redis.Client, guildID string, sender, receiver *discordgo.Member, amount int64) (err error) {
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

	err = insertUpdateUserRep(common.MustParseInt(guildID), common.MustParseInt(receiver.User.ID), amount)
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
		err = errors.WithMessage(err, "ModifyRep log entry.Insert")
	}

	return
}

func insertUpdateUserRep(guildID, userID int64, amount int64) (err error) {

	user := &models.ReputationUser{
		GuildID: guildID,
		UserID:  userID,
		Points:  amount,
	}

	// First try inserting a new user
	err = user.InsertG()
	if err == nil {
		logrus.Debug("Inserted")
		return nil
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
	if conf.AdminRole.String != "" && common.ContainsStringSlice(sender.Roles, conf.AdminRole.String) {
		return nil
	}

	if conf.RequiredGiveRole.String != "" && !common.ContainsStringSlice(sender.Roles, conf.RequiredGiveRole.String) {
		return ErrMissingRequiredGiveRole
	}

	if conf.RequiredReceiveRole.String != "" && !common.ContainsStringSlice(receiver.Roles, conf.RequiredReceiveRole.String) {
		return ErrMissingRequiredReceiveRole
	}

	if conf.BlacklistedGiveRole.String != "" && common.ContainsStringSlice(sender.Roles, conf.BlacklistedGiveRole.String) {
		return ErrBlacklistedGive
	}

	if conf.BlacklistedReceiveRole.String != "" && common.ContainsStringSlice(sender.Roles, conf.BlacklistedReceiveRole.String) {
		return ErrBlacklistedReceive
	}

	return nil
}

func IsAdmin(gs *dstate.GuildState, member *discordgo.Member, config *models.ReputationConfig) bool {
	memberPerms, _ := gs.MemberPermissions(false, gs.ID(), member.User.ID)

	if memberPerms&discordgo.PermissionManageServer != 0 {
		return true
	}

	if config.AdminRole.String != "" {
		if common.ContainsStringSlice(member.Roles, config.AdminRole.String) {
			return true
		}
	}

	return false
}

func SetRep(gid int64, senderID, userID int64, points int64) error {
	user := &models.ReputationUser{
		GuildID: gid,
		UserID:  userID,
		Points:  points,
	}

	err := user.UpsertG(true, []string{"guild_id", "user_id"}, []string{"points"})
	if err != nil {
		return err
	}

	// Insert log entry
	entry := &models.ReputationLog{
		GuildID:        gid,
		SenderID:       senderID,
		ReceiverID:     userID,
		SetFixedAmount: true,
		Amount:         points,
	}

	err = entry.InsertG()
	return errors.WithMessage(err, "SetRep log entry.Insert")
}

// CheckSetCooldown checks and updates the reputation cooldown of a user,
// it returns true if the user was not on cooldown
func CheckSetCooldown(conf *models.ReputationConfig, redisClient *redis.Client, senderID string) (bool, error) {
	if conf.Cooldown < 1 {
		return true, nil
	}

	reply := redisClient.Cmd("SET", KeyCooldown(strconv.FormatInt(conf.GuildID, 10), senderID), true, "EX", conf.Cooldown, "NX")
	if reply.IsType(redis.Nil) {
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
		return nil, errors.Wrap(err, "Reputation.GetConfig")
	}

	return conf, nil
}

type LeaderboardEntry struct {
	*RankEntry
	Username string `json:"username"`
	Bot      bool   `json:"bot"`
	Avatar   string `json:"avatar"`
}

func DetailedLeaderboardEntries(guildID string, ranks []*RankEntry) ([]*LeaderboardEntry, error) {
	var members []*discordgo.Member
	var err error

	compiledIDs := make([]string, len(ranks))
	for i := 0; i < len(ranks); i++ {
		compiledIDs[i] = strconv.FormatInt(ranks[i].UserID, 10)
	}

	if bot.Running {
		members, err = bot.GetMembers(guildID, compiledIDs...)
	} else {
		members, err = botrest.GetMembers(guildID, compiledIDs...)
	}

	if err != nil {
		return nil, err
	}

	var resultEntries = make([]*LeaderboardEntry, len(ranks))
	for i := 0; i < len(ranks); i++ {
		lEntry := &LeaderboardEntry{
			RankEntry: ranks[i],
			Username:  compiledIDs[i],
		}

		for _, m := range members {
			if m.User.ID == compiledIDs[i] {
				lEntry.Username = m.User.Username + "#" + m.User.Discriminator
				lEntry.Avatar = m.User.AvatarURL("256")
				lEntry.Bot = m.User.Bot
				break
			}
		}

		resultEntries[i] = lEntry
	}

	return resultEntries, nil
}

// func DetailedLeaderboardEntries(ranks []*RankEntry) ([]*LeaderboardEntry, error) {
// 	if len(ranks) < 1 {
// 		return []*LeaderboardEntry{}, nil
// 	}

// 	query := "SELECT id,username,bot,avatar FROM d_users WHERE id in ("
// 	args := make([]interface{}, len(ranks))

// 	for i, v := range ranks {
// 		if i != 0 {
// 			query += ","
// 		}

// 		args[i] = v.UserID
// 		query += "$" + strconv.Itoa(i+1)
// 	}
// 	query += ")"

// 	result := make([]*LeaderboardEntry, len(ranks))
// 	for i, v := range ranks {
// 		result[i] = &LeaderboardEntry{
// 			RankEntry: v,
// 		}
// 	}

// 	rows, err := common.DSQLStateDB.Query(query, args...)
// 	if err != nil {
// 		return nil, errors.WithMessage(err, "ToLeaderboardEntries")
// 	}
// 	defer rows.Close()
// 	for rows.Next() {
// 		var id int64
// 		var entry LeaderboardEntry
// 		err = rows.Scan(&id, &entry.Username, &entry.Bot, &entry.Avatar)
// 		if err != nil {
// 			logrus.WithError(err).Error("Failed scanning row")
// 			continue
// 		}

// 		for i, v := range result {
// 			if v.UserID == id {
// 				entry.RankEntry = v.RankEntry
// 				result[i] = &entry
// 				if entry.Avatar != "" {
// 					result[i].Avatar = discordgo.EndpointUserAvatar(strconv.FormatInt(id, 10), entry.Avatar)
// 				} else {
// 					result[i].Avatar = "/static/dist/img/unknown-user.png"
// 				}
// 			}
// 		}
// 	}

// 	return result, nil
// }
