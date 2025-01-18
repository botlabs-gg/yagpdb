package reputation

//go:generate sqlboiler --no-hooks psql

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strconv"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/botrest"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/featureflags"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/reputation/models"
	"github.com/mediocregopher/radix/v3"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
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
		GuildID:         guildID,
		PointsName:      "Rep",
		Enabled:         false,
		Cooldown:        120,
		MaxGiveAmount:   1,
		MaxRemoveAmount: 1,
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

	ErrBlacklistedGive    = UserError("Disallowed from giving points")
	ErrBlacklistedReceive = UserError("Disallowed from receiving points")
	ErrCooldown           = UserError("You're still on cooldown")

	ErrUpdatingRepRoles = UserError("Failed updating rep roles for member")
)

func ModifyRep(ctx context.Context, conf *models.ReputationConfig, gs *dstate.GuildSet, sender, receiver *dstate.MemberState, amount int64) (err error) {
	if conf == nil {
		conf, err = GetConfig(ctx, gs.ID)
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

	ok, err := CheckSetCooldown(conf, sender.User.ID)
	if err != nil || !ok {
		if err == nil {
			err = ErrCooldown
		}
		return
	}

	newRep, err := insertUpdateUserRep(ctx, gs.ID, receiver.User.ID, amount)
	if err != nil {
		// Clear the cooldown since it failed updating the rep
		ClearCooldown(gs.ID, sender.User.ID)
		return
	}

	if err := UpdateRepRoles(gs, receiver, newRep); err != nil {
		logger.WithField("guild_id", gs.ID).Errorf("failed updating rep roles: %s", err)
		return ErrUpdatingRepRoles
	}

	receiverUsername := receiver.User.String()
	senderUsername := sender.User.String()

	// Add the log element
	entry := &models.ReputationLog{
		GuildID:          gs.ID,
		SenderID:         sender.User.ID,
		SenderUsername:   senderUsername,
		ReceiverID:       receiver.User.ID,
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

const (
	MaxRepRoles        = 5
	MaxRepRolesPremium = 25
)

func GuildMaxRepRoles(guildID int64) int {
	if isPrem, _ := premium.IsGuildPremium(guildID); isPrem {
		return MaxRepRolesPremium
	}
	return MaxRepRoles
}

func UpdateRepRoles(gs *dstate.GuildSet, ms *dstate.MemberState, rep int64) error {
	repRoles, err := models.ReputationRoles(models.ReputationRoleWhere.GuildID.EQ(gs.ID)).AllG(context.Background())
	if err != nil {
		return err
	}
	if len(repRoles) == 0 {
		return nil // nothing to do
	}

	botMember, err := bot.GetMember(gs.ID, common.BotUser.ID)
	if err != nil {
		return err
	}
	botHighestRole := bot.MemberHighestRole(gs, botMember)

	add, remove := computeRoleChanges(ms.Member.Roles, rep, repRoles)
	add, remove = filterActionable(add, gs, botHighestRole), filterActionable(remove, gs, botHighestRole)
	if len(add) == 0 && len(remove) == 0 {
		return nil // nothing to do
	}

	// Can we edit the member's roles in one go?
	if common.IsRoleAbove(botHighestRole, bot.MemberHighestRole(gs, ms)) {
		return bulkEditRoles(ms, add, remove)
	}

	// Otherwise, we need to add and remove roles one by one to avoid a permissions error.
	// Issue the API requests sequentially to help avoid ratelimits.
	for _, r := range add {
		if err = common.BotSession.GuildMemberRoleAdd(gs.ID, ms.User.ID, r); err != nil {
			return err
		}
	}
	for _, r := range remove {
		if err := common.BotSession.GuildMemberRoleRemove(gs.ID, ms.User.ID, r); err != nil {
			return err
		}
	}
	return nil
}

func bulkEditRoles(ms *dstate.MemberState, add []int64, remove []int64) error {
	expectedLen := len(ms.Member.Roles) + len(add) - len(remove)
	newRoles := make([]string, 0, max(expectedLen, 0))
	for _, r := range ms.Member.Roles {
		// O(n) but `remove` should be small so it's OK.
		if !slices.Contains(remove, r) {
			newRoles = append(newRoles, discordgo.StrID(r))
		}
	}
	for _, r := range add {
		newRoles = append(newRoles, discordgo.StrID(r))
	}

	return common.BotSession.GuildMemberEdit(ms.GuildID, ms.User.ID, newRoles)
}

func filterActionable(roles []int64, gs *dstate.GuildSet, botHighest *discordgo.Role) []int64 {
	return slices.DeleteFunc(roles, func(r int64) bool {
		rr := gs.GetRole(r)
		return rr == nil || !common.IsRoleAbove(botHighest, rr)
	})
}

func computeRoleChanges(memberRoles []int64, rep int64, repRoles models.ReputationRoleSlice) (add []int64, remove []int64) {
	for _, rr := range repRoles {
		switch {
		case rep >= rr.RepThreshold:
			if !slices.Contains(memberRoles, rr.Role) {
				add = append(add, rr.Role)
			}
		case rep < rr.RepThreshold:
			if slices.Contains(memberRoles, rr.Role) {
				remove = append(remove, rr.Role)
			}
		}
	}

	// deduplicate
	slices.Sort(add)
	slices.Sort(remove)
	return slices.Compact(add), slices.Compact(remove)
}

func insertUpdateUserRep(ctx context.Context, guildID, userID int64, amount int64) (newRep int64, err error) {

	// upsert query which is too advanced for orms
	const query = `
INSERT INTO reputation_users (created_at, guild_id, user_id, points)
VALUES ($1, $2, $3, $4)
ON CONFLICT (guild_id, user_id)
DO UPDATE SET points = reputation_users.points + $4
RETURNING points;
`
	row := common.PQ.QueryRowContext(ctx, query, time.Now(), guildID, userID, amount)
	err = row.Scan(&newRep)
	return
}

// Returns a user error if the sender can not modify the rep of receiver
// Admins are always able to modify the rep of everyone
func CanModifyRep(conf *models.ReputationConfig, sender, receiver *dstate.MemberState) error {
	if common.ContainsInt64SliceOneOf(sender.Member.Roles, conf.AdminRoles) {
		return nil
	}

	if len(conf.RequiredGiveRoles) > 0 && !common.ContainsInt64SliceOneOf(sender.Member.Roles, conf.RequiredGiveRoles) {
		return ErrMissingRequiredGiveRole
	}

	if len(conf.RequiredReceiveRoles) > 0 && !common.ContainsInt64SliceOneOf(receiver.Member.Roles, conf.RequiredReceiveRoles) {
		return ErrMissingRequiredReceiveRole
	}

	if common.ContainsInt64SliceOneOf(sender.Member.Roles, conf.BlacklistedGiveRoles) {
		return ErrBlacklistedGive
	}

	if common.ContainsInt64SliceOneOf(receiver.Member.Roles, conf.BlacklistedReceiveRoles) {
		return ErrBlacklistedReceive
	}

	return nil
}

func IsAdmin(gs *dstate.GuildSet, member *dstate.MemberState, config *models.ReputationConfig) bool {

	memberPerms, _ := gs.GetMemberPermissions(0, member.User.ID, member.Member.Roles)

	if memberPerms&discordgo.PermissionManageGuild != 0 {
		return true
	}

	if common.ContainsInt64SliceOneOf(member.Member.Roles, config.AdminRoles) {
		return true
	}

	return false
}

func SetRep(ctx context.Context, gs *dstate.GuildSet, sender, receiver *dstate.MemberState, points int64) error {
	user := &models.ReputationUser{
		GuildID: gs.ID,
		UserID:  receiver.User.ID,
		Points:  points,
	}

	err := user.UpsertG(ctx, true, []string{"guild_id", "user_id"}, boil.Whitelist("points"), boil.Infer())
	if err != nil {
		return err
	}

	if err := UpdateRepRoles(gs, receiver, points); err != nil {
		logger.WithField("guild_id", gs.ID).Errorf("failed updating rep roles: %s", err)
		return ErrUpdatingRepRoles
	}

	// Insert log entry
	entry := &models.ReputationLog{
		GuildID:        gs.ID,
		SenderID:       sender.User.ID,
		ReceiverID:     receiver.User.ID,
		SetFixedAmount: true,
		Amount:         points,
	}

	err = entry.InsertG(ctx, boil.Infer())
	return errors.WithMessage(err, "SetRep log entry.Insert")
}

func DelRep(ctx context.Context, gs *dstate.GuildSet, userID int64) error {
	_, err := models.ReputationUsers(qm.Where("guild_id = ? AND user_id = ?", gs.ID, userID)).DeleteAll(ctx, common.PQ)
	if err != nil {
		return err
	}

	// Try to update the ms's rep roles if we can find them.
	ms, err := bot.GetMember(gs.ID, userID)
	if err != nil {
		// Ignore the error; presumably the member isn't in the server anymore.
		return nil
	}

	if err := UpdateRepRoles(gs, ms, 0); err != nil {
		logger.WithField("guild_id", gs.ID).Errorf("failed updating rep roles: %s", err)
		return ErrUpdatingRepRoles
	}
	return nil
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
				members = append(members, v.DgoMember())
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
				lEntry.Username = m.User.String()
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
