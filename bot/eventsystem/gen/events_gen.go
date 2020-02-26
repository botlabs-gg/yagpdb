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
	"sort"
	"strings"
	"text/template"

	"golang.org/x/tools/go/packages"
)

const templateSource = `// GENERATED using events_gen.go

// Custom event handlers that adds a redis connection to the handler
// They will also recover from panics

package eventsystem

import (
	"github.com/jonas747/discordgo"
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

var handlers = make([][][]*Handler, {{len .}})

{{range .}}{{if .Discord}}
func (data *EventData) {{.Name}}() *discordgo.{{.Name}}{ 
	return data.EvtInterface.(*discordgo.{{.Name}})
}{{end}}{{end}}

func fillEvent(evtData *EventData) {

	switch evtData.EvtInterface.(type){ {{range $k, $v := .}}{{if .Discord}}
	case *discordgo.{{.Name}}:
		evtData.Type = Event({{$k}}){{end}}{{end}}
	default:
		return
	}

	return 
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
	// Sent once a shard either resumes or readies for the first time, signifying that its now ready
	Event{"YagShardReady", false},
	// Sent one or more shards is either migrated to this node or added freshly
	Event{"YagShardsAdded", false},
	// Sent once a shard has been either migrated away or removeotherwise
	Event{"YagShardRemoved", false},
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
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.LoadFiles,
	}, "github.com/jonas747/discordgo")

	if err != nil {
		panic(err)
	}

	fPath := ""
	for _, v := range pkgs {
		log.Println(v.Name, v.PkgPath, v.GoFiles)
		for _, p := range v.GoFiles {
			if strings.HasSuffix(p, "/events.go") {
				fPath = p
			}
		}
	}
	log.Println("Found path: ", fPath)

	fs := token.NewFileSet()
	parsedFile, err := parser.ParseFile(fs, fPath, nil, 0)
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
