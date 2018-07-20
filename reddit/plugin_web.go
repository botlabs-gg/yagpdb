package reddit

import (
	"context"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

type CtxKey int

const (
	CurrentConfig CtxKey = iota
)

type Form struct {
	Subreddit string `schema:"subreddit" valid:",1,100"`
	Channel   string `schema:"channel" valid:"channel,false`
	ID        int    `schema:"id"`
	UseEmbeds bool   `schema:"use_embeds"`
}

func (p *Plugin) InitWeb() {
	tmplPathSettings := "templates/plugins/reddit.html"
	if common.Testing {
		tmplPathSettings = "../../reddit/assets/reddit.html"
	}

	web.Templates = template.Must(web.Templates.ParseFiles(tmplPathSettings))

	redditMux := goji.SubMux()
	web.CPMux.Handle(pat.New("/reddit/*"), redditMux)
	web.CPMux.Handle(pat.New("/reddit"), redditMux)

	// Alll handlers here require guild channels present
	redditMux.Use(web.RequireGuildChannelsMiddleware)
	redditMux.Use(web.RequireFullGuildMW)
	redditMux.Use(web.RequireBotMemberMW)
	redditMux.Use(web.RequirePermMW(discordgo.PermissionEmbedLinks))
	redditMux.Use(baseData)

	redditMux.Handle(pat.Get("/"), web.RenderHandler(HandleReddit, "cp_reddit"))
	redditMux.Handle(pat.Get(""), web.RenderHandler(HandleReddit, "cp_reddit"))

	// If only html forms allowed patch and delete.. if only
	redditMux.Handle(pat.Post(""), web.FormParserMW(web.RenderHandler(HandleNew, "cp_reddit"), Form{}))
	redditMux.Handle(pat.Post("/"), web.FormParserMW(web.RenderHandler(HandleNew, "cp_reddit"), Form{}))
	redditMux.Handle(pat.Post("/:item/update"), web.FormParserMW(web.RenderHandler(HandleModify, "cp_reddit"), Form{}))
	redditMux.Handle(pat.Post("/:item/delete"), web.FormParserMW(web.RenderHandler(HandleRemove, "cp_reddit"), Form{}))
}

// Adds the current config to the context
func baseData(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		activeGuild, templateData := web.GetBaseCPContextData(ctx)
		templateData["VisibleURL"] = "/manage/" + discordgo.StrID(activeGuild.ID) + "/reddit/"

		currentConfig, err := GetConfig("guild_subreddit_watch:" + discordgo.StrID(activeGuild.ID))
		if web.CheckErr(templateData, err, "Failed retrieving config, message support in the yagpdb server", web.CtxLogger(ctx).Error) {
			web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_reddit", templateData))
		} else {
			sort.Slice(currentConfig, func(i, j int) bool {
				return currentConfig[i].Sub < currentConfig[j].Sub
			})
		}

		inner.ServeHTTP(w, r.WithContext(context.WithValue(ctx, CurrentConfig, currentConfig)))
	}

	return http.HandlerFunc(mw)
}

func HandleReddit(w http.ResponseWriter, r *http.Request) interface{} {
	ctx := r.Context()
	_, templateData := web.GetBaseCPContextData(ctx)

	currentConfig := ctx.Value(CurrentConfig).([]*SubredditWatchItem)
	templateData["RedditConfig"] = currentConfig

	return templateData
}

