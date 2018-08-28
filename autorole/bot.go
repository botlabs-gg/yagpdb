package autorole

import (
	"fmt"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/mediocregopher/radix.v3"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

var _ bot.BotInitHandler = (*Plugin)(nil)
var _ bot.BotStartedHandler = (*Plugin)(nil)
var _ bot.BotStopperHandler = (*Plugin)(nil)
var _ commands.CommandProvider = (*Plugin)(nil)

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(roleCommands...)
}

func (p *Plugin) BotInit() {
	eventsystem.AddHandler(OnMemberJoin, eventsystem.EventGuildMemberAdd)
	eventsystem.AddHandler(HandlePresenceUpdate, eventsystem.EventPresenceUpdate)

	pubsub.AddHandler("autorole_stop_processing", HandleUpdateAutoroles, nil)
}

func (p *Plugin) BotStarted() {
	go runDurationChecker()
}

func (p *Plugin) StopBot(wg *sync.WaitGroup) {
	close(completeStop)
	wg.Done()
}

var roleCommands = []*commands.YAGCommand{
	&commands.YAGCommand{
		CmdCategory: commands.CategoryDebug,
		Name:        "roledbg",
		Description: "Debug debug debug autorole assignment",
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			var processing int
			err := common.RedisPool.Do(radix.Cmd(&processing, "GET", KeyProcessing(parsed.GS.ID)))
			return fmt.Sprintf("Processing %d users.", processing), err
		},
	},
}

// Stop updating
func HandleUpdateAutoroles(event *pubsub.Event) {
	stopProcessing(event.TargetGuildInt)
}

// HandlePresenceUpdate makes sure the member with joined_at is available for the relevant guilds
// TODO: Figure out a solution that scales better
func HandlePresenceUpdate(evt *eventsystem.EventData) {
	p := evt.PresenceUpdate()
	if p.Status == discordgo.StatusOffline {
		return
	}

	gs := bot.State.Guild(true, p.GuildID)
	if gs == nil {
		return
	}
	gs.RLock()
	m := gs.Member(false, p.User.ID)
	if m != nil && m.MemberSet {
		gs.RUnlock()
		return
	}
	gs.RUnlock()

	config, err := GetGeneralConfig(gs.ID)
	if err != nil {
		return
	}

	if !config.OnlyOnJoin && config.Role != 0 {
		go bot.GetMember(gs.ID, p.User.ID)
	}
}

var (
	processingGuilds = make(map[int64]chan bool)
	processingLock   sync.Mutex
	completeStop     = make(chan bool)
)

func stopProcessing(guildID int64) {
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

	ticker := time.NewTicker(time.Minute)
	state := bot.State

	for {
		select {
		case <-completeStop:
			return
		case <-ticker.C:
		}

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
			checkGuild(g)
		}
	}
}

