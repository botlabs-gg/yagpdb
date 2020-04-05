package common

import (
	"bytes"
	"database/sql"
	"fmt"
	"math/rand"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"time"

	"emperror.dev/errors"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/lib/pq"
	"github.com/mediocregopher/radix/v3"
	"github.com/sirupsen/logrus"
)

func KeyGuild(guildID int64) string         { return "guild:" + discordgo.StrID(guildID) }
func KeyGuildChannels(guildID int64) string { return "channels:" + discordgo.StrID(guildID) }

var LinkRegex = regexp.MustCompile(`(http(s)?:\/\/)?(www\.)?[-a-zA-Z0-9@:%_\+~#=]{1,256}\.[a-z]{2,6}\b([-a-zA-Z0-9@:%_\+.~#?&//=]*)`)

type GuildWithConnected struct {
	*discordgo.UserGuild
	Connected bool
}

// GetGuildsWithConnected Returns a wrapped guild with connected set
func GetGuildsWithConnected(in []*discordgo.UserGuild) ([]*GuildWithConnected, error) {
	if len(in) < 1 {
		return []*GuildWithConnected{}, nil
	}

	out := make([]*GuildWithConnected, len(in))

	for i, g := range in {
		out[i] = &GuildWithConnected{
			UserGuild: g,
			Connected: false,
		}

		err := RedisPool.Do(radix.Cmd(&out[i].Connected, "SISMEMBER", "connected_guilds", strconv.FormatInt(g.ID, 10)))
		if err != nil {
			return nil, err
		}
	}

	return out, nil
}

// DelayedMessageDelete Deletes a message after delay
func DelayedMessageDelete(session *discordgo.Session, delay time.Duration, cID, mID int64) {
	time.Sleep(delay)
	err := session.ChannelMessageDelete(cID, mID)
	if err != nil {
		logger.WithError(err).Error("Failed deleting message")
	}
}

// SendTempMessage sends a message that gets deleted after duration
func SendTempMessage(session *discordgo.Session, duration time.Duration, cID int64, msg string) {
	m, err := BotSession.ChannelMessageSendComplex(cID, &discordgo.MessageSend{
		Content:         msg,
		AllowedMentions: AllowedMentionsParseUsers,
	})
	if err != nil {
		return
	}

	DelayedMessageDelete(session, duration, cID, m.ID)
}

// GetGuildChannels returns the guilds channels either from cache or api
func GetGuildChannels(guildID int64) (channels []*discordgo.Channel, err error) {
	// Check cache first
	err = GetCacheDataJson(KeyGuildChannels(guildID), &channels)
	if err != nil {
		channels, err = BotSession.GuildChannels(guildID)
		if err == nil {
			SetCacheDataJsonSimple(KeyGuildChannels(guildID), channels)
		}
	}

	return
}

// GetGuild returns the guild from guildid either from cache or api
func GetGuild(guildID int64) (guild *discordgo.Guild, err error) {
	// Check cache first
	err = GetCacheDataJson(KeyGuild(guildID), &guild)
	if err != nil {
		guild, err = BotSession.Guild(guildID)
		if err == nil {
			SetCacheDataJsonSimple(KeyGuild(guildID), guild)
		}
	}

	return
}

func RandomAdjective() string {
	return Adjectives[rand.Intn(len(Adjectives))]
}

func RandomNoun() string {
	return Nouns[rand.Intn(len(Nouns))]
}

type DurationFormatPrecision int

const (
	DurationPrecisionSeconds DurationFormatPrecision = iota
	DurationPrecisionMinutes
	DurationPrecisionHours
	DurationPrecisionDays
	DurationPrecisionWeeks
	DurationPrecisionYears
)

func (d DurationFormatPrecision) String() string {
	switch d {
	case DurationPrecisionSeconds:
		return "second"
	case DurationPrecisionMinutes:
		return "minute"
	case DurationPrecisionHours:
		return "hour"
	case DurationPrecisionDays:
		return "day"
	case DurationPrecisionWeeks:
		return "week"
	case DurationPrecisionYears:
		return "year"
	}
	return "Unknown"
}

