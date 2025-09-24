package personalizer

import (
	"bytes"
	"context"
	"database/sql"
	_ "embed"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/bot/botrest"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/cplogs"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/personalizer/models"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"goji.io"
	"goji.io/pat"
)

var (
	panelLogKeyPersonalizerUpdate = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "personalizer_updated", FormatString: "Updated bot profile"})
	panelLogKeyPersonalizerReset  = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "personalizer_reset", FormatString: "Reset bot profile"})
)

//go:embed assets/personalizer.html
var PageHTML string

func (p *Plugin) InitWeb() {
	web.AddHTMLTemplate("personalizer/assets/personalizer.html", PageHTML)
	mux := goji.SubMux()
	// Register routes under control panel mux
	gethandler := web.ControllerHandler(handleGetPage, "cp_personalizer")
	postHandler := web.ControllerPostHandler(handleUpdateProfile, gethandler, postForm{})
	deleteHandler := web.ControllerPostHandler(handleResetProfile, gethandler, nil)
	web.CPMux.Handle(pat.New("/personalizer/*"), mux)
	web.CPMux.Handle(pat.New("/personalizer"), mux)
	mux.Use(web.RequireBotMemberMW)
	mux.Use(web.RequirePermMW(discordgo.PermissionChangeNickname))
	mux.Use(premium.PremiumGuildMW)
	mux.Handle(pat.Get("/"), gethandler)
	mux.Handle(pat.Get(""), gethandler)
	mux.Handle(pat.Post("/"), postHandler)
	mux.Handle(pat.Post(""), postHandler)

	mux.Handle(pat.Post("/reset"), deleteHandler)
	mux.Handle(pat.Post("/reset/"), deleteHandler)
}

type postForm struct {
	Nick string
}

func handleGetPage(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	guild, tmpl := web.GetBaseCPContextData(r.Context())

	botMember, err := botrest.GetBotMember(guild.ID)
	if err != nil {
		return tmpl, err
	}

	nick := botMember.Nick
	if nick == "" {
		nick = botMember.User.Username
	}
	avatarURL := botMember.AvatarURL("256")
	bannerURL := botMember.BannerURL("512")

	if bannerURL == "" {
		if pg, err := models.FindPersonalizedGuildG(context.Background(), guild.ID); err == nil && pg != nil && pg.Banner.Valid && pg.Banner.String != "" {
			bannerURL = pg.Banner.String
		}
	}

	if bannerURL != "" {
		ext := "png"
		if strings.HasPrefix(bannerURL, "a_") {
			ext = "gif"
		}
		bannerURL = fmt.Sprintf("https://cdn.discordapp.com/guilds/%d/users/%d/banners/%s.%s?size=512", guild.ID, botMember.User.ID, bannerURL, ext)
	}

	current := map[string]string{
		"nick":       nick,
		"avatar_url": avatarURL,
		"banner_url": bannerURL,
	}

	tmpl["CurrentMember"] = current
	return tmpl, nil
}

const maxUploadBytes = 8 * 1024 * 1024

func readImageToDataURI(fh *multipart.FileHeader) (string, error) {
	if fh == nil || fh.Size == 0 {
		return "", fmt.Errorf("file is empty")
	}
	if fh.Size > maxUploadBytes {
		return "", fmt.Errorf("file too large")
	}

	file, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()

	raw, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	var contentType string
	if len(raw) >= 512 {
		contentType = http.DetectContentType(raw[:512])
	} else {
		contentType = http.DetectContentType(raw)
	}
	switch contentType {
	case "image/png", "image/jpeg", "image/gif":
		// ok
	default:
		return "", fmt.Errorf("invalid image data")
	}

	// Validate dimensions (decodes only first frame for GIF, which is fine for checks)
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("failed to decode image")
	}
	w := img.Bounds().Dx()
	h := img.Bounds().Dy()
	if w < 64 || h < 64 || w > 4096 || h > 4096 {
		return "", fmt.Errorf("invalid dimensions")
	}

	prefix := "data:image/png;base64,"
	switch contentType {
	case "image/jpeg":
		prefix = "data:image/jpeg;base64,"
	case "image/gif":
		prefix = "data:image/gif;base64,"
	}
	return prefix + base64.StdEncoding.EncodeToString(raw), nil
}

