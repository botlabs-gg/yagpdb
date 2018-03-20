// +build ignore

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
	"os/exec"
	"path/filepath"
	"sort"
	"text/template"
)

const templateSource = `// GENERATED using events_gen.go

// Custom event handlers that adds a redis connection to the handler
// They will also recover from panics

package eventsystem

import (
	"context"
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"runtime/debug"
)

type Event int

const (
	{{range $k, $v := .}}
	Event{{.Name}} Event = {{$k}}{{end}}
)

var EventNames = []string{ {{range.}}
	"{{.Name}}",{{end}}
}

func (e Event) String() string {
	return EventNames[e]
}

var AllDiscordEvents = []Event{ {{range .}}{{if .Discord}}
	Event{{.Name}},{{end}}{{end}}
}

var AllEvents = []Event{ {{range .}}
	Event{{.Name}},{{end}}
}

var handlers = make([][]*Handler, {{len .}})

type EventDataContainer struct{ {{range .}}{{if .Discord}}
	{{.Name}} *discordgo.{{.Name}}{{end}}{{end}}

	Session *discordgo.Session
}

func HandleEvent(s *discordgo.Session, evt interface{}){

	var evtData = &EventData{
		EventDataContainer: &EventDataContainer{
			Session: s,
		},
		EvtInterface: evt,
	}

	switch t := evt.(type){ {{range $k, $v := .}}{{if .Discord}}
	case *discordgo.{{.Name}}:
		evtData.{{.Name}} = t
		evtData.Type = Event({{$k}}){{end}}{{end}}
	default:
		return
	}

	defer func() {
		if err := recover(); err != nil {
			stack := string(debug.Stack())
			logrus.WithField(logrus.ErrorKey, err).WithField("evt", evtData.Type.String()).Error("Recovered from panic in event handler\n" + stack)
		}
	}()

	ctx := context.WithValue(context.Background(), common.ContextKeyDiscordSession, s)
	evtData.ctx = ctx
	EmitEvent(evtData, EventAllPre)
	EmitEvent(evtData, evtData.Type)
	EmitEvent(evtData, EventAllPost)
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
	flag.StringVar(&flagOut, "o", "../events.go", "Output file")
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
	cmd := exec.Command("go", "fmt")
	err = cmd.Run()
	CheckErr("Failed running gofmt", err)
}
