package common

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"fmt"
	"math/rand"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/lib/pq"
	"github.com/mediocregopher/radix/v3"
	"github.com/microcosm-cc/bluemonday"
	"github.com/sirupsen/logrus"
)

func KeyGuild(guildID int64) string         { return "guild:" + discordgo.StrID(guildID) }
func KeyGuildChannels(guildID int64) string { return "channels:" + discordgo.StrID(guildID) }

var LinkRegex = regexp.MustCompile(`(?i)([a-z\d]+://)([\w-._~:/?#\[\]@!$&'()*+,;%=]+(?:(?:\.[\w_-]+)+))([\w.,@?^=%&:/~+#-]*[\w@?^=%&/~+#-])`)
var DomainFinderRegex = regexp.MustCompile(`(?i)(?:[a-z\d](?:[a-z\d-]{0,61}[a-z\d])?\.)+[a-z\d][a-z\d-]{0,61}[a-z\d]`)
var UGCHtmlPolicy = bluemonday.NewPolicy().AllowElements("h1", "h2", "h3", "h4", "h5", "h6", "p", "ol", "ul", "li", "dl", "dd", "dt", "blockquote", "table", "thead", "th", "tr", "td", "tbody", "del", "i", "b")
var ForwardSlashReplacer = strings.NewReplacer("\\", "")

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

func RandomAdjective() string {
	return Adjectives[rand.Intn(len(Adjectives))]
}

func RandomNoun() string {
	return Nouns[rand.Intn(len(Nouns))]
}

func RandomVerb() string {
	return Verbs[rand.Intn(len(Verbs))]
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
		days := (((in / 60) / 60) / 24)
		// 365 % 7 == 1, meaning calculating days based on remainder after dividing
		// into weeks for a year would leave us with 1 extra day. This wouldn't be
		// a problem if we stopped at weeks, since 365 days == 52 weeks and 1 day,
		// however the weeks mod out to 0 so that 365 days properly becomes a year.
		// 52 weeks ≠ 1 year. Resultingly, we need to skip every 365th day.
		return (days - days/365) % 7
	case DurationPrecisionWeeks:
		// There's 52 weeks + 1 day per year (techically +1.25... but were doing +1)
		// Make sure 364 days isnt 0 weeks and 0 years
		days := (((in / 60) / 60) / 24) % 365
		return days / 7
	case DurationPrecisionYears:
		return (((in / 60) / 60) / 24) / 365
	}

	panic("We shouldn't be here")
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
	for _, v := range ms.Member.Roles {
		if v == role {
			// Already has the role
			return nil
		}
	}

	return BotSession.GuildMemberRoleAdd(ms.GuildID, ms.User.ID, role)
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
	for _, r := range ms.Member.Roles {
		if r == role {
			return BotSession.GuildMemberRoleRemove(ms.GuildID, ms.User.ID, r)
		}
	}

	// Never had the role in the first place if we got here
	return nil
}

var StringPerms = map[int64]string{
	discordgo.PermissionViewChannel:        "View Channel",
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
	discordgo.PermissionManageGuild:         "Manage Guild",
	discordgo.PermissionManageWebhooks:      "Manage Webhooks",
	discordgo.PermissionModerateMembers:     "Moderate Members / Timeout Members",
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

// for backward compatibility with previous implementations of HumanizePermissions
var legacyPermNames = map[int64]string{
	discordgo.PermissionManageGuild:  "ManageServer",
	discordgo.PermissionViewChannel:  "ReadMessages",
	discordgo.PermissionViewAuditLog: "ViewAuditLogs",
}

func HumanizePermissions(perms int64) (res []string) {
	for _, p := range discordgo.AllPermissions {
		if perms&p == p {
			if legacyName, ok := legacyPermNames[p]; ok {
				res = append(res, legacyName)
			} else {
				res = append(res, discordgo.PermissionName(p))
			}
		}
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

func LogLongCallTime(treshold time.Duration, isErr bool, logMsg string, extraData logrus.Fields, f func()) {
	started := time.Now()
	f()
	elapsed := time.Since(started)

	if elapsed > treshold {
		l := logrus.WithFields(extraData).WithField("elapsed", elapsed.String())
		if isErr {
			l.Error(logMsg)
		} else {
			l.Warn(logMsg)
		}
	}
}

func SplitSendMessage(channelID int64, contents string, allowedMentions discordgo.AllowedMentions) ([]*discordgo.Message, error) {
	result := make([]*discordgo.Message, 0, 1)

	split := dcmd.SplitString(contents, 2000)
	for _, v := range split {
		var err error
		var m *discordgo.Message
		m, err = BotSession.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
			Content:         v,
			AllowedMentions: allowedMentions,
		})

		if err != nil {
			return result, err
		}

		result = append(result, m)
	}

	return result, nil
}

func FormatList(list []string, conjunction string) string {
	var sb strings.Builder
	for i, item := range list {
		if i > 0 {
			sb.WriteString(", ")
			if i == len(list)-1 {
				sb.WriteString(conjunction)
				if conjunction != "" {
					sb.WriteByte(' ')
				}
			}
		}
		sb.WriteByte('`')
		sb.WriteString(item)
		sb.WriteByte('`')
	}
	return sb.String()
}

func BoolToPointer(b bool) *bool {
	return &b
}

// Also accepts spaces due to how dcmd reconstructs arguments wrapped in triple backticks.
var codeblockRegexp = regexp.MustCompile(`(?m)\A(?:\x60{2} ?\x60)(?:.*\n)?([\S\s]+)(?:\x60 ?\x60{2})\z`)

// parseCodeblock returns the content wrapped in a Discord markdown block.
// If no (valid) codeblock was found, the given input is returned back.
func ParseCodeblock(input string) string {
	parts := codeblockRegexp.FindStringSubmatch(input)

	// No match found, input was not wrapped in (valid) codeblock markdown
	// just dump it, don't bother fixing things for the user.
	if parts == nil {
		return input
	}

	logger.Debugf("Found matches: %#v", parts)
	logger.Debugf("Returning %s", parts[1])
	return parts[1]
}

func Base64DecodeToString(str string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
