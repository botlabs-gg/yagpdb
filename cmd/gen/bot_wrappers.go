// Generates the wrapper event handlers for discordgo events
// The wrappers adds an extra parameter to the handlers which is a redis connection
// And will also recover from panic that occured inside them
package main

import (
	"flag"
	"fmt"
	"os"
	"text/template"
)

const templateSource = `// GENERATED using yagpdb/cmd/gen/bot_wrappers.go

// Custom event handlers that adds a redis connection to the handler
// They will also recover from panics

package bot

import (
	"context"
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"runtime/debug"
)

type Event int

const (
	{{range $k, $v := .}}
	Event{{.Name}} Event = {{$k}}{{end}}
)

var AllDiscordEvents = []Event{ {{range .}}{{if .Discord}}
	Event{{.Name}},{{end}}{{end}}
}

type Handler func(ctx context.Context, evt interface{})
var handlers = make([][]*Handler, {{len .}})

func handleEvent(s *discordgo.Session, evt interface{}){

	evtId := -10
	name := ""

	switch evt.(type){ {{range $k, $v := .}}{{if .Discord}}
	case *discordgo.{{.Name}}:
		evtId = {{$k}}
		name = "{{.Name}}"{{end}}{{end}}
	default:
		return
	}

	defer func() {
		if err := recover(); err != nil {
			stack := string(debug.Stack())
			logrus.WithField(logrus.ErrorKey, err).WithField("evt", name).Error("Recovered from panic in event handler\n" + stack)
		}
	}()

	ctx := context.WithValue(context.Background(), ContextKeySession, s)

	triggerHandlers(ctx, EventAllPre, evt)
	triggerHandlers(ctx, Event(evtId), evt)
	triggerHandlers(ctx, EventAllPost, evt)
}
`

type Event struct {
	Name    string
	Discord bool
}

var Events = []Event{
	Event{"ChannelCreate", true},
	Event{"ChannelUpdate", true},
	Event{"ChannelDelete", true},
	Event{"ChannelPinsUpdate", true},
	Event{"GuildCreate", true},
	Event{"GuildUpdate", true},
	Event{"GuildDelete", true},
	Event{"GuildBanAdd", true},
	Event{"GuildBanRemove", true},
	Event{"GuildMemberAdd", true},
	Event{"GuildMemberUpdate", true},
	Event{"GuildMemberRemove", true},
	Event{"GuildMembersChunk", true},
	Event{"GuildRoleCreate", true},
	Event{"GuildRoleUpdate", true},
	Event{"GuildRoleDelete", true},
	Event{"GuildIntegrationsUpdate", true},
	Event{"GuildEmojisUpdate", true},
	Event{"MessageAck", true},
	Event{"MessageCreate", true},
	Event{"MessageUpdate", true},
	Event{"MessageDelete", true},
	Event{"PresenceUpdate", true},
	Event{"PresencesReplace", true},
	Event{"Ready", true},
	Event{"UserUpdate", true},
	Event{"UserSettingsUpdate", true},
	Event{"UserGuildSettingsUpdate", true},
	Event{"TypingStart", true},
	Event{"VoiceServerUpdate", true},
	Event{"VoiceStateUpdate", true},
	Event{"Resumed", true},

	Event{"All", false},
	Event{"AllPre", false},
	Event{"AllPost", false},
}

var (
	parsedTemplate = template.Must(template.New("").Parse(templateSource))
	flagOut        string
)

func init() {
	flag.StringVar(&flagOut, "o", "../../bot/wrappers.go", "Output file")
	flag.Parse()
}

func CheckErr(errMsg string, err error) {
	if err != nil {
		fmt.Println(errMsg+":", err)
		os.Exit(1)
	}
}

func main() {
	file, err := os.Create(flagOut)
	CheckErr("Failed creating output file", err)
	defer file.Close()
	err = parsedTemplate.Execute(file, Events)
	CheckErr("Failed executing template", err)
}