func HandleNew(w http.ResponseWriter, r *http.Request) interface{} {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)

	currentConfig := ctx.Value(CurrentConfig).([]*SubredditWatchItem)

	templateData["RedditConfig"] = currentConfig

	newElem := ctx.Value(common.ContextKeyParsedForm).(*Form)
	ok := ctx.Value(common.ContextKeyFormOk).(bool)
	if !ok {
		return templateData
	}

	// get an id the ez (and not safe if 2 people create at same time) way
	highest := 0
	for _, v := range currentConfig {
		if v.ID > highest {
			highest = v.ID
		}
	}

	if len(currentConfig) >= GuildMaxFeeds {
		return templateData.AddAlerts(web.ErrorAlert("Max " + strconv.Itoa(GuildMaxFeeds) + " items allowed"))
	}

	watchItem := &SubredditWatchItem{
		Sub:       strings.TrimSpace(newElem.Subreddit),
		Channel:   newElem.Channel,
		Guild:     discordgo.StrID(activeGuild.ID),
		ID:        highest + 1,
		UseEmbeds: newElem.UseEmbeds,
	}

	err := watchItem.Set()
	if web.CheckErr(templateData, err, "Failed saving item :'(", web.CtxLogger(ctx).Error) {
		return templateData
	}

	currentConfig = append(currentConfig, watchItem)
	sort.Slice(currentConfig, func(i, j int) bool {
		return currentConfig[i].Sub < currentConfig[j].Sub
	})

	templateData["RedditConfig"] = currentConfig
	templateData.AddAlerts(web.SucessAlert("Sucessfully added subreddit feed for /r/" + watchItem.Sub))

	// Log
	user := ctx.Value(common.ContextKeyUser).(*discordgo.User)
	go common.AddCPLogEntry(user, activeGuild.ID, "Added reddit feed from /r/"+newElem.Subreddit)
	return templateData
}

func HandleModify(w http.ResponseWriter, r *http.Request) interface{} {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)

	currentConfig := ctx.Value(CurrentConfig).([]*SubredditWatchItem)
	templateData["RedditConfig"] = currentConfig

	updated := ctx.Value(common.ContextKeyParsedForm).(*Form)
	ok := ctx.Value(common.ContextKeyFormOk).(bool)
	if !ok {
		return templateData
	}
	updated.Subreddit = strings.TrimSpace(updated.Subreddit)

	item := FindWatchItem(currentConfig, updated.ID)
	if item == nil {
		return templateData.AddAlerts(web.ErrorAlert("Unknown id"))
	}

	subIsNew := !strings.EqualFold(updated.Subreddit, item.Sub)
	item.Channel = updated.Channel
	item.UseEmbeds = updated.UseEmbeds

	var err error
	if !subIsNew {
		// Pretty simple then
		err = item.Set()
	} else {
		err = item.Remove()
		if err == nil {
			item.Sub = strings.ToLower(r.FormValue("subreddit"))
			err = item.Set()
		}
	}

	if web.CheckErr(templateData, err, "Failed saving item :'(", web.CtxLogger(ctx).Error) {
		return templateData
	}

	templateData.AddAlerts(web.SucessAlert("Sucessfully updated reddit feed! :D"))

	user := ctx.Value(common.ContextKeyUser).(*discordgo.User)
	common.AddCPLogEntry(user, activeGuild.ID, "Modified a feed to /r/"+r.FormValue("subreddit"))
	return templateData
}

func HandleRemove(w http.ResponseWriter, r *http.Request) interface{} {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)

	currentConfig := ctx.Value(CurrentConfig).([]*SubredditWatchItem)
	templateData["RedditConfig"] = currentConfig

	id := pat.Param(r, "item")
	idInt, err := strconv.ParseInt(id, 10, 32)
	if err != nil {
		return templateData.AddAlerts(web.ErrorAlert("Failed parsing id", err))
	}

	// Get tha actual watch item from the config
	item := FindWatchItem(currentConfig, int(idInt))

	if item == nil {
		return templateData.AddAlerts(web.ErrorAlert("Unknown id"))
	}

	err = item.Remove()
	if web.CheckErr(templateData, err, "Failed removing item :'(", web.CtxLogger(ctx).Error) {
		return templateData
	}

	templateData.AddAlerts(web.SucessAlert("Sucessfully removed subreddit feed for /r/" + item.Sub))

	// Remove it form the displayed list
	for k, c := range currentConfig {
		if c.ID == int(idInt) {
			currentConfig = append(currentConfig[:k], currentConfig[k+1:]...)
		}
	}

	templateData["RedditConfig"] = currentConfig

	user := ctx.Value(common.ContextKeyUser).(*discordgo.User)
	go common.AddCPLogEntry(user, activeGuild.ID, "Removed feed from /r/"+item.Sub)
	return templateData
}