func checkGuild(gs *dstate.GuildState) {
	gs.RLock()
	defer gs.RUnlock()
	if gs.Guild.Unavailable {
		return
	}

	logger := logrus.WithField("guild", gs.ID)

	perms, err := gs.MemberPermissions(false, 0, common.BotUser.ID)
	if err != nil && err != dstate.ErrChannelNotFound {
		logger.WithError(err).Error("Error checking perms")
		return
	}

	if perms&discordgo.PermissionManageRoles == 0 {
		// Not enough permissions to assign roles, skip this guild
		return
	}

	conf, err := GetGeneralConfig(gs.ID)
	if err != nil {
		logger.WithError(err).Error("Failed retrieivng general config")
		return
	}

	if conf.Role == 0 || conf.OnlyOnJoin {
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
	conf.Role = 0
	saveGeneral(gs.ID, conf)
}

func processGuild(gs *dstate.GuildState, config *GeneralConfig) {
	processingLock.Lock()

	if _, ok := processingGuilds[gs.ID]; ok {
		// Still processing this guild
		processingLock.Unlock()
		return
	}
	stopChan := make(chan bool, 1)
	processingGuilds[gs.ID] = stopChan
	processingLock.Unlock()

	var setProcessingRedis bool
	// Reset the processing state
	defer func() {
		processingLock.Lock()
		delete(processingGuilds, gs.ID)
		processingLock.Unlock()

		if setProcessingRedis {
			common.RedisPool.Do(radix.Cmd(nil, "DEL", KeyProcessing(gs.ID)))
		}
	}()

	membersToGiveRole := make([]int64, 0)

	gs.RLock()

	now := time.Now()
OUTER:
	for _, ms := range gs.Members {
		if !ms.MemberSet {
			continue
		}

		if now.Sub(ms.JoinedAt) > time.Duration(config.RequiredDuration)*time.Minute && config.CanAssignTo(ms) {
			for _, r := range ms.Roles {
				if r == config.Role {
					continue OUTER
				}
			}

			membersToGiveRole = append(membersToGiveRole, ms.ID)
		}
	}

	gs.RUnlock()

	if len(membersToGiveRole) > 10 {
		setProcessingRedis = true
		common.RedisPool.Do(radix.FlatCmd(nil, "SET", KeyProcessing(gs.ID), len(membersToGiveRole)))
	}

	cntSinceLastRedisUpdate := 0
	for i, userID := range membersToGiveRole {
		select {
		case <-stopChan:
			logrus.WithField("guild", gs.ID).Info("Stopping autorole assigning...")
			return
		default:
		}

		cntSinceLastRedisUpdate++

		err := common.BotSession.GuildMemberRoleAdd(gs.ID, userID, config.Role)
		if err != nil {
			if cast, ok := err.(*discordgo.RESTError); ok && cast.Message != nil && cast.Message.Code == 50013 {
				// No perms, remove autorole
				logrus.WithError(err).Info("No perms to add autorole, removing from config")
				config.Role = 0
				saveGeneral(gs.ID, config)
				return
			}
			logrus.WithError(err).WithField("guild", gs.ID).Error("Failed adding autorole role")
		} else {
			if setProcessingRedis && cntSinceLastRedisUpdate > 10 {
				common.RedisPool.Do(radix.FlatCmd(nil, "SET", KeyProcessing(gs.ID), len(membersToGiveRole)-i))
				cntSinceLastRedisUpdate = 0
			}
			logrus.WithField("guild", gs.ID).WithField("user", userID).Debug("Gave autorole role")
		}
	}
}

func saveGeneral(guildID int64, config *GeneralConfig) {

	err := common.SetRedisJson(KeyGeneral(guildID), config)
	if err != nil {
		logrus.WithError(err).Error("Failed saving autorole config")
	}
}

func OnMemberJoin(evt *eventsystem.EventData) {
	addEvt := evt.GuildMemberAdd()

	config, err := GetGeneralConfig(addEvt.GuildID)
	if err != nil {
		return
	}

	gs := bot.State.Guild(true, addEvt.GuildID)
	ms := gs.MemberCopy(true, addEvt.User.ID)
	if ms == nil {
		logrus.Error("Member not found in add event")
		return
	}

	if config.Role != 0 && config.RequiredDuration < 1 && config.CanAssignTo(ms) {
		common.BotSession.GuildMemberRoleAdd(addEvt.GuildID, addEvt.User.ID, config.Role)
	}
}

func (conf *GeneralConfig) CanAssignTo(ms *dstate.MemberState) bool {
	if len(conf.IgnoreRoles) < 1 && len(conf.RequiredRoles) < 1 {
		return true
	}

	for _, ignoreRole := range conf.IgnoreRoles {
		if common.ContainsInt64Slice(ms.Roles, ignoreRole) {
			return false
		}
	}

	// If require roles are set up, make sure the member has one of them
	if len(conf.RequiredRoles) > 0 {
		for _, reqRole := range conf.RequiredRoles {
			if common.ContainsInt64Slice(ms.Roles, reqRole) {
				return true
			}
		}
		return false
	}

	return true
}
