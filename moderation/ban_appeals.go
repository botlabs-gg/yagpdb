package moderation

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2"
	seventsmodels "github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2/models"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/moderation/models"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
)

type BanAppealSource int16

const (
	BanAppealSourceDMButton BanAppealSource = iota
	BanAppealSourceWeb
)

const (
	BanAppealMinLength = 20
	BanAppealMaxLength = 1000

	banAppealButtonCustomIDPrefix = "DM_BAN_APPEAL_"
	banAppealModalCustomIDPrefix  = "DM_BAN_APPEAL_MODAL_"
	banAppealModalInputID         = "appeal"

	banAppealFiledEvent = "moderation_ban_appeal_filed"
)

var (
	ErrBanAppealsDisabled = errors.New("This server does not accept ban appeals.")
	ErrNotBanned          = errors.New("You are not banned from this server.")
	ErrAlreadyAppealed    = errors.New("You have already appealed this ban.")
)

func BanAppealLink(guildID int64) string {
	return fmt.Sprintf("%s/%d/appeal", web.BaseURL(), guildID)
}

func banAppealLinkIfOpen(config *Config, guildID int64) string {
	if !config.BanAppealsOpen() {
		return ""
	}

	return BanAppealLink(guildID)
}

func (c *Config) BanAppealsOpen() bool {
	return c.BanAppealsEnabled && c.BanAppealsReviewChannel() != 0
}

func banAppealDMButtonAvailable(config *Config, guildID int64) bool {
	if !config.BanAppealsOpen() || !config.BanAppealsDMButton {
		return false
	}

	return isPremiumGuild(guildID)
}

func banAppealDMButton(guildID int64) discordgo.Button {
	return discordgo.Button{
		Label:    "Appeal Ban",
		Style:    discordgo.SuccessButton,
		Emoji:    &discordgo.ComponentEmoji{Name: "📝"},
		CustomID: fmt.Sprintf("%s%d", banAppealButtonCustomIDPrefix, guildID),
	}
}

func FindOpenBanAppeal(ctx context.Context, guildID, userID int64) (*models.ModerationBanAppeal, error) {
	appeal, err := models.ModerationBanAppeals(
		models.ModerationBanAppealWhere.GuildID.EQ(guildID),
		models.ModerationBanAppealWhere.UserID.EQ(userID),
		models.ModerationBanAppealWhere.ClosedAt.IsNull(),
	).OneG(ctx)
	if err == sql.ErrNoRows {
		return nil, nil
	}

	return appeal, err
}

// CurrentBan returns nil when the user is not banned.
func CurrentBan(guildID, userID int64) (*discordgo.GuildBan, error) {
	ban, err := common.BotSession.GuildBan(guildID, userID)
	if notFound, _ := isNotFound(err); notFound {
		return nil, nil
	}

	return ban, err
}

// FileBanAppeal stores the appeal, the database allowing only one open appeal
// per ban. It is not yet posted for review.
func FileBanAppeal(ctx context.Context, config *Config, guildID, userID int64, source BanAppealSource, appeal string) (*models.ModerationBanAppeal, error) {
	if !config.BanAppealsOpen() {
		return nil, ErrBanAppealsDisabled
	}

	ban, err := CurrentBan(guildID, userID)
	if err != nil {
		return nil, err
	}
	if ban == nil {
		return nil, ErrNotBanned
	}

	model := &models.ModerationBanAppeal{
		GuildID:   guildID,
		UserID:    userID,
		Source:    int16(source),
		Appeal:    appeal,
		BanReason: ban.Reason,
	}

	err = model.InsertG(ctx, boil.Infer())
	if common.ErrPQIsUniqueViolation(err) {
		return nil, ErrAlreadyAppealed
	}

	return model, err
}

type BanAppealFiledData struct {
	AppealID int64 `json:"appeal_id"`
}

