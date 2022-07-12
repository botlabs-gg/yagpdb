package soundboard

import (
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/cplogs"
	"github.com/botlabs-gg/yagpdb/v2/soundboard/models"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"goji.io"
	"goji.io/pat"
)

//go:embed assets/soundboard.html
var PageHTML string

type PostForm struct {
	ID   int
	Name string `valid:",100"`

	RequiredRoles    []int64 `valid:"role"`
	BlacklistedRoles []int64 `valid:"role"`
}

func (pf *PostForm) ToDBModel() *models.SoundboardSound {
	return &models.SoundboardSound{
		ID: pf.ID,

		Name:             pf.Name,
		RequiredRoles:    pf.RequiredRoles,
		BlacklistedRoles: pf.BlacklistedRoles,
	}
}

var (
	panelLogKeyAddedSound   = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "soundboard_added_sound", FormatString: "Added soundboard sound %s"})
	panelLogKeyUpdatedSound = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "soundboard_updated_sound", FormatString: "Updated soundboard sound %s"})
	panelLogKeyRemovedSound = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "soundboard_removed_sound", FormatString: "Removed soundboard sound %s"})
)

func (p *Plugin) InitWeb() {
	web.AddHTMLTemplate("soundboard/assets/soundboard.html", PageHTML)
	web.AddSidebarItem(web.SidebarCategoryFun, &web.SidebarItem{
		Name: "Soundboard",
		URL:  "soundboard/",
		Icon: "fas fa-border-all",
	})

	cpMux := goji.SubMux()

	web.CPMux.Handle(pat.New("/soundboard/*"), cpMux)
	web.CPMux.Handle(pat.New("/soundboard"), cpMux)

	cpMux.Use(web.RequireBotMemberMW)

	getHandler := web.ControllerHandler(HandleGetCP, "cp_soundboard")

	cpMux.Handle(pat.Get("/"), getHandler)
	//cpMux.Handle(pat.Get(""), getHandler)
	cpMux.Handle(pat.Post("/new"), web.ControllerPostHandler(HandleNew, getHandler, PostForm{}))
	cpMux.Handle(pat.Post("/update"), web.ControllerPostHandler(HandleUpdate, getHandler, PostForm{}))
	cpMux.Handle(pat.Post("/delete"), web.ControllerPostHandler(HandleDelete, getHandler, PostForm{}))
}

func HandleGetCP(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	g, tmpl := web.GetBaseCPContextData(ctx)

	sounds, err := GetSoundboardSounds(g.ID, ctx)
	if err != nil {
		return tmpl, err
	}

	tmpl["SoundboardSounds"] = sounds
	return tmpl, nil
}

func HandleNew(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	g, tmpl := web.GetBaseCPContextData(ctx)

	isDCA := false
	var file multipart.File
	if r.FormValue("SoundURL") == "" {
		f, header, err := r.FormFile("Sound")
		if err != nil {
			return tmpl, err
		}
		file = f

		if strings.HasSuffix(header.Filename, ".dca") {
			isDCA = true
		}
	}

	data := ctx.Value(common.ContextKeyParsedForm).(*PostForm)

	dbModel := data.ToDBModel()
	dbModel.Status = int(TranscodingStatusQueued)
	if isDCA {
		dbModel.Status = int(TranscodingStatusReady)
	}
	dbModel.GuildID = g.ID

	// check for name conflict
	nameConflict, err := models.SoundboardSounds(qm.Where("guild_id=? AND name=?", g.ID, data.Name)).ExistsG(r.Context())
	if err != nil {
		return tmpl, err
	}

	if nameConflict {
		tmpl.AddAlerts(web.ErrorAlert("Name already used"))
		return tmpl, nil
	}

	// check sound limit
	count, err := models.SoundboardSounds(qm.Where("guild_id=?", g.ID)).CountG(r.Context())
	if err != nil {
		return tmpl, err
	}
	if count >= int64(MaxSoundsForContext(ctx)) {
		tmpl.AddAlerts(web.ErrorAlert(fmt.Sprintf("Max %d sounds allowed (%d for premium servers)", MaxGuildSounds, MaxGuildSoundsPremium)))
		return tmpl, nil
	}

	err = dbModel.InsertG(r.Context(), boil.Infer())
	if err != nil {
		return tmpl, err
	}

	// Lock it
	locked, err := common.TryLockRedisKey(KeySoundLock(dbModel.ID), 60)
	if err != nil || !locked {
		if !locked {
			tmpl.AddAlerts(web.ErrorAlert("Uh oh failed locking"))
		}
		return tmpl, err
	}
	defer common.UnlockRedisKey(KeySoundLock(dbModel.ID))

	//logrus.Error("CREAte errror:", err)
	fname := "soundboard/queue/" + strconv.Itoa(int(dbModel.ID))
	if isDCA {
		fname += ".dca"
	}
	destFile, err := os.Create(fname)
	if err != nil {
		return tmpl, err
	}

	tooBig := false
	if file != nil {
		tooBig, err = DownloadNewSoundFile(file, destFile, 10000000)
	} else if r.FormValue("SoundURL") != "" {
		var resp *http.Response
		resp, err = http.Get(r.FormValue("SoundURL"))
		if err != nil {
			tmpl.AddAlerts(web.ErrorAlert("Failed downloading sound: " + err.Error()))
			destFile.Close()
		} else {
			defer resp.Body.Close()
			tooBig, err = DownloadNewSoundFile(resp.Body, destFile, 10000000)
		}
	} else {
		err = errors.New("No sound!?")
	}

	destFile.Close()

	if tooBig || err != nil {
		os.Remove(fname)
		if tooBig {
			tmpl.AddAlerts(web.ErrorAlert("Max 10MB files allowed"))
		}
		dbModel.DeleteG(ctx)
		return tmpl, err
	}

	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyAddedSound, &cplogs.Param{Type: cplogs.ParamTypeString, Value: data.Name}))

	return tmpl, nil
}

