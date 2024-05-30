// Generates the wrapper event handlers for discordgo events
// The wrappers adds an extra parameter to the handlers which is a redis connection
// And will also recover from panic that occured inside them
package main

import (
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"sort"
	"text/template"
)

const templateSource = `// GENERATED using yagpdb/cmd/gen/bot_wrappers.go

// Custom event handlers that adds a redis connection to the handler
// They will also recover from panics

package bot

import (
	"context"
	"github.com/sirupsen/logrus"
	 "github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
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

	EmitEvent(ctx, EventAllPre, evt)
	EmitEvent(ctx, Event(evtId), evt)
	EmitEvent(ctx, EventAllPost, evt)
}
`

type Event struct {
	Name    string
	Discord bool
}

var NonStandardEvents = []Event{
	Event{"NewGuild", false},
	Event{"All", false},
	Event{"AllPre", false},
	Event{"AllPost", false},
	Event{"MemberFetched", false},
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

	fs := token.NewFileSet()
	parsedFile, err := parser.ParseFile(fs, filepath.Join(os.Getenv("GOPATH"), "src/github.com/jonas747/discordgo/events.go"), nil, 0)
	if err != nil {
		log.Fatalf("warning: internal error: could not parse events.go: %s", err)
		return
	}

	names := []string{}
	for name, _ := range parsedFile.Scope.Objects {
		names = append(names, name)
	}
	sort.Strings(names)

	// Create the combined event slice
	events := make([]Event, len(names)+len(NonStandardEvents)-1)
	copy(events, NonStandardEvents)
	i := len(NonStandardEvents)
	for _, name := range names {
		if name == "Event" {
			continue
		}
		evt := Event{
			Name:    name,
			Discord: true,
		}
		events[i] = evt
		i++
	}

	file, err := os.Create(flagOut)
	CheckErr("Failed creating output file", err)
	defer file.Close()
	err = parsedTemplate.Execute(file, events)
	CheckErr("Failed executing template", err)
}