// handleBanAppealFiled posts appeals filed on the dashboard, as only the bot
// can post them.
func handleBanAppealFiled(evt *seventsmodels.ScheduledEvent, data any) (retry bool, err error) {
	ctx := context.Background()
	appealID := data.(*BanAppealFiledData).AppealID

	appeal, err := models.FindModerationBanAppealG(ctx, appealID)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return true, err
	}

	if appeal.ReviewMessageID != 0 || appeal.ClosedAt.Valid {
		return false, nil
	}

	config, err := FetchConfig(appeal.GuildID)
	if err != nil {
		return true, err
	}

	user, err := common.BotSession.User(appeal.UserID)
	if err != nil {
		user = &discordgo.User{ID: appeal.UserID, Username: "unknown"}
	}

	err = postBanAppealForReview(ctx, config, appeal, user)
	return false, err
}

func handleGuildBanRemoveCloseAppeal(evt *eventsystem.EventData) (retry bool, err error) {
	data := evt.GuildBanRemove()
	if data.User == nil {
		return false, nil
	}

	ctx := evt.Context()
	appeal, err := FindOpenBanAppeal(ctx, data.GuildID, data.User.ID)
	if err != nil || appeal == nil {
		return err != nil, err
	}

	appeal.ClosedAt = null.TimeFrom(time.Now())
	if _, err := appeal.UpdateG(ctx, boil.Whitelist("closed_at", "updated_at")); err != nil {
		return true, err
	}

	if appeal.ReviewMessageID != 0 && !appeal.RejectedAt.Valid {
		markBanAppealReviewClosed(appeal)
	}

	return false, nil
}

// DMs are handled on whichever node receives them, where the cached config of
// the server may be stale, so these read the config directly.
func handleBanAppealDMInteraction(evt *eventsystem.EventData) {
	switch evt.InteractionCreate().Type {
	case discordgo.InteractionMessageComponent:
		openBanAppealModal(evt)
	case discordgo.InteractionModalSubmit:
		submitBanAppealModal(evt)
	}
}

func openBanAppealModal(evt *eventsystem.EventData) {
	ic := evt.InteractionCreate()

	guildID, err := strconv.ParseInt(strings.TrimPrefix(ic.MessageComponentData().CustomID, banAppealButtonCustomIDPrefix), 10, 64)
	if err != nil || ic.Message == nil {
		bot.RespondDMInteraction(evt, "Something went wrong opening your appeal.")
		return
	}

	config, err := FetchConfig(guildID)
	if err != nil {
		logger.WithError(err).WithField("guild", guildID).Error("failed fetching config for ban appeal")
		bot.RespondDMInteraction(evt, "Something went wrong opening your appeal.")
		return
	}

	if !banAppealDMButtonAvailable(config, guildID) {
		bot.RespondDMInteraction(evt, banAppealButtonUnavailableMessage(config, guildID))
		return
	}

	open, err := FindOpenBanAppeal(context.Background(), guildID, ic.User.ID)
	if err != nil {
		logger.WithError(err).WithField("guild", guildID).Error("failed looking up open ban appeal")
		bot.RespondDMInteraction(evt, "Something went wrong opening your appeal.")
		return
	}
	if open != nil {
		disableBanAppealButton(evt.Session, ic.ChannelID, ic.Message.ID)
		bot.RespondDMInteraction(evt, ErrAlreadyAppealed.Error())
		return
	}

	err = evt.Session.CreateInteractionResponse(ic.ID, ic.Token, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			Title:    "Appeal Your Ban",
			CustomID: fmt.Sprintf("%s%d_%d", banAppealModalCustomIDPrefix, guildID, ic.Message.ID),
			Components: []discordgo.TopLevelComponent{
				discordgo.TextDisplay{
					Content: "Your appeal will be sent to the moderators of the server that banned you.",
				},
				discordgo.Label{
					Label: "Why should you be unbanned?",
					Component: discordgo.TextInput{
						CustomID:  banAppealModalInputID,
						Style:     discordgo.TextInputParagraph,
						Required:  true,
						MinLength: BanAppealMinLength,
						MaxLength: BanAppealMaxLength,
					},
				},
			},
		},
	})
	if err != nil {
		logger.WithError(err).Error("failed opening ban appeal modal")
	}
}