func DownloadNewSoundFile(r io.Reader, w io.Writer, limit int) (tooBig bool, err error) {
	soundSize := 0
	for {
		buf := make([]byte, 1024)
		n := 0
		n, err = r.Read(buf)

		if n > 0 {
			buf = buf[:n]
			soundSize += len(buf)
			if soundSize > limit {
				tooBig = true
				break
			}

			_, writeErr := w.Write(buf)
			if writeErr != nil {
				err = writeErr
				break
			}
		}

		if err != nil {
			if err == io.EOF {
				err = nil
			}
			break
		}
	}

	return
}

func HandleUpdate(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	g, tmpl := web.GetBaseCPContextData(ctx)
	data := ctx.Value(common.ContextKeyParsedForm).(*PostForm)

	dbModel, err := models.SoundboardSounds(qm.Where("guild_id = ? AND id = ?", g.ID, data.ID)).OneG(ctx)
	if err != nil {
		return tmpl.AddAlerts(web.ErrorAlert("Error retrieiving sound")), errors.WrapIf(err, "unknown sound")
	}

	nameConflict, err := models.SoundboardSounds(qm.Where("guild_id = ? AND name = ? AND id != ?", g.ID, data.Name, data.ID)).ExistsG(r.Context())
	if err != nil {
		return tmpl, err
	}

	if nameConflict {
		tmpl.AddAlerts(web.ErrorAlert("Name already used"))
		return tmpl, nil
	}

	dbModel.Name = data.Name
	dbModel.RequiredRoles = data.RequiredRoles
	dbModel.BlacklistedRoles = data.BlacklistedRoles

	_, err = dbModel.UpdateG(ctx, boil.Whitelist("name", "required_roles", "blacklisted_roles", "updated_at"))
	if err == nil {
		go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyUpdatedSound, &cplogs.Param{Type: cplogs.ParamTypeString, Value: data.Name}))
	}
	return tmpl, err
}

func HandleDelete(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	g, tmpl := web.GetBaseCPContextData(ctx)
	data := ctx.Value(common.ContextKeyParsedForm).(*PostForm)

	locked, err := common.TryLockRedisKey(KeySoundLock(data.ID), 10)
	if err != nil {
		return tmpl, err
	}
	if !locked {
		tmpl.AddAlerts(web.ErrorAlert("This sound is busy, try again in a minute and if it's still busy contact support"))
		return tmpl, nil
	}
	defer common.UnlockRedisKey(KeySoundLock(data.ID))

	storedSound, err := models.SoundboardSounds(qm.Where("guild_id = ? AND id = ?", g.ID, data.ID)).OneG(ctx)
	if err != nil {
		return tmpl, err
	}

	switch TranscodingStatus(storedSound.Status) {
	case TranscodingStatusQueued, TranscodingStatusReady:
		err = os.Remove(SoundFilePath(data.ID, TranscodingStatus(storedSound.Status)))
	case TranscodingStatusTranscoding:
		tmpl.AddAlerts(web.ErrorAlert("This sound is busy? try again in a minute and if its still busy contact support"))
		return tmpl, nil
	}

	if err != nil {
		if !os.IsNotExist(err) {
			return tmpl, err
		}
	}

	_, err = storedSound.DeleteG(ctx)
	if err == nil {
		go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyRemovedSound, &cplogs.Param{Type: cplogs.ParamTypeString, Value: storedSound.Name}))
	}
	return tmpl, err
}

var _ web.PluginWithServerHomeWidget = (*Plugin)(nil)

func (p *Plugin) LoadServerHomeWidget(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ag, templateData := web.GetBaseCPContextData(r.Context())

	templateData["WidgetTitle"] = "Soundboard"
	templateData["SettingsPath"] = "/soundboard/"

	sounds, err := GetSoundboardSounds(ag.ID, r.Context())
	if err != nil {
		return templateData, err
	}

	if len(sounds) > 0 {
		templateData["WidgetEnabled"] = true
	} else {
		templateData["WidgetDisabled"] = true
	}

	const format = `<p>Soundboard sounds: <code>%d</code></p>`
	templateData["WidgetBody"] = template.HTML(fmt.Sprintf(format, len(sounds)))

	return templateData, nil
}
