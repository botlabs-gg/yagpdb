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
	"github.com/jonas747/discordgo"
	"github.com/fzzy/radix/redis"
	"log"
	"runtime/debug"
)

{{range .}}
func Custom{{.}}(inner func(s *discordgo.Session, evt *discordgo.{{.}}, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.{{.}}) {
	return func(s *discordgo.Session, evt *discordgo.{{.}}) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event {{.}}:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in {{.}}:", err, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}
{{end}}
`

var Events = []string{
	"ChannelCreate",
	"ChannelUpdate",
	"ChannelDelete",
	// "ChannelPinsUpdate", Waiting for pr to be merged
	"GuildCreate",
	"GuildUpdate",
	"GuildDelete",
	"GuildBanAdd",
	"GuildBanRemove",
	"GuildMemberAdd",
	"GuildMemberUpdate",
	"GuildMemberRemove",
	"GuildRoleCreate",
	"GuildRoleUpdate",
	"GuildRoleDelete",
	"GuildIntegrationsUpdate",
	"GuildEmojisUpdate",
	"MessageAck",
	"MessageCreate",
	"MessageUpdate",
	"MessageDelete",
	"PresenceUpdate",
	"PresencesReplace",
	"Ready",
	"UserUpdate",
	"UserSettingsUpdate",
	"UserGuildSettingsUpdate",
	"TypingStart",
	"VoiceServerUpdate",
	"VoiceStateUpdate",
	"Resumed",
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