func submitBanAppealModal(evt *eventsystem.EventData) {
	ic := evt.InteractionCreate()
	data := ic.ModalSubmitData()

	guildID, dmMessageID, err := parseBanAppealModalCustomID(data.CustomID)
	if err != nil {
		bot.RespondDMInteraction(evt, "Something went wrong sending your appeal.")
		return
	}

	reply, ok := bot.DeferDMInteraction(evt)
	if !ok {
		return
	}

	config, err := FetchConfig(guildID)
	if err != nil {
		logger.WithError(err).WithField("guild", guildID).Error("failed fetching config for ban appeal")
		reply("Something went wrong sending your appeal.")
		return
	}

	if !banAppealDMButtonAvailable(config, guildID) {
		reply(banAppealButtonUnavailableMessage(config, guildID))
		return
	}

	ctx := context.Background()
	appeal, err := FileBanAppeal(ctx, config, guildID, ic.User.ID, BanAppealSourceDMButton, banAppealModalText(data))
	switch {
	case err == ErrAlreadyAppealed:
		disableBanAppealButton(evt.Session, ic.ChannelID, dmMessageID)
		reply(err.Error())
		return
	case err == ErrNotBanned || err == ErrBanAppealsDisabled:
		reply(err.Error())
		return
	case err != nil:
		logger.WithError(err).WithField("guild", guildID).Error("failed filing ban appeal")
		reply("Something went wrong sending your appeal.")
		return
	}

	if err := postBanAppealForReview(ctx, config, appeal, ic.User); err != nil {
		logger.WithError(err).WithField("guild", guildID).Error("failed posting ban appeal for review")
		reply("Your appeal could not be delivered to the moderators. Please try again later.")
		return
	}

	disableBanAppealButton(evt.Session, ic.ChannelID, dmMessageID)
	reply("Your appeal has been sent to the server's moderators.")
}

func banAppealButtonUnavailableMessage(config *Config, guildID int64) string {
	if link := banAppealLinkIfOpen(config, guildID); link != "" {
		return "This server no longer accepts appeals through this button. You can appeal here instead: " + link
	}

	return ErrBanAppealsDisabled.Error()
}

func disableBanAppealButton(session *discordgo.Session, channelID, messageID int64) {
	if err := bot.DisableDMButton(session, channelID, messageID, banAppealButtonCustomIDPrefix, "Ban Appealed"); err != nil {
		logger.WithError(err).Error("failed disabling ban appeal button")
	}
}

func parseBanAppealModalCustomID(customID string) (guildID, dmMessageID int64, err error) {
	guildPart, messagePart, ok := strings.Cut(strings.TrimPrefix(customID, banAppealModalCustomIDPrefix), "_")
	if !ok {
		return 0, 0, errors.New("missing message id")
	}

	guildID, err = strconv.ParseInt(guildPart, 10, 64)
	if err != nil {
		return 0, 0, err
	}

	dmMessageID, err = strconv.ParseInt(messagePart, 10, 64)
	return guildID, dmMessageID, err
}

func banAppealModalText(data discordgo.ModalSubmitInteractionData) string {
	for _, component := range data.Components {
		label, ok := component.(*discordgo.Label)
		if !ok {
			continue
		}

		if input, ok := label.Component.(*discordgo.TextInput); ok && input.CustomID == banAppealModalInputID {
			return strings.TrimSpace(input.Value)
		}
	}

	return ""
}

func scheduleBanAppealPost(guildID, appealID int64) error {
	return scheduledevents2.ScheduleEvent(banAppealFiledEvent, guildID, time.Now(), &BanAppealFiledData{AppealID: appealID})
}
