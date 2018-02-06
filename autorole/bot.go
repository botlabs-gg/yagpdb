package autorole

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/dutil/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/mediocregopher/radix.v2/redis"
	"strconv"
	"sync"
	"time"
)

func (p *Plugin) InitBot() {
	commands.CommandSystem.RegisterCommands(roleCommands...)
}

var roleCommands = []commandsystem.CommandHandler{
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

	perms, err := gs.MemberPermissions(false, "", bot.State.User(true).ID)
	if err != nil && err != dstate.ErrChannelNotFound {
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

	if conf.Role == "" || conf.OnlyOnJoin {
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

		if now.Sub(parsedJoined) > time.Duration(config.RequiredDuration)*time.Minute && config.CanAssignTo(ms.Member) {
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

func (conf *GeneralConfig) CanAssignTo(member *discordgo.Member) bool {
	if len(conf.IgnoreRoles) < 1 && len(conf.RequiredRoles) < 1 {
		return true
	}

	parsedMemberRoles := make([]int64, len(member.Roles))
	for i, v := range member.Roles {
		p, _ := strconv.ParseInt(v, 10, 64)
		parsedMemberRoles[i] = p
	}

	for _, ignoreRole := range conf.IgnoreRoles {
		if common.ContainsInt64Slice(parsedMemberRoles, ignoreRole) {
			return false
		}
	}

	// If require roles are set up, make sure the member has one of them
	if len(conf.RequiredRoles) > 0 {
		for _, reqRole := range conf.RequiredRoles {
			if common.ContainsInt64Slice(parsedMemberRoles, reqRole) {
				return true
			}
		}
		return false
	}

	return true

}