func (d DurationFormatPrecision) FromSeconds(in int64) int64 {
	switch d {
	case DurationPrecisionSeconds:
		return in % 60
	case DurationPrecisionMinutes:
		return (in / 60) % 60
	case DurationPrecisionHours:
		return ((in / 60) / 60) % 24
	case DurationPrecisionDays:
		return (((in / 60) / 60) / 24) % 7
	case DurationPrecisionWeeks:
		// There's 52 weeks + 1 day per year (techically +1.25... but were doing +1)
		// Make sure 364 days isnt 0 weeks and 0 years
		days := (((in / 60) / 60) / 24) % 365
		return days / 7
	case DurationPrecisionYears:
		return (((in / 60) / 60) / 24) / 365
	}

	panic("We shouldn't be here")

	return 0
}

func pluralize(val int64) string {
	if val == 1 {
		return ""
	}
	return "s"
}

func HumanizeDuration(precision DurationFormatPrecision, in time.Duration) string {
	seconds := int64(in.Seconds())

	out := make([]string, 0)

	for i := int(precision); i < int(DurationPrecisionYears)+1; i++ {
		curPrec := DurationFormatPrecision(i)
		units := curPrec.FromSeconds(seconds)
		if units > 0 {
			out = append(out, fmt.Sprintf("%d %s%s", units, curPrec.String(), pluralize(units)))
		}
	}

	outStr := ""

	for i := len(out) - 1; i >= 0; i-- {
		if i == 0 && i != len(out)-1 {
			outStr += " and "
		} else if i != len(out)-1 {
			outStr += " "
		}
		outStr += out[i]
	}

	if outStr == "" {
		outStr = "less than 1 " + precision.String()
	}

	return outStr
}

func HumanizeTime(precision DurationFormatPrecision, in time.Time) string {

	now := time.Now()
	if now.After(in) {
		duration := now.Sub(in)
		return HumanizeDuration(precision, duration) + " ago"
	} else {
		duration := in.Sub(now)
		return "in " + HumanizeDuration(precision, duration)
	}
}

func FallbackEmbed(embed *discordgo.MessageEmbed) string {
	body := ""

	if embed.Title != "" {
		body += embed.Title + "\n"
	}

	if embed.Description != "" {
		body += embed.Description + "\n"
	}
	if body != "" {
		body += "\n"
	}

	for _, v := range embed.Fields {
		body += fmt.Sprintf("**%s**\n%s\n\n", v.Name, v.Value)
	}
	return body + "**I have no 'embed links' permissions here, this is a fallback. it looks prettier if i have that perm :)**"
}

// CutStringShort stops a strinng at "l"-3 if it's longer than "l" and adds "..."
func CutStringShort(s string, l int) string {
	var mainBuf bytes.Buffer
	var latestBuf bytes.Buffer

	i := 0
	for _, r := range s {
		latestBuf.WriteRune(r)
		if i > 3 {
			lRune, _, _ := latestBuf.ReadRune()
			mainBuf.WriteRune(lRune)
		}

		if i >= l {
			return mainBuf.String() + "..."
		}
		i++
	}

	return mainBuf.String() + latestBuf.String()
}

