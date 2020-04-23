package reputation

//go:generate sqlboiler --no-hooks psql

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"emperror.dev/errors"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/botrest"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/featureflags"
	"github.com/jonas747/yagpdb/reputation/models"
	"github.com/mediocregopher/radix/v3"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

var logger = common.GetPluginLogger(&Plugin{})

func RegisterPlugin() {

	plugin := &Plugin{}

	common.InitSchemas("reputation", DBSchemas...)

	common.RegisterPlugin(plugin)
}

type Plugin struct{}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Reputation",
		SysName:  "reputation",
		Category: common.PluginCategoryMisc,
	}
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

func KeyCooldown(guildID, userID int64) string {
	return "reputation_cooldown:" + discordgo.StrID(guildID) + ":" + discordgo.StrID(userID)
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
	ErrMissingRequiredGiveRole    = UserError("You don't have any of the required roles to give points")
	ErrMissingRequiredReceiveRole = UserError("Target don't have any of the required roles to receive points")

	ErrBlacklistedGive    = UserError("Blacklisted from giving points")
	ErrBlacklistedReceive = UserError("Blacklisted from receiving points")
	ErrCooldown           = UserError("You're still on cooldown")
)

func ModifyRep(ctx context.Context, conf *models.ReputationConfig, guildID int64, sender, receiver *dstate.MemberState, amount int64) (err error) {
	if conf == nil {
		conf, err = GetConfig(ctx, guildID)
		if err != nil {
			return
		}
	}

	if err = CanModifyRep(conf, sender, receiver); err != nil {
		return
	}

	if amount > 0 && amount > conf.MaxGiveAmount {
		err = UserError(fmt.Sprintf("Can't give that much (max %d)", conf.MaxGiveAmount))
		return
	} else if amount < 0 && -amount > conf.MaxRemoveAmount {
		err = UserError(fmt.Sprintf("Can't remove that much (max %d)", conf.MaxRemoveAmount))
		return
	} else if amount == 0 {
		return nil
	}

	ok, err := CheckSetCooldown(conf, sender.ID)
	if err != nil || !ok {
		if err == nil {
			err = ErrCooldown
		}
		return
	}

	err = insertUpdateUserRep(ctx, guildID, receiver.ID, amount)
	if err != nil {
		// Clear the cooldown since it failed updating the rep
		ClearCooldown(guildID, sender.ID)
		return
	}

	receiver.Guild.RLock()
	defer receiver.Guild.RUnlock()
	receiverUsername := receiver.Username + "#" + receiver.StrDiscriminator()
	senderUsername := sender.Username + "#" + sender.StrDiscriminator()

	// Add the log element
	entry := &models.ReputationLog{
		GuildID:          guildID,
		SenderID:         sender.ID,
		SenderUsername:   senderUsername,
		ReceiverID:       receiver.ID,
		ReceiverUsername: receiverUsername,
		SetFixedAmount:   false,
		Amount:           amount,
	}

	err = entry.InsertG(ctx, boil.Infer())
	if err != nil {
		err = errors.WithMessage(err, "ModifyRep log entry.Insert")
	}

	return
}

func insertUpdateUserRep(ctx context.Context, guildID, userID int64, amount int64) (err error) {

	// upsert query which is too advanced for orms
	const query = `
INSERT INTO reputation_users (created_at, guild_id, user_id, points)
VALUES ($1, $2, $3, $4)
ON CONFLICT (guild_id, user_id)
DO UPDATE SET points = reputation_users.points + $4;
`
	_, err = common.PQ.ExecContext(ctx, query, time.Now(), guildID, userID, amount)
	return
}

// Returns a user error if the sender can not modify the rep of receiver
// Admins are always able to modify the rep of everyone
func CanModifyRep(conf *models.ReputationConfig, sender, receiver *dstate.MemberState) error {
	if common.ContainsInt64SliceOneOf(sender.Roles, conf.AdminRoles) {
		return nil
	}

	if len(conf.RequiredGiveRoles) > 0 && !common.ContainsInt64SliceOneOf(sender.Roles, conf.RequiredGiveRoles) {
		return ErrMissingRequiredGiveRole
	}

	if len(conf.RequiredReceiveRoles) > 0 && !common.ContainsInt64SliceOneOf(receiver.Roles, conf.RequiredReceiveRoles) {
		return ErrMissingRequiredReceiveRole
	}

	if common.ContainsInt64SliceOneOf(sender.Roles, conf.BlacklistedGiveRoles) {
		return ErrBlacklistedGive
	}

	if common.ContainsInt64SliceOneOf(receiver.Roles, conf.BlacklistedReceiveRoles) {
		return ErrBlacklistedReceive
	}

	return nil
}

func IsAdmin(gs *dstate.GuildState, member *dstate.MemberState, config *models.ReputationConfig) bool {

	memberPerms, _ := gs.MemberPermissions(false, gs.ID, member.ID)

	if memberPerms&discordgo.PermissionManageServer != 0 {
		return true
	}

	if common.ContainsInt64SliceOneOf(member.Roles, config.AdminRoles) {
		return true
	}

	return false
}

func SetRep(ctx context.Context, gid int64, senderID, userID int64, points int64) error {
	user := &models.ReputationUser{
		GuildID: gid,
		UserID:  userID,
		Points:  points,
	}

	err := user.UpsertG(ctx, true, []string{"guild_id", "user_id"}, boil.Whitelist("points"), boil.Infer())
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

	err = entry.InsertG(ctx, boil.Infer())
	return errors.WithMessage(err, "SetRep log entry.Insert")
}

func DelRep(ctx context.Context, gid int64, userID int64) error {
	_, err := models.ReputationUsers(qm.Where("guild_id = ? AND user_id = ?", gid, userID)).DeleteAll(ctx, common.PQ)
	return err
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

func GetConfig(ctx context.Context, guildID int64) (*models.ReputationConfig, error) {
	conf, err := models.FindReputationConfigG(ctx, guildID)
	if err != nil {
		if err == sql.ErrNoRows {
			return DefaultConfig(guildID), nil
		}
		return nil, errors.WrapIf(err, "Reputation.GetConfig")
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
	if len(ranks) < 1 {
		return []*LeaderboardEntry{}, nil
	}

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

var _ featureflags.PluginWithFeatureFlags = (*Plugin)(nil)

const (
	featureFlagReputationEnabled = "reputation_enabled"
	featureFlagThanksEnabled     = "reputation_thanks_enabled"
)

func (p *Plugin) UpdateFeatureFlags(guildID int64) ([]string, error) {
	config, err := GetConfig(context.Background(), guildID)
	if err != nil {
		return nil, errors.WithStackIf(err)
	}

	var flags []string
	if config.Enabled {
		flags = append(flags, featureFlagReputationEnabled)

		if !config.DisableThanksDetection {
			flags = append(flags, featureFlagThanksEnabled)
		}
	}

	return flags, nil
}

func (p *Plugin) AllFeatureFlags() []string {
	return []string{
		featureFlagReputationEnabled, // set if reputation is enabled on this server
		featureFlagThanksEnabled,     // set if reputation thanks detection is anabled
	}
}
