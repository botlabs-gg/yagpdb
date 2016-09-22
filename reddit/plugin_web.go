package reddit

import (
	"fmt"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
	"golang.org/x/net/context"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type CtxKey int

const (
	CurrentConfig CtxKey = iota
)

func (p *Plugin) InitWeb() {
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/reddit.html"))

	redditMux := goji.SubMux()
	web.CPMux.HandleC(pat.New("/reddit/*"), redditMux)
	web.CPMux.HandleC(pat.New("/reddit"), redditMux)

	// Alll handlers here require guild channels present
	redditMux.UseC(web.RequireGuildChannelsMiddleware)
	redditMux.UseC(baseData)

	redditMux.HandleC(pat.Get("/"), web.RenderHandler(HandleReddit, "cp_reddit"))
	redditMux.HandleC(pat.Get(""), web.RenderHandler(HandleReddit, "cp_reddit"))

	// If only html forms allowed patch and delete.. if only
	redditMux.HandleC(pat.Post(""), web.RenderHandler(HandleNew, "cp_reddit"))
	redditMux.HandleC(pat.Post("/:item/update"), web.RenderHandler(HandleModify, "cp_reddit"))
	redditMux.HandleC(pat.Post("/:item/delete"), web.RenderHandler(HandleRemove, "cp_reddit"))
}

// Adds the current config to the context
func baseData(inner goji.Handler) goji.Handler {
	mw := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		client, activeGuild, templateData := web.GetBaseCPContextData(ctx)

		currentConfig, err := GetConfig(client, "guild_subreddit_watch:"+activeGuild.ID)
		if err != nil {
			templateData.AddAlerts(web.ErrorAlert("Failed retrieving current config, conact support", err))
			log.Println("Failed retrieving config", activeGuild.ID, err)
			web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_reddit", templateData))
			return
		}
		inner.ServeHTTPC(context.WithValue(ctx, CurrentConfig, currentConfig), w, r)

	}

	return goji.HandlerFunc(mw)
}

func HandleReddit(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	_, _, templateData := web.GetBaseCPContextData(ctx)

	currentConfig := ctx.Value(CurrentConfig).([]*SubredditWatchItem)
	templateData["RedditConfig"] = currentConfig

	return templateData
}

func HandleNew(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)

	currentConfig := ctx.Value(CurrentConfig).([]*SubredditWatchItem)
	templateData["RedditConfig"] = currentConfig

	highest := 0
	for _, v := range currentConfig {
		if v.ID > highest {
			highest = v.ID
		}
	}

	if len(currentConfig) > 24 {
		return templateData.AddAlerts(web.ErrorAlert("Max 25 items allowed"))
	}

	channelId, ok := GetChannel(r.FormValue("channel"), activeGuild.ID, templateData, ctx)
	if !ok {
		return templateData.AddAlerts(web.ErrorAlert("Unknown channel"))
	}

	watchItem := &SubredditWatchItem{
		Sub:     strings.ToLower(r.FormValue("subreddit")),
		Channel: channelId,
		Guild:   activeGuild.ID,
		ID:      highest + 1,
	}

	err := watchItem.Set(client)
	if web.CheckErr(templateData, err, "Failed saving item :'(") {
		return templateData
	}

	currentConfig = append(currentConfig, watchItem)
	templateData["RedditConfig"] = currentConfig
	templateData.AddAlerts(web.SucessAlert("Sucessfully added subreddit feed for /r/" + watchItem.Sub))

	// Log
	user := ctx.Value(web.ContextKeyUser).(*discordgo.User)
	common.AddCPLogEntry(client, activeGuild.ID, fmt.Sprintf("%s(%s) Added feed from /r/%s", user.Username, user.ID, r.FormValue("subreddit")))
	return templateData
}

func HandleModify(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)

	currentConfig := ctx.Value(CurrentConfig).([]*SubredditWatchItem)
	templateData["RedditConfig"] = currentConfig

	id := pat.Param(ctx, "item")
	idInt, err := strconv.ParseInt(id, 10, 32)
	if err != nil {
		return templateData.AddAlerts(web.ErrorAlert("Failed parsing id", err))
	}

	item := FindWatchItem(currentConfig, int(idInt))
	if item == nil {
		return templateData.AddAlerts(web.ErrorAlert("Unknown id"))
	}

	r.ParseForm()
	channel, ok := GetChannel(r.FormValue("channel"), activeGuild.ID, templateData, ctx)
	if !ok {
		return templateData.AddAlerts(web.ErrorAlert("Failed retrieving channel"))
	}

	newSub := r.FormValue("subreddit") != item.Sub

	item.Channel = channel

	if !newSub {
		// Pretty simple then
		err = item.Set(client)
	} else {
		err = item.Remove(client)
		if err == nil {
			item.Sub = strings.ToLower(r.FormValue("subreddit"))
			err = item.Set(client)
		}
	}

	if web.CheckErr(templateData, err, "Failed saving item :'(") {
		return templateData
	}

	templateData.AddAlerts(web.SucessAlert("Sucessfully updated reddit feed! :D"))

	user := ctx.Value(web.ContextKeyUser).(*discordgo.User)
	common.AddCPLogEntry(client, activeGuild.ID, fmt.Sprintf("%s(%s) Modified a feed to /r/%s", user.Username, user.ID, r.FormValue("subreddit")))
	return templateData
}

func HandleRemove(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)

	currentConfig := ctx.Value(CurrentConfig).([]*SubredditWatchItem)
	templateData["RedditConfig"] = currentConfig

	id := pat.Param(ctx, "item")
	idInt, err := strconv.ParseInt(id, 10, 32)
	if err != nil {
		return templateData.AddAlerts(web.ErrorAlert("Failed parsing id", err))
	}

	// Get tha actual watch item from the config
	item := FindWatchItem(currentConfig, int(idInt))

	if item == nil {
		return templateData.AddAlerts(web.ErrorAlert("Unknown id"))
	}

	err = item.Remove(client)
	if web.CheckErr(templateData, err, "Failed removing item :'(") {
		return templateData
	}

	templateData.AddAlerts(web.SucessAlert("Sucessfully removed subreddit feed for /r/ :')", item.Sub))

	// Remove it form the displayed list
	for k, c := range currentConfig {
		if c.ID == int(idInt) {
			currentConfig = append(currentConfig[:k], currentConfig[k+1:]...)
		}
	}

	templateData["RedditConfig"] = currentConfig

	user := ctx.Value(web.ContextKeyUser).(*discordgo.User)
	common.AddCPLogEntry(client, activeGuild.ID, fmt.Sprintf("%s(%s) Removed feed from /r/%s", user.Username, user.ID, r.FormValue("subreddit")))
	return templateData
}

// Validates a channel name or id, adds an error message if not found
// Returns true if everythign went okay
func GetChannel(name, guild string, templateData web.TemplateData, ctx context.Context) (string, bool) {
	if name == "" {
		return guild, true
	}

	for _, v := range ctx.Value(web.ContextKeyGuildChannels).([]*discordgo.Channel) {
		if v.ID == name {
			return v.ID, true
		}
	}

	templateData.AddAlerts(web.ErrorAlert("Failed finding channel"))
	log.Println("Failed finding channel", guild)
	return "", false
}
