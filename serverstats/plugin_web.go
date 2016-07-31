package serverstats

import (
	"encoding/json"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
	"golang.org/x/net/context"
	"html/template"
	"log"
	"net/http"
)

func (p *Plugin) InitWeb(rootMux, cpMux *goji.Mux) {
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/serverstats.html"))
	cpMux.HandleC(pat.Get("/cp/:server/stats"), publicHandler(HandleStatsHtml, false))
	cpMux.HandleC(pat.Get("/cp/:server/stats/"), publicHandler(HandleStatsHtml, false))
	cpMux.HandleFuncC(pat.Post("/cp/:server/stats/settings"), HandleStatsSettings)
	cpMux.HandleC(pat.Get("/cp/:server/stats/full"), publicHandler(HandleStatsJson, false))

	// Public
	rootMux.HandleC(pat.Get("/public/:server/stats"), publicHandler(HandleStatsHtml, true))
	rootMux.HandleC(pat.Get("/public/:server/stats/full"), publicHandler(HandleStatsJson, true))
}

func publicHandler(inner func(ctx context.Context, w http.ResponseWriter, r *http.Request, publicAccess bool), public bool) goji.Handler {
	mw := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		inner(ctx, w, r, public)
	}

	return goji.HandlerFunc(mw)
}

// Somewhat dirty - should clean up this mess sometime
func HandleStatsHtml(ctx context.Context, w http.ResponseWriter, r *http.Request, isPublicAccess bool) {
	client := web.RedisClientFromContext(ctx)

	var guildID string
	var guildName string
	templateData := make(map[string]interface{})

	if !isPublicAccess {
		_, activeGuild, t := web.GetBaseCPContextData(ctx)
		templateData = t
		guildID = activeGuild.ID
		guildName = activeGuild.Name
	} else {
		guildID = pat.Param(ctx, "server")
		public, _ := client.Cmd("GET", "stats_settings_public:"+guildID).Bool()
		if !public {
			return
		}

		guild, err := common.GetGuild(client, guildID)
		if err != nil {
			log.Println("Failed retrieving guildname")
			return
		}
		guildName = guild.Name

		templateData["public_guild_id"] = guildID
	}

	templateData["guild_name"] = guildName
	templateData["current_page"] = "serverstats"

	if !isPublicAccess {
		public, _ := client.Cmd("GET", "stats_settings_public:"+guildID).Bool()
		templateData["public"] = public
	}

	web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_serverstats", templateData))
}

func HandleStatsSettings(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)
	templateData["current_page"] = "serverstats"

	public := r.FormValue("public") == "on"

	current, _ := client.Cmd("GET", "stats_settings_public:"+activeGuild.ID).Bool()
	err := client.Cmd("SET", "stats_settings_public:"+activeGuild.ID, public).Err

	if err != nil {
		log.Println("Error saving stats setting", err)
		templateData["public"] = current
	} else {
		templateData["public"] = public
	}

	web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_serverstats", templateData))
}

func HandleStatsJson(ctx context.Context, w http.ResponseWriter, r *http.Request, isPublicAccess bool) {
	client := web.RedisClientFromContext(ctx)

	var guildID string

	if !isPublicAccess {
		_, activeGuild, _ := web.GetBaseCPContextData(ctx)
		guildID = activeGuild.ID
	} else {
		guildID = pat.Param(ctx, "server")
		public, _ := client.Cmd("GET", "stats_settings_public:"+guildID).Bool()
		if !public {
			return
		}
	}

	stats, err := RetrieveFullStats(client, guildID)
	if err != nil {
		log.Println("Failed retrieving stats", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	out, err := json.Marshal(stats)
	if err != nil {
		log.Println("Failed Encoding stats", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(out)
}