type SmallModel struct {
	ID        uint `gorm:"primary_key"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func MustParseInt(s string) int64 {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		panic("Failed parsing int: " + err.Error())
	}

	return i
}

func AddRole(member *discordgo.Member, role int64, guildID int64) error {
	for _, v := range member.Roles {
		if v == role {
			// Already has the role
			return nil
		}
	}

	return BotSession.GuildMemberRoleAdd(guildID, member.User.ID, role)
}

func AddRoleDS(ms *dstate.MemberState, role int64) error {
	for _, v := range ms.Roles {
		if v == role {
			// Already has the role
			return nil
		}
	}

	return BotSession.GuildMemberRoleAdd(ms.Guild.ID, ms.ID, role)
}

func RemoveRole(member *discordgo.Member, role int64, guildID int64) error {
	for _, r := range member.Roles {
		if r == role {
			return BotSession.GuildMemberRoleRemove(guildID, member.User.ID, r)
		}
	}

	// Never had the role in the first place if we got here
	return nil
}

func RemoveRoleDS(ms *dstate.MemberState, role int64) error {
	for _, r := range ms.Roles {
		if r == role {
			return BotSession.GuildMemberRoleRemove(ms.Guild.ID, ms.ID, r)
		}
	}

	// Never had the role in the first place if we got here
	return nil
}

var StringPerms = map[int]string{
	discordgo.PermissionReadMessages:       "Read Messages",
	discordgo.PermissionSendMessages:       "Send Messages",
	discordgo.PermissionSendTTSMessages:    "Send TTS Messages",
	discordgo.PermissionManageMessages:     "Manage Messages",
	discordgo.PermissionEmbedLinks:         "Embed Links",
	discordgo.PermissionAttachFiles:        "Attach Files",
	discordgo.PermissionReadMessageHistory: "Read Message History",
	discordgo.PermissionMentionEveryone:    "Mention Everyone",
	discordgo.PermissionVoiceConnect:       "Voice Connect",
	discordgo.PermissionVoiceSpeak:         "Voice Speak",
	discordgo.PermissionVoiceMuteMembers:   "Voice Mute Members",
	discordgo.PermissionVoiceDeafenMembers: "Voice Deafen Members",
	discordgo.PermissionVoiceMoveMembers:   "Voice Move Members",
	discordgo.PermissionVoiceUseVAD:        "Voice Use VAD",

	discordgo.PermissionCreateInstantInvite: "Create Instant Invite",
	discordgo.PermissionKickMembers:         "Kick Members",
	discordgo.PermissionBanMembers:          "Ban Members",
	discordgo.PermissionManageRoles:         "Manage Roles",
	discordgo.PermissionManageChannels:      "Manage Channels",
	discordgo.PermissionManageServer:        "Manage Server",
	discordgo.PermissionManageWebhooks:      "Manage Webhooks",
}

func ErrWithCaller(err error) error {
	pc, _, _, ok := runtime.Caller(1)
	if !ok {
		panic("No caller")
	}

	f := runtime.FuncForPC(pc)
	return errors.WithMessage(err, filepath.Base(f.Name()))
}

// DiscordError extracts the errorcode discord sent us
func DiscordError(err error) (code int, msg string) {
	err = errors.Cause(err)

	if rError, ok := err.(*discordgo.RESTError); ok && rError.Message != nil {
		return rError.Message.Code, rError.Message.Message
	}

	return 0, ""
}

// IsDiscordErr returns true if this was a discord error and one of the codes matches
func IsDiscordErr(err error, codes ...int) bool {
	code, _ := DiscordError(err)

	for _, v := range codes {
		if code == v {
			return true
		}
	}

	return false
}

type LoggedExecutedCommand struct {
	SmallModel

	UserID    string
	ChannelID string
	GuildID   string

	// Name of command that was triggered
	Command string
	// Raw command with arguments passed
	RawCommand string
	// If command returned any error this will be no-empty
	Error string

	TimeStamp    time.Time
	ResponseTime int64
}

func (l LoggedExecutedCommand) TableName() string {
	return "executed_commands"
}

func HumanizePermissions(perms int64) (res []string) {
	if perms&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator {
		res = append(res, "Administrator")
	}
	if perms&discordgo.PermissionManageServer == discordgo.PermissionManageServer {
		res = append(res, "ManageServer")
	}

	if perms&discordgo.PermissionReadMessages == discordgo.PermissionReadMessages {
		res = append(res, "ReadMessages")
	}
	if perms&discordgo.PermissionSendMessages == discordgo.PermissionSendMessages {
		res = append(res, "SendMessages")
	}
	if perms&discordgo.PermissionSendTTSMessages == discordgo.PermissionSendTTSMessages {
		res = append(res, "SendTTSMessages")
	}
	if perms&discordgo.PermissionManageMessages == discordgo.PermissionManageMessages {
		res = append(res, "ManageMessages")
	}
	if perms&discordgo.PermissionEmbedLinks == discordgo.PermissionEmbedLinks {
		res = append(res, "EmbedLinks")
	}
	if perms&discordgo.PermissionAttachFiles == discordgo.PermissionAttachFiles {
		res = append(res, "AttachFiles")
	}
	if perms&discordgo.PermissionReadMessageHistory == discordgo.PermissionReadMessageHistory {
		res = append(res, "ReadMessageHistory")
	}
	if perms&discordgo.PermissionMentionEveryone == discordgo.PermissionMentionEveryone {
		res = append(res, "MentionEveryone")
	}
	if perms&discordgo.PermissionUseExternalEmojis == discordgo.PermissionUseExternalEmojis {
		res = append(res, "UseExternalEmojis")
	}

	// Constants for the different bit offsets of voice permissions
	if perms&discordgo.PermissionVoiceConnect == discordgo.PermissionVoiceConnect {
		res = append(res, "VoiceConnect")
	}
	if perms&discordgo.PermissionVoiceSpeak == discordgo.PermissionVoiceSpeak {
		res = append(res, "VoiceSpeak")
	}
	if perms&discordgo.PermissionVoiceMuteMembers == discordgo.PermissionVoiceMuteMembers {
		res = append(res, "VoiceMuteMembers")
	}
	if perms&discordgo.PermissionVoiceDeafenMembers == discordgo.PermissionVoiceDeafenMembers {
		res = append(res, "VoiceDeafenMembers")
	}
	if perms&discordgo.PermissionVoiceMoveMembers == discordgo.PermissionVoiceMoveMembers {
		res = append(res, "VoiceMoveMembers")
	}
	if perms&discordgo.PermissionVoiceUseVAD == discordgo.PermissionVoiceUseVAD {
		res = append(res, "VoiceUseVAD")
	}

	// Constants for general management.
	if perms&discordgo.PermissionChangeNickname == discordgo.PermissionChangeNickname {
		res = append(res, "ChangeNickname")
	}
	if perms&discordgo.PermissionManageNicknames == discordgo.PermissionManageNicknames {
		res = append(res, "ManageNicknames")
	}
	if perms&discordgo.PermissionManageRoles == discordgo.PermissionManageRoles {
		res = append(res, "ManageRoles")
	}
	if perms&discordgo.PermissionManageWebhooks == discordgo.PermissionManageWebhooks {
		res = append(res, "ManageWebhooks")
	}
	if perms&discordgo.PermissionManageEmojis == discordgo.PermissionManageEmojis {
		res = append(res, "ManageEmojis")
	}

	if perms&discordgo.PermissionCreateInstantInvite == discordgo.PermissionCreateInstantInvite {
		res = append(res, "CreateInstantInvite")
	}
	if perms&discordgo.PermissionKickMembers == discordgo.PermissionKickMembers {
		res = append(res, "KickMembers")
	}
	if perms&discordgo.PermissionBanMembers == discordgo.PermissionBanMembers {
		res = append(res, "BanMembers")
	}
	if perms&discordgo.PermissionManageChannels == discordgo.PermissionManageChannels {
		res = append(res, "ManageChannels")
	}
	if perms&discordgo.PermissionAddReactions == discordgo.PermissionAddReactions {
		res = append(res, "AddReactions")
	}
	if perms&discordgo.PermissionViewAuditLogs == discordgo.PermissionViewAuditLogs {
		res = append(res, "ViewAuditLogs")
	}

	return
}

func LogIgnoreError(err error, msg string, data logrus.Fields) {
	if err == nil {
		return
	}

	l := logger.WithError(err)
	if data != nil {
		l = l.WithFields(data)
	}

	l.Error(msg)
}

func ErrPQIsUniqueViolation(err error) bool {
	if err == nil {
		return false
	}

	if cast, ok := errors.Cause(err).(*pq.Error); ok {
		if cast.Code == "23505" {
			return true
		}
	}

	return false
}

func GetJoinedServerCount() (int64, error) {
	var count int64
	err := RedisPool.Do(radix.Cmd(&count, "SCARD", "connected_guilds"))
	return count, err
}

func BotIsOnGuild(guildID int64) (bool, error) {
	isOnGuild := false
	err := RedisPool.Do(radix.FlatCmd(&isOnGuild, "SISMEMBER", "connected_guilds", guildID))
	return isOnGuild, err
}

func GetActiveNodes() ([]string, error) {
	var nodes []string
	err := RedisPool.Do(radix.FlatCmd(&nodes, "ZRANGEBYSCORE", "dshardorchestrator_nodes_z", time.Now().Add(-time.Minute).Unix(), "+inf"))
	return nodes, err
}

// helper for creating transactions
func SqlTX(f func(tx *sql.Tx) error) error {
	tx, err := PQ.Begin()
	if err != nil {
		return err
	}

	err = f(tx)

	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func SendOwnerAlert(msgf string, args ...interface{}) {
	mainOwner := int64(0)
	if len(BotOwners) > 0 {
		mainOwner = BotOwners[0]
	}
	ch, err := BotSession.UserChannelCreate(int64(mainOwner))
	if err != nil {
		return
	}

	BotSession.ChannelMessageSend(ch.ID, fmt.Sprintf(msgf, args...))
}

func IsOwner(userID int64) bool {
	for _, v := range BotOwners {
		if v == userID {
			return true
		}
	}

	return false
}

var AllowedMentionsParseUsers = discordgo.AllowedMentions{
	Parse: []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers},
}
