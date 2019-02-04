package reddit

import (
	"context"
	"fmt"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/reddit/models"
	"github.com/jonas747/yagpdb/web"
	"github.com/volatiletech/sqlboiler/boil"
	"goji.io"
	"goji.io/pat"
	"html/template"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
)

type CtxKey int

const (
	CurrentConfig CtxKey = iota
)

type CreateForm struct {
	Subreddit string `schema:"subreddit" valid:",1,100"`
	Channel   int64  `schema:"channel" valid:"channel,false`
	ID        int64  `schema:"id"`
	UseEmbeds bool   `schema:"use_embeds"`
}

type UpdateForm struct {
	Channel   int64 `schema:"channel" valid:"channel,false`
	ID        int64 `schema:"id"`
	UseEmbeds bool  `schema:"use_embeds"`
}

func (p *Plugin) InitWeb() {
	if os.Getenv("YAGPDB_REDDIT_DISABLE_REDIS_PQ_MIGRATION") == "" {
		go migrateLegacyRedisFormatToPostgres()
	}

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
	redditMux.Handle(pat.Post(""), web.FormParserMW(web.RenderHandler(HandleNew, "cp_reddit"), CreateForm{}))
	redditMux.Handle(pat.Post("/"), web.FormParserMW(web.RenderHandler(HandleNew, "cp_reddit"), CreateForm{}))
	redditMux.Handle(pat.Post("/:item/update"), web.FormParserMW(web.RenderHandler(HandleModify, "cp_reddit"), UpdateForm{}))
	redditMux.Handle(pat.Post("/:item/delete"), web.RenderHandler(HandleRemove, "cp_reddit"))
}

// Adds the current config to the context
func baseData(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		activeGuild, templateData := web.GetBaseCPContextData(ctx)
		templateData["VisibleURL"] = "/manage/" + discordgo.StrID(activeGuild.ID) + "/reddit/"

		feeds, err := models.RedditFeeds(models.RedditFeedWhere.GuildID.EQ(activeGuild.ID)).AllG(ctx)
		if web.CheckErr(templateData, err, "Failed retrieving config, message support in the yagpdb server", web.CtxLogger(ctx).Error) {
			web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_reddit", templateData))
		} else {
			sort.Slice(feeds, func(i, j int) bool {
				return feeds[i].Subreddit < feeds[j].Subreddit
			})
		}

		inner.ServeHTTP(w, r.WithContext(context.WithValue(ctx, CurrentConfig, feeds)))
	}

	return http.HandlerFunc(mw)
}

func HandleReddit(w http.ResponseWriter, r *http.Request) interface{} {
	ctx := r.Context()
	_, templateData := web.GetBaseCPContextData(ctx)

	currentConfig := ctx.Value(CurrentConfig).(models.RedditFeedSlice)
	templateData["RedditConfig"] = currentConfig

	return templateData
}

func HandleNew(w http.ResponseWriter, r *http.Request) interface{} {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)

	currentConfig := ctx.Value(CurrentConfig).(models.RedditFeedSlice)

	templateData["RedditConfig"] = currentConfig

	newElem := ctx.Value(common.ContextKeyParsedForm).(*CreateForm)
	ok := ctx.Value(common.ContextKeyFormOk).(bool)
	if !ok {
		return templateData
	}

	maxFeeds := MaxFeedForCtx(ctx)
	if len(currentConfig) >= maxFeeds {
		return templateData.AddAlerts(web.ErrorAlert(fmt.Sprintf("Max %d feeds allowed (or %d for premium servers)", GuildMaxFeedsNormal, GuildMaxFeedsPremium)))
	}

	watchItem := &models.RedditFeed{
		GuildID:   activeGuild.ID,
		ChannelID: newElem.Channel,
		Subreddit: strings.ToLower(strings.TrimSpace(newElem.Subreddit)),
		UseEmbeds: newElem.UseEmbeds,
	}

	err := watchItem.InsertG(ctx, boil.Infer())
	if web.CheckErr(templateData, err, "Failed saving item :'(", web.CtxLogger(ctx).Error) {
		return templateData
	}

	currentConfig = append(currentConfig, watchItem)
	sort.Slice(currentConfig, func(i, j int) bool {
		return currentConfig[i].Subreddit < currentConfig[j].Subreddit
	})

	templateData["RedditConfig"] = currentConfig
	templateData.AddAlerts(web.SucessAlert("Sucessfully added subreddit feed for /r/" + watchItem.Subreddit))

	// Log
	user := ctx.Value(common.ContextKeyUser).(*discordgo.User)
	go common.AddCPLogEntry(user, activeGuild.ID, "Added reddit feed from /r/"+newElem.Subreddit)
	return templateData
}

func HandleModify(w http.ResponseWriter, r *http.Request) interface{} {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)

	currentConfig := ctx.Value(CurrentConfig).(models.RedditFeedSlice)
	templateData["RedditConfig"] = currentConfig

	updated := ctx.Value(common.ContextKeyParsedForm).(*UpdateForm)
	ok := ctx.Value(common.ContextKeyFormOk).(bool)
	if !ok {
		return templateData
	}

	item := FindFeed(currentConfig, updated.ID)
	if item == nil {
		return templateData.AddAlerts(web.ErrorAlert("Unknown id"))
	}

	item.ChannelID = updated.Channel
	item.UseEmbeds = updated.UseEmbeds

	_, err := item.UpdateG(ctx, boil.Whitelist("channel_id", "use_embeds"))
	if web.CheckErr(templateData, err, "Failed saving item :'(", web.CtxLogger(ctx).Error) {
		return templateData
	}

	templateData.AddAlerts(web.SucessAlert("Sucessfully updated reddit feed! :D"))

	user := ctx.Value(common.ContextKeyUser).(*discordgo.User)
	common.AddCPLogEntry(user, activeGuild.ID, "Modified a feed to /r/"+item.Subreddit)
	return templateData
}

func HandleRemove(w http.ResponseWriter, r *http.Request) interface{} {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)

	currentConfig := ctx.Value(CurrentConfig).(models.RedditFeedSlice)
	templateData["RedditConfig"] = currentConfig

	id := pat.Param(r, "item")
	idInt, err := strconv.ParseInt(id, 10, 32)
	if err != nil {
		return templateData.AddAlerts(web.ErrorAlert("Failed parsing id", err))
	}

	// Get tha actual watch item from the config
	item := FindFeed(currentConfig, idInt)
	if item == nil {
		return templateData.AddAlerts(web.ErrorAlert("Unknown id"))
	}

	_, err = item.DeleteG(ctx)
	if web.CheckErr(templateData, err, "Failed removing item :'(", web.CtxLogger(ctx).Error) {
		return templateData
	}

	templateData.AddAlerts(web.SucessAlert("Sucessfully removed subreddit feed for /r/" + item.Subreddit))

	// Remove it form the displayed list
	for k, c := range currentConfig {
		if c.ID == idInt {
			currentConfig = append(currentConfig[:k], currentConfig[k+1:]...)
		}
	}

	templateData["RedditConfig"] = currentConfig

	user := ctx.Value(common.ContextKeyUser).(*discordgo.User)
	go common.AddCPLogEntry(user, activeGuild.ID, "Removed feed from /r/"+item.Subreddit)
	return templateData
}

func FindFeed(feeds []*models.RedditFeed, id int64) *models.RedditFeed {
	for _, v := range feeds {
		if v.ID == id {
			return v
		}
	}

	return nil
}
