package reputation

//go:generate sqlboiler --no-hooks -w "reputation_configs,reputation_users,reputation_log" postgres

import (
	"database/sql"
	"fmt"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/botrest"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/reputation/models"
	"github.com/mediocregopher/radix.v3"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"strconv"
)

func KeyCooldown(guildID, userID int64) string {
	return "reputation_cooldown:" + discordgo.StrID(guildID) + ":" + discordgo.StrID(userID)
}

type Plugin struct{}

func RegisterPlugin() {
	plugin := &Plugin{}
	common.ValidateSQLSchema(DBSchema)
	_, err := common.PQ.Exec(DBSchema)
	if err != nil {
		panic(errors.WithMessage(err, "Failed upating reputation schema"))
	}

	common.RegisterPlugin(plugin)
}

func (p *Plugin) Name() string {
	return "Reputation"
}

func DefaultConfig(guildID int64) *models.ReputationConfig {
	return &models.ReputationConfig{
		GuildID:       guildID,
		PointsName:    "Rep",
		Enabled:       false,
		Cooldown:      120,
		MaxGiveAmount: 1,
	}
}

var (
	ErrUserNotFound = errors.New("User not found")
)

func GetUserStats(guildID, userID int64) (score int64, rank int, err error) {

	const query = `SELECT points, position FROM
(
	SELECT user_id, points,
	RANK() OVER(ORDER BY points DESC) AS position
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

func TopUsers(guildID int64, offset, limit int) ([]*RankEntry, error) {
	const query = `SELECT points, position, user_id FROM
(
	SELECT user_id, points,
	RANK() OVER(ORDER BY points DESC) AS position
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

func ModifyRep(conf *models.ReputationConfig, guildID int64, sender, receiver *dstate.MemberState, amount int64) (err error) {
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

	ok, err := CheckSetCooldown(conf, sender.ID)
	if err != nil || !ok {
		if err == nil {
			err = ErrCooldown
		}
		return
	}

	err = insertUpdateUserRep(guildID, receiver.ID, amount)
	if err != nil {
		// Clear the cooldown since it failed updating the rep
		ClearCooldown(guildID, sender.ID)
		return
	}

	// Add the log element
	entry := &models.ReputationLog{
		GuildID:        guildID,
		SenderID:       sender.ID,
		ReceiverID:     receiver.ID,
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
func CanModifyRep(conf *models.ReputationConfig, sender, receiver *dstate.MemberState) error {
	parsedAdminRole, _ := strconv.ParseInt(conf.AdminRole.String, 10, 64)
	if conf.AdminRole.String != "" && common.ContainsInt64Slice(sender.Roles, parsedAdminRole) {
		return nil
	}

	parsedRequiredGiveRole, _ := strconv.ParseInt(conf.RequiredGiveRole.String, 10, 64)
	if conf.RequiredGiveRole.String != "" && !common.ContainsInt64Slice(sender.Roles, parsedRequiredGiveRole) {
		return ErrMissingRequiredGiveRole
	}

	parsedRequiredReceiveRole, _ := strconv.ParseInt(conf.RequiredReceiveRole.String, 10, 64)
	if conf.RequiredReceiveRole.String != "" && !common.ContainsInt64Slice(receiver.Roles, parsedRequiredReceiveRole) {
		return ErrMissingRequiredReceiveRole
	}

	parsedBlacklistedGiveRole, _ := strconv.ParseInt(conf.BlacklistedGiveRole.String, 10, 64)
	if conf.BlacklistedGiveRole.String != "" && common.ContainsInt64Slice(sender.Roles, parsedBlacklistedGiveRole) {
		return ErrBlacklistedGive
	}

	parsedBlacklistedReceiveRole, _ := strconv.ParseInt(conf.BlacklistedReceiveRole.String, 10, 64)
	if conf.BlacklistedReceiveRole.String != "" && common.ContainsInt64Slice(sender.Roles, parsedBlacklistedReceiveRole) {
		return ErrBlacklistedReceive
	}

	return nil
}

func IsAdmin(gs *dstate.GuildState, member *dstate.MemberState, config *models.ReputationConfig) bool {

	memberPerms, _ := gs.MemberPermissions(false, gs.ID, member.ID)

	if memberPerms&discordgo.PermissionManageServer != 0 {
		return true
	}

	if config.AdminRole.String != "" {
		parsedAdminRole, _ := strconv.ParseInt(config.AdminRole.String, 10, 64)
		if common.ContainsInt64Slice(member.Roles, parsedAdminRole) {
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
func CheckSetCooldown(conf *models.ReputationConfig, senderID int64) (bool, error) {
	if conf.Cooldown < 1 {
		return true, nil
	}

	var resp string
	err := common.RedisPool.Do(radix.FlatCmd(&resp, "SET", KeyCooldown(conf.GuildID, senderID), true, "EX", conf.Cooldown, "NX"))
	if resp != "OK" {
		return false, err
	}

	return true, err
}

func ClearCooldown(guildID, senderID int64) error {
	return common.RedisPool.Do(radix.Cmd(nil, "DEL", KeyCooldown(guildID, senderID)))
}

func GetConfig(guildID int64) (*models.ReputationConfig, error) {
	conf, err := models.FindReputationConfigG(guildID)
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

func DetailedLeaderboardEntries(guildID int64, ranks []*RankEntry) ([]*LeaderboardEntry, error) {
	var members []*discordgo.Member
	var err error

	userIDs := make([]int64, len(ranks))
	for i := 0; i < len(ranks); i++ {
		userIDs[i] = ranks[i].UserID
	}

	if bot.Running {
		var tmp []*dstate.MemberState
		tmp, err = bot.GetMembers(guildID, userIDs...)
		if tmp != nil {
			for _, v := range tmp {
				members = append(members, v.DGoCopy())
			}
		}
	} else {
		members, err = botrest.GetMembers(guildID, userIDs...)
	}

	if err != nil {
		return nil, err
	}

	var resultEntries = make([]*LeaderboardEntry, len(ranks))
	for i := 0; i < len(ranks); i++ {
		lEntry := &LeaderboardEntry{
			RankEntry: ranks[i],
			Username:  strconv.FormatInt(userIDs[i], 10),
		}

		for _, m := range members {
			if m.User.ID == userIDs[i] {
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
