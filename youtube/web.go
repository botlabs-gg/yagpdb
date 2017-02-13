package youtube

import (
	"context"
	"errors"
	"github.com/jinzhu/gorm"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
	"html/template"
	"net/http"
)

type CtxKey int

const (
	CurrentConfig CtxKey = iota
)

type Form struct {
	YoutubeChannelID   string
	YoutubeChannelUser string
	DiscordChannel     string `valid:"channel,false`
	ID                 uint
}

func (p *Plugin) InitWeb() {
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/youtube.html"))

	ytMux := goji.SubMux()
	web.CPMux.Handle(pat.New("/youtube/*"), ytMux)
	web.CPMux.Handle(pat.New("/youtube"), ytMux)

	// Alll handlers here require guild channels present
	ytMux.Use(web.RequireGuildChannelsMiddleware)
	ytMux.Use(web.RequireFullGuildMW)
	ytMux.Use(web.RequireBotMemberMW)
	ytMux.Use(web.RequirePermMW(discordgo.PermissionEmbedLinks))

	mainGetHandler := web.ControllerHandler(HandleYoutube, "cp_youtube")

	ytMux.Handle(pat.Get("/"), mainGetHandler)
	ytMux.Handle(pat.Get(""), mainGetHandler)

	ytMux.Handle(pat.Post(""), web.ControllerPostHandler(p.HandleNew, mainGetHandler, Form{}, "Added a new youtube feed"))
	// ytMux.Handle(pat.Post("/"), web.FormParserMW(web.RenderHandler(HandleNew, "cp_youtube"), Form{}))
	// ytMux.Handle(pat.Post("/:item/update"), web.FormParserMW(web.RenderHandler(HandleModify, "cp_youtube"), Form{}))
	ytMux.Handle(pat.Post("/:item/update"), web.ControllerPostHandler(BaseEditHandler(HandleEdit), mainGetHandler, Form{}, "Updated a youtube feed"))
	ytMux.Handle(pat.Post("/:item/delete"), web.ControllerPostHandler(BaseEditHandler(HandleRemove), mainGetHandler, nil, "Removed a youtube feed"))
}

func HandleYoutube(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	_, ag, templateData := web.GetBaseCPContextData(ctx)

	var subs []*ChannelSubscription
	err := common.SQL.Where("guild_id = ?", ag.ID).Find(&subs).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return templateData, err
	}

	templateData["Subs"] = subs
	templateData["VisibleURL"] = "/cp/" + ag.ID + "/youtube"

	return templateData, nil
}

func (p *Plugin) HandleNew(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)

	// limit it to max 25 feeds
	var count int
	common.SQL.Model(&ChannelSubscription{}).Where("guild_id = ?", activeGuild.ID).Count(&count)

	if count > 24 {
		return templateData.AddAlerts(web.ErrorAlert("Max 25 items allowed")), errors.New("Max limit reached")
	}

	data := ctx.Value(common.ContextKeyParsedForm).(*Form)

	if data.YoutubeChannelID == "" && data.YoutubeChannelUser == "" {
		return templateData.AddAlerts(web.ErrorAlert("Neither channelid or username specified.")), errors.New("ChannelID and username not specified")
	}

	_, err := p.AddFeed(client, activeGuild.ID, data.DiscordChannel, data.YoutubeChannelID, data.YoutubeChannelUser)
	if err != nil {
		if err == ErrNoChannel {
			return templateData.AddAlerts(web.ErrorAlert("No channel by that id found")), errors.New("Channel not found")
		}
		return templateData, err
	}

	return templateData, nil
}

// func HandleModify(w http.ResponseWriter, r *http.Request) interface{} {
// 	ctx := r.Context()
// 	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)

// 	currentConfig := ctx.Value(CurrentConfig).([]*SubredditWatchItem)
// 	templateData["RedditConfig"] = currentConfig

// 	updated := ctx.Value(common.ContextKeyParsedForm).(*Form)
// 	ok := ctx.Value(common.ContextKeyFormOk).(bool)
// 	if !ok {
// 		return templateData
// 	}
// 	updated.Subreddit = strings.TrimSpace(updated.Subreddit)

// 	item := FindWatchItem(currentConfig, updated.ID)
// 	if item == nil {
// 		return templateData.AddAlerts(web.ErrorAlert("Unknown id"))
// 	}

// 	subIsNew := !strings.EqualFold(updated.Subreddit, item.Sub)
// 	item.Channel = updated.Channel

// 	var err error
// 	if !subIsNew {
// 		// Pretty simple then
// 		err = item.Set(client)
// 	} else {
// 		err = item.Remove(client)
// 		if err == nil {
// 			item.Sub = strings.ToLower(r.FormValue("subreddit"))
// 			err = item.Set(client)
// 		}
// 	}

// 	if web.CheckErr(templateData, err, "Failed saving item :'(", log.Error) {
// 		return templateData
// 	}

// 	templateData.AddAlerts(web.SucessAlert("Sucessfully updated reddit feed! :D"))

// 	user := ctx.Value(common.ContextKeyUser).(*discordgo.User)
// 	common.AddCPLogEntry(user, activeGuild.ID, "Modified a feed to /r/"+r.FormValue("subreddit"))
// 	return templateData
// }

type ContextKey int

const (
	ContextKeySub ContextKey = iota
)

func BaseEditHandler(inner web.ControllerHandlerFunc) web.ControllerHandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
		ctx := r.Context()
		_, activeGuild, templateData := web.GetBaseCPContextData(ctx)

		id := pat.Param(r, "item")

		// Get tha actual watch item from the config
		var sub ChannelSubscription
		err := common.SQL.Model(&ChannelSubscription{}).Where("id = ?", id).First(&sub).Error
		if err != nil {
			return templateData.AddAlerts(web.ErrorAlert("Failed retrieving that feed item")), err
		}

		if sub.GuildID != activeGuild.ID {
			return templateData.AddAlerts(web.ErrorAlert("This appears to belong somewhere else...")), nil
		}

		ctx = context.WithValue(ctx, ContextKeySub, &sub)

		return inner(w, r.WithContext(ctx))
	}
}

func HandleEdit(w http.ResponseWriter, r *http.Request) (templateData web.TemplateData, err error) {
	ctx := r.Context()
	_, _, templateData = web.GetBaseCPContextData(ctx)

	sub := ctx.Value(ContextKeySub).(*ChannelSubscription)

	data := ctx.Value(common.ContextKeyParsedForm).(*Form)

	err = common.SQL.Model(sub).Update("channel_id", data.DiscordChannel).Error
	return
}

func HandleRemove(w http.ResponseWriter, r *http.Request) (templateData web.TemplateData, err error) {
	ctx := r.Context()
	_, _, templateData = web.GetBaseCPContextData(ctx)

	sub := ctx.Value(ContextKeySub).(*ChannelSubscription)
	err = common.SQL.Delete(sub).Error
	if err != nil {
		return
	}

	maybeRemoveChannelWatch(sub.YoutubeChannelID)
	return
}
