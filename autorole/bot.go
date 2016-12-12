package autorole

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/pubsub"
	"strings"
	"sync"
	"time"
)

func (p *Plugin) InitBot() {
	commands.CommandSystem.RegisterCommands(roleCommands...)
}

var roleCommands = []commandsystem.CommandHandler{
	&commands.CustomCommand{
		Cooldown: 10,
		Category: commands.CategoryTool,
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:        "Role",
			Description: "Give yourself a role or list all available roles",
			Arguments: []*commandsystem.ArgumentDef{
				&commandsystem.ArgumentDef{Name: "Role", Type: commandsystem.ArgumentTypeString},
			},
		},
		RunFunc: func(parsed *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			roleCommands, err := GetCommands(client, parsed.Guild.ID)
			if err != nil {
				return "Failed retrieving roles, contact support", err
			}

			role := ""
			if parsed.Args[0] != nil {
				for _, v := range roleCommands {
					if strings.EqualFold(v.Name, parsed.Args[0].Str()) {
						role = v.Role
						break
					}
				}
			}

			// If no role
			if parsed.Args[0] == nil || role == "" {

				out := "Here is a list of roles you can assign yourself:"
				if parsed.Args[0] != nil {
					// We failed to find the proper role
					out = "Sorry sir, i do not recognize that role (maybe your finger slipped?), heres a list of the roles you can assign yourself:"
				}

				for _, r := range roleCommands {
					out += "\n`" + r.Name + "`"
				}

				if len(roleCommands) < 1 {
					out += "\nNo self assignable roles set up. Server admins can set them up in the control panel."
				}

				return out, nil
			}

			member, err := common.GetGuildMember(common.BotSession, parsed.Guild.ID, m.Author.ID)
			if err != nil {
				return "Failed assigning role, contact support (bot error, not permissions)", err
			}

			found := false
			newRoles := make([]string, 0)
			for _, v := range member.Roles {
				if v == role {
					found = true
				} else {
					newRoles = append(newRoles, v)
				}
			}

			if !found {
				newRoles = append(newRoles, role)
			}

			err = common.BotSession.GuildMemberEdit(parsed.Guild.ID, m.Author.ID, newRoles)
			if err != nil {
				if cast, ok := err.(*discordgo.RESTError); ok && cast.Message != nil {
					return "API error, Discord said: " + cast.Message.Message, err
				}
				return "Something went wrong :upside_down: ", err
			}

			if found {
				return "Took away your role! :eyes: ", nil
			}

			return "Gave you the role sir! :water_polo:", nil
		},
	},
	&commands.CustomCommand{
		Cooldown: 10,
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:         "roledbg",
			HideFromHelp: true,
			Description:  "Debug debug debug autorole assignment",
		},
		RunFunc: func(parsed *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			processing, _ := client.Cmd("GET", KeyProcessing(parsed.Guild.ID)).Int()
			return fmt.Sprintf("Processing %d users.", processing), nil
		},
	},
}

func (p *Plugin) StartBot() {
	go runDurationChecker()
	pubsub.AddHandler("autorole_stop_processing", HandleUpdateAutomodRules, nil)
}

// Stop updating
func HandleUpdateAutomodRules(event *pubsub.Event) {
	stopProcessing(event.TargetGuild)
}

var (
	processingGuilds = make(map[string]chan bool)
	processingLock   sync.Mutex
)

func stopProcessing(guildID string) {
	processingLock.Lock()
	if c, ok := processingGuilds[guildID]; ok {
		go func() {
			select {
			case c <- true:
			default:
				return
			}
		}()
	}
	processingLock.Unlock()
}

