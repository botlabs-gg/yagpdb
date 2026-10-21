package moderation

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/botlabs-gg/yagpdb/v2/moderation/models"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
)

const (
	banAppealUnbanCustomIDPrefix  = "moderation-appeal-unban-"
	banAppealRejectCustomIDPrefix = "moderation-appeal-reject-"

	banAppealColorPending  = 0xfaa61a
	banAppealColorUnbanned = 0x43b581
	banAppealColorRejected = 0xf04747

	banAppealAcceptedUnbanReason = "Ban appeal accepted"
)

// The bot bans with "moderator: reason" so the audit log shows who banned,
// and Discord usernames cannot contain a colon.
var banReasonModeratorPrefix = regexp.MustCompile(`^((?:[a-z0-9_.]{2,32}|[^:#\n]{2,32}#\d{4})): `)

func splitBanReason(reason string) (moderator, rest string) {
	match := banReasonModeratorPrefix.FindStringSubmatch(reason)
	if match == nil {
		return "", strings.TrimSpace(reason)
	}

	return match[1], strings.TrimSpace(reason[len(match[0]):])
}

// BanReasonForBannedUser leaves out the moderator the bot names in its ban reasons.
func BanReasonForBannedUser(reason string) string {
	_, rest := splitBanReason(reason)
	return rest
}

