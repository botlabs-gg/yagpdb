package autorole

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/dutil/dstate"
	"github.com/jonas747/yagpdb/bot"
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
		Category: commands.CategoryTool,
		Command: &commandsystem.Command{
			Name:        "Role",
			Description: "Give yourself a role or list all available roles",
			Arguments: []*commandsystem.ArgDef{
				&commandsystem.ArgDef{Name: "Role", Type: commandsystem.ArgumentString},
			},
			Run: func(parsed *commandsystem.ExecData) (interface{}, error) {
				client := parsed.Context().Value(commands.CtxKeyRedisClient).(*redis.Client)
				roleCommands, err := GetCommands(client, parsed.Guild.ID())
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
						out = "Sorry " + common.RandomAdjective() + " person, i do not recognize that role (maybe your finger slipped?), heres a list of the roles you can assign yourself:"
					}

					usedCommands := make([]string, 0, len(roleCommands))
					for _, r := range roleCommands {
						if common.FindStringSlice(usedCommands, r.Role) {
							continue
						}

						out += "\n"
						first := true
						for _, r2 := range roleCommands {
							if r2.Role == r.Role {
								if !first {
									out += "/"
								}
								first = false
								out += "`" + r.Name + "` "
							}
						}

						usedCommands = append(usedCommands, r.Role)
					}

					if len(roleCommands) < 1 {
						out += "\nNo self assignable roles set up. Server admins can set them up in the control panel."
					}

					return out, nil
				}

				member, err := bot.GetMember(parsed.Guild.ID(), parsed.Message.Author.ID)
				if err != nil {
					return "Failed assigning role, contact bot support (bot error, not permissions)", err
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

				if found {
					err = common.RemoveRole(member, role, parsed.Guild.ID())
				} else {
					err = common.AddRole(member, role, parsed.Guild.ID())
				}

				if err != nil {
					if cast, ok := err.(*discordgo.RESTError); ok && cast.Message != nil {
						return "API error, Discord said: " + cast.Message.Message, err
					}

					return "Something went wrong :upside_down: ", err
				}

				if found {
					return "Took away your role!", nil
				}

				return "Gave you the role!", nil
			},
		},
	},
	&commands.CustomCommand{
		Category: commands.CategoryDebug,
		Command: &commandsystem.Command{
			Name:        "roledbg",
			Description: "Debug debug debug autorole assignment",
			Run: func(parsed *commandsystem.ExecData) (interface{}, error) {
				client := parsed.Context().Value(commands.CtxKeyRedisClient).(*redis.Client)
				processing, _ := client.Cmd("GET", KeyProcessing(parsed.Guild.ID())).Int()
				return fmt.Sprintf("Processing %d users.", processing), nil
			},
		},
	},
}

var _ bot.BotStarterHandler = (*Plugin)(nil)

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
	state := bot.State

	for {
		<-ticker.C

		// Copy the list of guilds so that we dont need to keep the entire state locked
		state.RLock()
		guildStates := make([]*dstate.GuildState, len(state.Guilds))
		i := 0
		for _, v := range state.Guilds {
			guildStates[i] = v
			i++
		}
		state.RUnlock()

		for _, g := range guildStates {
			checkGuild(client, g)
		}
	}
}

func checkGuild(client *redis.Client, gs *dstate.GuildState) {
	gs.RLock()
	defer gs.RUnlock()
	if gs.Guild.Unavailable {
		return
	}

	logger := logrus.WithField("guild", gs.ID())

	perms, err := gs.MemberPermissions(false, gs.ID(), bot.State.User(true).ID)
	if err != nil {
		logger.WithError(err).Error("Error checking perms")
		return
	}

	if perms&discordgo.PermissionManageRoles == 0 {
		// Not enough permissions to assign roles, skip this guild
		return
	}

	conf, err := GetGeneralConfig(client, gs.ID())
	if err != nil {
		logger.WithError(err).Error("Failed retrieivng general config")
		return
	}

	if conf.Role == "" {
		return
	}

	// Make sure the role exists
	for _, role := range gs.Guild.Roles {
		if role.ID == conf.Role {
			go processGuild(gs, conf)
			return
		}
	}

	// If not remove it
	logger.Info("Autorole role dosen't exist, removing config...")
	conf.Role = ""
	saveGeneral(client, gs.ID(), conf)
}

func processGuild(gs *dstate.GuildState, config *GeneralConfig) {
	processingLock.Lock()

	if _, ok := processingGuilds[gs.ID()]; ok {
		// Still processing this guild
		processingLock.Unlock()
		return
	}
	stopChan := make(chan bool, 1)
	processingGuilds[gs.ID()] = stopChan
	processingLock.Unlock()

	var client *redis.Client

	// Reset the processing state
	defer func() {
		processingLock.Lock()
		delete(processingGuilds, gs.ID())
		processingLock.Unlock()

		if client != nil {
			client.Cmd("DEL", KeyProcessing(gs.ID()))
			common.RedisPool.Put(client)
		}
	}()

	membersToGiveRole := make([]string, 0)

	gs.RLock()

	now := time.Now()
OUTER:
	for _, ms := range gs.Members {
		if ms.Member == nil {
			continue
		}

		parsedJoined, err := discordgo.Timestamp(ms.Member.JoinedAt).Parse()
		if err != nil {
			logrus.WithError(err).Error("Failed parsing join timestamp")
			continue
		}

		if now.Sub(parsedJoined) > time.Duration(config.RequiredDuration)*time.Minute {
			for _, r := range ms.Member.Roles {
				if r == config.Role {
					continue OUTER
				}
			}

			membersToGiveRole = append(membersToGiveRole, ms.ID())
		}
	}

	gs.RUnlock()

	if len(membersToGiveRole) > 10 {
		var err error
		client, err = common.RedisPool.Get()
		if err != nil {
			logrus.WithError(err).Error("Failed retrieving redis client from pool")
			return
		}
		client.Cmd("SET", KeyProcessing(gs.ID()), len(membersToGiveRole))
	}

	for i, userID := range membersToGiveRole {
		select {
		case <-stopChan:
			logrus.WithField("guild", gs.ID()).Info("Stopping autorole assigning...")
			return
		default:
		}

		err := common.BotSession.GuildMemberRoleAdd(gs.ID(), userID, config.Role)
		if err != nil {
			if cast, ok := err.(*discordgo.RESTError); ok && cast.Message != nil && cast.Message.Code == 50013 {
				// No perms, remove autorole
				logrus.WithError(err).Info("No perms to add autorole, removing from config")
				config.Role = ""
				saveGeneral(client, gs.ID(), config)
				return
			}
			logrus.WithError(err).WithField("guild", gs.ID()).Error("Failed adding autorole role")
		} else {
			if client != nil {
				client.Cmd("SET", KeyProcessing(gs.ID()), len(membersToGiveRole)-i)
			}
			logrus.WithField("guild", gs.ID()).WithField("user", userID).Info("Gave autorole role")
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
