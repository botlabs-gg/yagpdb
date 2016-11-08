package autorole

import (
	"github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
	"golang.org/x/net/context"
	"html/template"
	"net/http"
)

type Form struct {
	General  *GeneralConfig
	Commands []*RoleCommand
}

func (f Form) Save(client *redis.Client, guildID string) error {
	bot.PublishEvent(client, "autorole_stop_processing", guildID, nil)

	realCommands := make([]*RoleCommand, 0)

	for _, v := range f.Commands {
		if v != nil {
			realCommands = append(realCommands, v)
		}
	}
	f.Commands = realCommands

	err := common.SetRedisJson(client, KeyGeneral(guildID), f.General)
	if err != nil {
		return err
	}

	err = common.SetRedisJson(client, KeyCommands(guildID), f.Commands)
	return err
}

func (f Form) Name() string {
	return "Autorole"
}

func (p *Plugin) InitWeb() {
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/autorole.html"))

	muxer := goji.SubMux()

	web.CPMux.HandleC(pat.New("/autorole"), muxer)
	web.CPMux.HandleC(pat.New("/autorole/*"), muxer)

	muxer.UseC(web.RequireFullGuildMW)

	getHandler := web.RenderHandler(HandleAutoroles, "cp_autorole")

	muxer.HandleC(pat.Get(""), getHandler)
	muxer.HandleC(pat.Get("/"), getHandler)

	muxer.HandleC(pat.Post(""), web.SimpleConfigSaverHandler(Form{}, getHandler))
	muxer.HandleC(pat.Post("/"), web.SimpleConfigSaverHandler(Form{}, getHandler))
}

func HandleAutoroles(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	client, activeGuild, tmpl := web.GetBaseCPContextData(ctx)

	commands, err := GetCommands(client, activeGuild.ID)
	web.CheckErr(tmpl, err, "Failed retrieving commands (contact support)", logrus.Error)
	tmpl["RoleCommands"] = commands

	general, err := GetGeneralConfig(client, activeGuild.ID)
	web.CheckErr(tmpl, err, "Failed retrieving general config (contact support)", logrus.Error)
	tmpl["Autorole"] = general

	proc, _ := client.Cmd("GET", KeyProcessing(activeGuild.ID)).Int()
	tmpl["Processing"] = proc
	tmpl["ProcessingETA"] = int(proc / 60)

	return tmpl

}