var markdownEscaper = strings.NewReplacer(`\`, `\\`, "*", `\*`, "_", `\_`, "~", `\~`, "`", "\\`", "|", `\|`, ">", `\>`)

// postBanAppealForReview withdraws the appeal when it cannot be posted, so the
// user is free to try again rather than waiting on an appeal nobody can see.
func postBanAppealForReview(ctx context.Context, config *Config, appeal *models.ModerationBanAppeal, user *discordgo.User) error {
	channelID := config.BanAppealsReviewChannel()

	msg, err := common.BotSession.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Embeds:          []*discordgo.MessageEmbed{banAppealEmbed(appeal, user)},
		Components:      banAppealReviewButtons(appeal.ID),
		AllowedMentions: discordgo.AllowedMentions{},
	})
	if err != nil {
		if _, deleteErr := appeal.DeleteG(ctx); deleteErr != nil {
			logger.WithError(deleteErr).WithField("guild", appeal.GuildID).Error("failed withdrawing undelivered ban appeal")
		}

		sendFailedDMError(appeal.GuildID, config.ErrorChannel, fmt.Sprintf("Failed posting a ban appeal from <@%d> to <#%d>, the user was asked to try again.\nError: `%v`", appeal.UserID, channelID, err))
		return err
	}

	appeal.ReviewChannelID = channelID
	appeal.ReviewMessageID = msg.ID
	_, err = appeal.UpdateG(ctx, boil.Whitelist("review_channel_id", "review_message_id", "updated_at"))
	return err
}

func banAppealEmbed(appeal *models.ModerationBanAppeal, user *discordgo.User) *discordgo.MessageEmbed {
	moderator, banReason := splitBanReason(appeal.BanReason)
	if banReason == "" {
		banReason = "*No reason given*"
	}

	fields := []*discordgo.MessageEmbedField{
		{Name: "User", Value: fmt.Sprintf("<@%d>", user.ID), Inline: true},
		{Name: "User ID", Value: fmt.Sprintf("`%d`", user.ID), Inline: true},
	}
	if moderator != "" {
		fields = append(fields, &discordgo.MessageEmbedField{Name: "Banned By", Value: markdownEscaper.Replace(moderator), Inline: true})
	}
	fields = append(fields, &discordgo.MessageEmbedField{Name: "Ban Reason", Value: common.CutStringShort(banReason, 1024)})

	return &discordgo.MessageEmbed{
		Title: "Ban Appeal",
		Author: &discordgo.MessageEmbedAuthor{
			Name:    user.String(),
			IconURL: user.AvatarURL("256"),
		},
		Description: appeal.Appeal,
		Color:       banAppealColorPending,
		Fields:      fields,
		Timestamp:   appeal.CreatedAt.Format(time.RFC3339),
	}
}

func banAppealReviewButtons(appealID int64) []discordgo.TopLevelComponent {
	return []discordgo.TopLevelComponent{
		discordgo.ActionsRow{Components: []discordgo.InteractiveComponent{
			discordgo.Button{
				Label:    "Unban",
				Style:    discordgo.SuccessButton,
				CustomID: fmt.Sprintf("%s%d", banAppealUnbanCustomIDPrefix, appealID),
			},
			discordgo.Button{
				Label:    "Reject",
				Style:    discordgo.DangerButton,
				CustomID: fmt.Sprintf("%s%d", banAppealRejectCustomIDPrefix, appealID),
			},
		}},
	}
}

func parseBanAppealReviewCustomID(customID string) (unban bool, appealID int64, ok bool) {
	var idPart string
	switch {
	case strings.HasPrefix(customID, banAppealUnbanCustomIDPrefix):
		unban, idPart = true, strings.TrimPrefix(customID, banAppealUnbanCustomIDPrefix)
	case strings.HasPrefix(customID, banAppealRejectCustomIDPrefix):
		idPart = strings.TrimPrefix(customID, banAppealRejectCustomIDPrefix)
	default:
		return false, 0, false
	}

	appealID, err := strconv.ParseInt(idPart, 10, 64)
	return unban, appealID, err == nil
}

// resolveBanAppealReview shows the outcome on the review message and removes
// its buttons, so an appeal cannot be decided twice from it.
func resolveBanAppealReview(channelID, messageID int64, embed *discordgo.MessageEmbed, status string, color int) {
	embed.Color = color
	embed.Footer = nil
	embed.Fields = slices.DeleteFunc(embed.Fields, func(f *discordgo.MessageEmbedField) bool { return f.Name == "Status" })
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Status", Value: status})

	_, err := common.BotSession.ChannelMessageEditComplex(&discordgo.MessageEdit{
		ID:         messageID,
		Channel:    channelID,
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: []discordgo.TopLevelComponent{},
	})
	if err != nil {
		logger.WithError(err).Error("failed updating ban appeal review message")
	}
}

func markBanAppealReviewClosed(appeal *models.ModerationBanAppeal) {
	msg, err := common.BotSession.ChannelMessage(appeal.ReviewChannelID, appeal.ReviewMessageID)
	if err != nil || len(msg.Embeds) == 0 {
		return
	}

	resolveBanAppealReview(appeal.ReviewChannelID, appeal.ReviewMessageID, msg.Embeds[0],
		fmt.Sprintf("Closed, the user was unbanned <t:%d:R>", appeal.ClosedAt.Time.Unix()), banAppealColorUnbanned)
}

func handleBanAppealReviewInteraction(evt *eventsystem.EventData) (retry bool, err error) {
	ic := evt.InteractionCreate()
	if ic.GuildID == 0 || ic.Type != discordgo.InteractionMessageComponent || ic.Member == nil || ic.Message == nil || len(ic.Message.Embeds) == 0 {
		return false, nil
	}

	unban, appealID, ok := parseBanAppealReviewCustomID(ic.MessageComponentData().CustomID)
	if !ok {
		return false, nil
	}

	err = evt.Session.CreateInteractionResponse(ic.ID, ic.Token, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseDeferredMessageUpdate})
	if err != nil {
		return false, err
	}

	reply := func(content string) {
		_, err := evt.Session.CreateFollowupMessage(common.BotApplication.ID, ic.Token, &discordgo.WebhookParams{Content: content, Flags: discordgo.MessageFlagsEphemeral})
		if err != nil {
			logger.WithError(err).Error("failed replying to ban appeal review")
		}
	}

	config, err := FetchConfig(ic.GuildID)
	if err != nil {
		reply("Something went wrong, please try again.")
		return false, err
	}

	if denied := banAppealReviewDeniedReason(config, ic); denied != "" {
		reply(denied)
		return false, nil
	}

	ctx := evt.Context()
	appeal, err := models.ModerationBanAppeals(
		models.ModerationBanAppealWhere.ID.EQ(appealID),
		models.ModerationBanAppealWhere.GuildID.EQ(ic.GuildID),
	).OneG(ctx)
	if err == sql.ErrNoRows {
		reply("This appeal no longer exists.")
		return false, nil
	}
	if err != nil {
		reply("Something went wrong, please try again.")
		return false, err
	}

	if unban {
		return false, unbanFromAppeal(ctx, config, ic, appeal, reply)
	}

	return false, rejectAppeal(ctx, ic, appeal, reply)
}

// banAppealReviewDeniedReason applies the same rules as the ban and unban commands.
func banAppealReviewDeniedReason(config *Config, ic *discordgo.InteractionCreate) string {
	if !config.BanEnabled {
		return "The ban commands are disabled on this server, so appeals cannot be decided here."
	}

	if slices.ContainsFunc(ic.Member.Roles, func(role int64) bool { return slices.Contains(config.BanCmdRoles, role) }) {
		return ""
	}

	hasPerms, err := bot.AdminOrPermMS(ic.GuildID, ic.ChannelID, dstate.MemberStateFromMember(ic.Member), discordgo.PermissionBanMembers)
	if err != nil || !hasPerms {
		return "You need the **Ban Members** permission, or a role allowed to use the ban commands, to decide appeals."
	}

	return ""
}

func unbanFromAppeal(ctx context.Context, config *Config, ic *discordgo.InteractionCreate, appeal *models.ModerationBanAppeal, reply func(string)) error {
	moderator := ic.Member.User
	now := time.Now()

	// Closed before unbanning, so the ban removal event finds nothing left to close.
	claimed, err := models.ModerationBanAppeals(
		models.ModerationBanAppealWhere.ID.EQ(appeal.ID),
		models.ModerationBanAppealWhere.ClosedAt.IsNull(),
		models.ModerationBanAppealWhere.RejectedAt.IsNull(),
	).UpdateAllG(ctx, models.M{"closed_at": now, "resolved_by": moderator.ID, "updated_at": now})
	if err != nil {
		reply("Something went wrong, please try again.")
		return err
	}
	if claimed == 0 {
		reply("This appeal has already been decided.")
		return nil
	}

	target := &discordgo.User{ID: appeal.UserID, Username: "unknown", Discriminator: "????"}
	notBanned, err := UnbanUser(config, ic.GuildID, moderator, banAppealAcceptedUnbanReason, target)
	if err != nil {
		appeal.ClosedAt = null.Time{}
		appeal.ResolvedBy = 0
		if _, reopenErr := appeal.UpdateG(ctx, boil.Whitelist("closed_at", "resolved_by", "updated_at")); reopenErr != nil {
			logger.WithError(reopenErr).WithField("guild", ic.GuildID).Error("failed reopening ban appeal after a failed unban")
		}

		reply("Failed unbanning the user: " + err.Error())
		return err
	}

	status := fmt.Sprintf("Unbanned by %s <t:%d:R>", moderator.Mention(), now.Unix())
	if notBanned {
		status = "Closed, the user was already unbanned"
	}

	resolveBanAppealReview(ic.ChannelID, ic.Message.ID, ic.Message.Embeds[0], status, banAppealColorUnbanned)
	return nil
}

func rejectAppeal(ctx context.Context, ic *discordgo.InteractionCreate, appeal *models.ModerationBanAppeal, reply func(string)) error {
	moderator := ic.Member.User
	now := time.Now()

	rejected, err := models.ModerationBanAppeals(
		models.ModerationBanAppealWhere.ID.EQ(appeal.ID),
		models.ModerationBanAppealWhere.ClosedAt.IsNull(),
		models.ModerationBanAppealWhere.RejectedAt.IsNull(),
	).UpdateAllG(ctx, models.M{"rejected_at": now, "resolved_by": moderator.ID, "updated_at": now})
	if err != nil {
		reply("Something went wrong, please try again.")
		return err
	}
	if rejected == 0 {
		reply("This appeal has already been decided.")
		return nil
	}

	resolveBanAppealReview(ic.ChannelID, ic.Message.ID, ic.Message.Embeds[0],
		fmt.Sprintf("Rejected by %s <t:%d:R>", moderator.Mention(), now.Unix()), banAppealColorRejected)
	return nil
}