func handleResetProfile(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	guild, tmpl := web.GetBaseCPContextData(r.Context())

	if !premium.ContextPremium(r.Context()) {
		tmpl.AddAlerts(web.ErrorAlert("This feature requires Premium"))
		return tmpl, nil
	}
	pg, err := models.FindPersonalizedGuildG(context.Background(), guild.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			pg = nil
		} else {
			logrus.WithError(err).Error("Failed to reset bot profile")
			return tmpl.AddAlerts(web.ErrorAlert("Failed to reset bot profile, please try again later.")), nil
		}
	}

	if pg != nil {
		_, _ = pg.DeleteG(context.Background())
	}

	err = common.BotSession.GuildMemberMeReset(guild.ID, true, true, false)
	if err != nil {
		logrus.WithError(err).Error("Failed to reset bot profile")
		return tmpl.AddAlerts(web.ErrorAlert("Failed to reset bot profile, please try again later.")), nil
	}

	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyPersonalizerReset))
	return tmpl, nil
}

func handleUpdateProfile(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	guild, tmpl := web.GetBaseCPContextData(r.Context())

	if !premium.ContextPremium(r.Context()) {
		tmpl.AddAlerts(web.ErrorAlert("This feature requires Premium"))
		return tmpl, nil
	}

	// Parse multipart form for uploads
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil && err != http.ErrNotMultipart {
		return tmpl.AddAlerts(web.ErrorAlert("Invalid form: ", err.Error())), nil
	}

	form := r.Context().Value(common.ContextKeyParsedForm).(*postForm)
	nick := strings.TrimSpace(form.Nick)

	var avatarDataURI, bannerDataURI string
	avatarFile, _ := func() (*multipart.FileHeader, error) { _, fh, err := r.FormFile("AvatarFile"); return fh, err }()
	bannerFile, _ := func() (*multipart.FileHeader, error) { _, fh, err := r.FormFile("BannerFile"); return fh, err }()
	logrus.Infof("handlePostUpdate: avatarFile: %#v, bannerFile: %#v", avatarFile, bannerFile)

	if avatarFile != nil && avatarFile.Size > 0 {
		if s, err := readImageToDataURI(avatarFile); err == nil {
			avatarDataURI = s
		} else {
			return tmpl.AddAlerts(web.ErrorAlert("Avatar error: ", err.Error())), nil
		}
	}
	if bannerFile != nil && bannerFile.Size > 0 {
		if s, err := readImageToDataURI(bannerFile); err == nil {
			bannerDataURI = s
		} else {
			return tmpl.AddAlerts(web.ErrorAlert("Banner error: ", err.Error())), nil
		}
	}

	update := &discordgo.CurrentGuildMemberUpdate{}
	if nick != "" {
		update.Nick = nick
	}
	if avatarDataURI != "" {
		update.Avatar = avatarDataURI
	}
	if bannerDataURI != "" {
		update.Banner = bannerDataURI
	}

	member, err := common.BotSession.GuildMemberMe(guild.ID, *update)
	if err != nil {
		logrus.WithError(err).Error("Failed to update bot profile")
		return tmpl.AddAlerts(web.ErrorAlert("Failed to update bot profile, please try again later.")), nil
	}

	pg, err := models.FindPersonalizedGuildG(context.Background(), guild.ID)
	if err != nil && err != sql.ErrNoRows {
		logrus.WithError(err).Error("Failed to update bot profile")
		return tmpl.AddAlerts(web.ErrorAlert("Failed to update bot profile, please try again later.")), nil
	}

	if pg == nil {
		pg = &models.PersonalizedGuild{GuildID: guild.ID}
	}
	if member.Nick != "" {
		pg.Nick = null.NewString(member.Nick, true)
	}
	if member.Avatar != "" {
		pg.Avatar = null.NewString(member.Avatar, true)
	}
	if member.Banner != "" {
		pg.Banner = null.NewString(member.Banner, true)
	}

	if err := pg.UpsertG(context.Background(), true, nil, boil.Whitelist(models.PersonalizedGuildColumns.Nick, models.PersonalizedGuildColumns.Avatar, models.PersonalizedGuildColumns.Banner), boil.Infer()); err != nil {
		return tmpl.AddAlerts(web.ErrorAlert("Failed to update bot profile, please try again later.")), nil
	}

	if err == nil {
		go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyPersonalizerUpdate))
	}

	return tmpl, nil
}