func runDurationChecker() {

	client, err := common.RedisPool.Get()
	if err != nil {
		panic(err)
	}

	ticker := time.NewTicker(time.Minute)
	state := common.BotSession.State

OUTER:
	for {
		<-ticker.C

		state.RLock()

		for _, g := range state.Guilds {
			if g.Unavailable {
				continue
			}

			state.RUnlock()
			perms, err := state.UserChannelPermissions(common.BotSession.State.User.ID, g.ID)
			state.RLock()
			if err != nil {
				logrus.WithError(err).Error("Error checking perms")
				continue
			}

			if perms&discordgo.PermissionManageRoles == 0 {
				continue
			}

			conf, err := GetGeneralConfig(client, g.ID)
			if err != nil {
				logrus.WithError(err).Error("Failed retrieivng general config")
				continue
			}

			if conf.Role == "" {
				continue
			}

			// Make sure the role exists
			for _, role := range g.Roles {
				if role.ID == conf.Role {
					go processGuild(g, conf)
					continue OUTER
				}
			}

			// If not remove it
			logrus.WithField("guild", g.ID).Info("Autorole role dosen't exist, removing...")
			conf.Role = ""
			saveGeneral(client, g.ID, conf)
		}

		state.RUnlock()
	}
}

func processGuild(guild *discordgo.Guild, config *GeneralConfig) {
	processingLock.Lock()

	if _, ok := processingGuilds[guild.ID]; ok {
		// Still processing this guild
		processingLock.Unlock()
		return
	}
	stopChan := make(chan bool, 1)
	processingGuilds[guild.ID] = stopChan
	processingLock.Unlock()

	var client *redis.Client

	// Reset the processing state
	defer func() {
		processingLock.Lock()
		delete(processingGuilds, guild.ID)
		processingLock.Unlock()

		if client != nil {
			client.Cmd("DEL", KeyProcessing(guild.ID))
			common.RedisPool.Put(client)
		}
	}()

	membersToGiveRole := make([]string, 0)

	now := time.Now()
OUTER:
	for _, member := range guild.Members {
		parsedJoined, err := discordgo.Timestamp(member.JoinedAt).Parse()
		if err != nil {
			logrus.WithError(err).Error("Failed parsing join timestamp")
			continue
		}

		if now.Sub(parsedJoined) > time.Duration(config.RequiredDuration)*time.Minute {
			for _, r := range member.Roles {
				if r == config.Role {
					continue OUTER
				}
			}

			membersToGiveRole = append(membersToGiveRole, member.User.ID)
		}
	}

	if len(membersToGiveRole) > 10 {
		var err error
		client, err = common.RedisPool.Get()
		if err != nil {
			logrus.WithError(err).Error("Failed retrieving redis client from pool")
			return
		}
		client.Cmd("SET", KeyProcessing(guild.ID), len(membersToGiveRole))
	}

	for i, userID := range membersToGiveRole {
		select {
		case <-stopChan:
			logrus.WithField("guild", guild.ID).Info("Stopping autorole assigning...")
			return
		default:
		}

		err := common.BotSession.GuildMemberRoleAdd(guild.ID, userID, config.Role)
		if err != nil {
			if cast, ok := err.(*discordgo.RESTError); ok && cast.Message != nil && cast.Message.Code == 50013 {
				// No perms, remove autorole
				logrus.WithError(err).Info("No perms to add autorole, removing from config")
				config.Role = ""
				saveGeneral(client, guild.ID, config)
				return
			}
			logrus.WithError(err).WithField("guild", guild.ID).Error("Failed adding autorole role")
		} else {
			if client != nil {
				client.Cmd("SET", KeyProcessing(guild.ID), len(membersToGiveRole)-i)
			}
			logrus.WithField("guild", guild.ID).WithField("g_name", guild.Name).WithField("user", userID).Info("Gave autorole role")
		}
	}
}

func saveGeneral(client *redis.Client, guildID string, config *GeneralConfig) {
	if client == nil {
		var err error
		client, err = common.RedisPool.Get()
		if err != nil {
			logrus.WithError(err).Error("Failed retrieving redis connection")
			return
		}

		common.RedisPool.Put(client)
	}

	err := common.SetRedisJson(client, KeyGeneral(guildID), config)
	if err != nil {
		logrus.WithError(err).Error("Failed saving autorole config")
	}
}
