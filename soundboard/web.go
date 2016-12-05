package soundboard

import (
	"errors"
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/configstore"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
	"golang.org/x/net/context"
	"html/template"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func (p *Plugin) InitWeb() {
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/soundboard.html"))

	cpMux := goji.SubMux()

	web.CPMux.HandleC(pat.New("/soundboard/*"), cpMux)
	web.CPMux.HandleC(pat.New("/soundboard"), cpMux)

	cpMux.UseC(web.RequireFullGuildMW)
	cpMux.UseC(web.RequireBotMemberMW)

	getHandler := web.ControllerHandler(HandleGetCP, "cp_soundboard")

	cpMux.HandleC(pat.Get("/"), getHandler)
	//cpMux.HandleC(pat.Get(""), getHandler)
	cpMux.HandleC(pat.Post("/new"), web.ControllerPostHandler(HandleNew, getHandler, SoundboardSound{}, "Added a new sound to the soundboard"))
	cpMux.HandleC(pat.Post("/update"), web.ControllerPostHandler(HandleUpdate, getHandler, SoundboardSound{}, "Updated a soundboard sound"))
	cpMux.HandleC(pat.Post("/delete"), web.ControllerPostHandler(HandleDelete, getHandler, SoundboardSound{}, "Removed a sound from the soundboard"))
}

func HandleGetCP(ctx context.Context, w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	_, g, tmpl := web.GetBaseCPContextData(ctx)

	var config SoundboardConfig
	err := configstore.Cached.GetGuildConfig(ctx, g.ID, &config)

	if err != nil {
		return tmpl, err
	}
	tmpl["Config"] = config
	return tmpl, nil
}

func HandleNew(ctx context.Context, w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	client, g, tmpl := web.GetBaseCPContextData(ctx)

	var file multipart.File
	if r.FormValue("SoundURL") == "" {
		f, header, err := r.FormFile("Sound")
		if err != nil {
			return tmpl, err
		}
		file = f

		if !strings.HasSuffix(header.Filename, ".mp3") && !strings.HasSuffix(header.Filename, ".ogg") && !strings.HasSuffix(header.Filename, ".wav") {
			tmpl.AddAlerts(web.ErrorAlert("Only mp3, ogg and wav files allowed"))
			return tmpl, nil
		}
	}

	data := ctx.Value(common.ContextKeyParsedForm).(*SoundboardSound)
	data.Status = TranscodingStatusQueued
	data.GuildID = common.MustParseInt(g.ID)

	count := 0
	err := common.SQL.Model(SoundboardSound{}).Where("guild_id = ? AND name = ?", g.ID, data.Name).Count(&count).Error
	if err != nil {
		return tmpl, err
	}

	if count > 0 {
		tmpl.AddAlerts(web.ErrorAlert("Name already used"))
		return tmpl, nil
	}

	err = common.SQL.Model(SoundboardSound{}).Where("guild_id = ?", g.ID).Count(&count).Error
	if err != nil {
		return tmpl, err
	}
	if count > 14 {
		tmpl.AddAlerts(web.ErrorAlert("You can have a maximum amount of 15 sounds"))
		return tmpl, nil
	}

	err = common.SQL.Create(data).Error
	if err != nil {
		return tmpl, err
	}

	// Lock it
	locked, err := common.TryLockRedisKey(client, KeySoundLock(data.ID), 60)
	if err != nil || !locked {
		if !locked {
			tmpl.AddAlerts(web.ErrorAlert("Uh oh failed locking"))
		}
		return tmpl, err
	}
	defer common.UnlockRedisKey(client, KeySoundLock(data.ID))

	//logrus.Error("CREAte errror:", err)
	fname := "soundboard/queue/" + strconv.Itoa(int(data.ID))
	destFile, err := os.Create(fname)
	if err != nil {
		return tmpl, err
	}

	tooBig := false
	if file != nil {
		logrus.Info("Download sound file from upload")
		tooBig, err = DownloadNewSondFile(file, destFile, 10000000)
	} else if r.FormValue("SoundURL") != "" {
		logrus.Info("Download sound file from url")
		var resp *http.Response
		resp, err = http.Get(r.FormValue("SoundURL"))
		if err != nil {
			tmpl.AddAlerts(web.ErrorAlert("Failed downloading sound: " + err.Error()))
			destFile.Close()
		} else {
			defer resp.Body.Close()
			tooBig, err = DownloadNewSondFile(resp.Body, destFile, 10000000)
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
		common.SQL.Delete(data)
		return tmpl, err
	}

	configstore.InvalidateGuildCache(client, g.ID, &SoundboardConfig{})
	return tmpl, err
}

func DownloadNewSondFile(r io.Reader, w io.Writer, limit int) (tooBig bool, err error) {
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

func HandleUpdate(ctx context.Context, w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	client, g, tmpl := web.GetBaseCPContextData(ctx)
	data := ctx.Value(common.ContextKeyParsedForm).(*SoundboardSound)
	data.GuildID = common.MustParseInt(g.ID)

	count := 0
	common.SQL.Model(SoundboardSound{}).Where("guild_id = ? AND name = ? AND id != ?", g.ID, data.Name, data.ID).Count(&count)
	if count > 0 {
		tmpl.AddAlerts(web.ErrorAlert("Name already used"))
		return tmpl, nil
	}

	err := common.SQL.Debug().Model(data).Updates(map[string]interface{}{"name": data.Name, "required_role": data.RequiredRole}).Error
	configstore.InvalidateGuildCache(client, g.ID, &SoundboardConfig{})
	return tmpl, err
}

func HandleDelete(ctx context.Context, w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	client, g, tmpl := web.GetBaseCPContextData(ctx)
	data := ctx.Value(common.ContextKeyParsedForm).(*SoundboardSound)
	data.GuildID = common.MustParseInt(g.ID)

	locked, err := common.TryLockRedisKey(client, KeySoundLock(data.ID), 10)
	if err != nil {
		return tmpl, err
	}
	if !locked {
		tmpl.AddAlerts(web.ErrorAlert("This sound is busy, try again in a minute and if it's still busy contact support"))
		return tmpl, nil
	}
	defer common.UnlockRedisKey(client, KeySoundLock(data.ID))

	var storedSound SoundboardSound
	err = common.SQL.Where("guild_id = ? AND id = ?", g.ID, data.ID).First(&storedSound).Error
	if err != nil {
		return tmpl, nil
	}

	switch storedSound.Status {
	case TranscodingStatusQueued, TranscodingStatusReady:
		err = os.Remove(SoundFilePath(data.ID, storedSound.Status))
	case TranscodingStatusTranscoding:
		tmpl.AddAlerts(web.ErrorAlert("This sound is busy? try again in a minute and if its still busy contact support"))
		return tmpl, nil
	}

	if err != nil {
		if !os.IsNotExist(err) {
			return tmpl, err
		}
	}

	err = common.SQL.Delete(storedSound).Error
	configstore.InvalidateGuildCache(client, g.ID, &SoundboardConfig{})

	return tmpl, err
}
