package autorole

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/retryableredis"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/jonas747/yagpdb/premium"
)

var _ bot.BotInitHandler = (*Plugin)(nil)
var _ bot.BotStopperHandler = (*Plugin)(nil)
var _ commands.CommandProvider = (*Plugin)(nil)

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(roleCommands...)
}

func (p *Plugin) BotInit() {
	eventsystem.AddHandlerAsyncLast(p, OnMemberJoin, eventsystem.EventGuildMemberAdd)
	eventsystem.AddHandlerAsyncLast(p, HandlePresenceUpdate, eventsystem.EventPresenceUpdate)
	eventsystem.AddHandlerAsyncLastLegacy(p, HandleGuildChunk, eventsystem.EventGuildMembersChunk)

	pubsub.AddHandler("autorole_stop_processing", HandleUpdateAutoroles, nil)
	go runDurationChecker()
}

func (p *Plugin) StopBot(wg *sync.WaitGroup) {
	close(completeStop)
	wg.Done()
}

var roleCommands = []*commands.YAGCommand{
	&commands.YAGCommand{
		CmdCategory: commands.CategoryDebug,
		Name:        "Roledbg",
		Description: "Debug debug debug autorole assignment",
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			var processing int
			err := common.RedisPool.Do(retryableredis.Cmd(&processing, "GET", KeyProcessing(parsed.GS.ID)))
			return fmt.Sprintf("Processing %d users.", processing), err
		},
	},
}

// Stop updating
func HandleUpdateAutoroles(event *pubsub.Event) {
	gs := bot.State.Guild(true, event.TargetGuildInt)
	if gs != nil {
		gs.UserCacheDel(CacheKeyConfig)
	}

	stopProcessing(event.TargetGuildInt)
}

// HandlePresenceUpdate makes sure the member with joined_at is available for the relevant guilds
// TODO: Figure out a solution that scales better
func HandlePresenceUpdate(evt *eventsystem.EventData) (retry bool, err error) {
	p := evt.PresenceUpdate()
	if p.Status == discordgo.StatusOffline {
		return
	}

	gs := evt.GS

	gs.RLock()
	m := gs.Member(false, p.User.ID)
	if m != nil && m.MemberSet {
		gs.RUnlock()
		return false, nil
	}
	gs.RUnlock()

	config, err := GuildCacheGetGeneralConfig(gs)
	if err != nil {
		return true, errors.WithStackIf(err)
	}

	if !config.OnlyOnJoin && config.Role != 0 {
		go bot.GetMember(gs.ID, p.User.ID)
	}

	return false, nil
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

	ticker := time.NewTicker(time.Second)
	state := bot.State

	var guildsToCheck []*dstate.GuildState
	var i int
	var numToCheckPerRun int

	for {
		select {
		case <-completeStop:
			return
		case <-ticker.C:
		}

		nonPremiumRetroActiveAssignment := confDisableNonPremiumRetroActiveAssignment.GetBool()

		if len(guildsToCheck) < 0 || i >= len(guildsToCheck) {
			// Copy the list of guilds so that we dont need to keep the entire state locked
			guildsToCheck = state.GuildsSlice(true)
			i = 0

			// Hit each guild once per minute
			numToCheckPerRun = len(guildsToCheck) / 60
			if numToCheckPerRun < 1 {
				numToCheckPerRun = 1
			}
		}

		for checkedThisRound := 0; i < len(guildsToCheck) && checkedThisRound < numToCheckPerRun; i++ {
			g := guildsToCheck[i]
			if !nonPremiumRetroActiveAssignment {
				if ok, err := premium.IsGuildPremium(g.ID); !ok && err == nil {
					continue
				} else if err != nil {
					logger.WithError(err).Error("failed checking if guild is premium")
				}
			}

			checkGuild(g)

			checkedThisRound++
		}
	}
}

// returns true if we should skip the guild
func stateLockedSkipGuild(gs *dstate.GuildState, conf *GeneralConfig) bool {
	gs.RLock()
	defer gs.RUnlock()

	if gs.Guild.Unavailable {
		return true
	}

	if !bot.BotProbablyHasPermissionGS(false, gs, 0, discordgo.PermissionManageRoles) {
		return true
	}

	if gs.Role(false, conf.Role) == nil {
		conf.Role = 0
		saveGeneral(gs.ID, conf)
		return true
	}

	return false
}

func checkGuild(gs *dstate.GuildState) {
	conf, err := GuildCacheGetGeneralConfig(gs)
	if err != nil {
		logger.WithField("guild", gs.ID).WithError(err).Error("Failed retrieivng general config")
		return
	}

	if conf.Role == 0 || conf.OnlyOnJoin {
		return
	}

	if stateLockedSkipGuild(gs, conf) {
		return
	}

	if WorkingOnFullScan(gs.ID) {
		return // Working on a full scan, do nothing
	}

	go processGuild(gs, conf)
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
			common.RedisPool.Do(retryableredis.Cmd(nil, "DEL", KeyProcessing(gs.ID)))
		}
	}()

	membersToGiveRole := make([]int64, 0)

	gs.RLock()

OUTER:
	for _, ms := range gs.Members {
		if !ms.MemberSet {
			continue
		}

		if config.CanAssignTo(ms.Roles, ms.JoinedAt) {
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
		common.RedisPool.Do(retryableredis.FlatCmd(nil, "SET", KeyProcessing(gs.ID), len(membersToGiveRole)))
	}

	cntSinceLastRedisUpdate := 0
	for i, userID := range membersToGiveRole {
		time.Sleep(time.Second * 2)
		select {
		case <-stopChan:
			logger.WithField("guild", gs.ID).Info("Stopping autorole assigning...")
			return
		default:
		}

		cntSinceLastRedisUpdate++

		if gs.Member(true, userID) == nil {
			continue
		}

		err := common.BotSession.GuildMemberRoleAdd(gs.ID, userID, config.Role)
		if err != nil {
			if cast, ok := err.(*discordgo.RESTError); ok && cast.Message != nil && cast.Message.Code == 50013 {
				// No perms, remove autorole
				logger.WithError(err).Info("No perms to add autorole, removing from config")
				config.Role = 0
				saveGeneral(gs.ID, config)
				return
			}
			logger.WithError(err).WithField("guild", gs.ID).Error("Failed adding autorole role")
		} else {
			if setProcessingRedis && cntSinceLastRedisUpdate > 10 {
				common.RedisPool.Do(retryableredis.FlatCmd(nil, "SET", KeyProcessing(gs.ID), len(membersToGiveRole)-i))
				cntSinceLastRedisUpdate = 0
			}
		}
	}
}

func saveGeneral(guildID int64, config *GeneralConfig) {

	err := common.SetRedisJson(KeyGeneral(guildID), config)
	if err != nil {
		logger.WithError(err).Error("Failed saving autorole config")
	}
}

func OnMemberJoin(evt *eventsystem.EventData) (retry bool, err error) {
	addEvt := evt.GuildMemberAdd()

	config, err := GetGeneralConfig(addEvt.GuildID)
	if err != nil {
		return true, errors.WithStackIf(err)
	}

	ms := evt.GS.MemberCopy(true, addEvt.User.ID)
	if ms == nil {
		logger.Error("Member not found in add event")
		return
	}

	if config.Role != 0 && config.RequiredDuration < 1 && config.CanAssignTo(ms.Roles, ms.JoinedAt) {
		common.BotSession.GuildMemberRoleAdd(addEvt.GuildID, addEvt.User.ID, config.Role)
	}

	return false, nil
}

func (conf *GeneralConfig) CanAssignTo(currentRoles []int64, joinedAt time.Time) bool {
	if time.Since(joinedAt) < time.Duration(conf.RequiredDuration)*time.Minute {
		return false
	}

	if len(conf.IgnoreRoles) < 1 && len(conf.RequiredRoles) < 1 {
		return true
	}

	for _, ignoreRole := range conf.IgnoreRoles {
		if common.ContainsInt64Slice(currentRoles, ignoreRole) {
			return false
		}
	}

	// If require roles are set up, make sure the member has one of them
	if len(conf.RequiredRoles) > 0 {
		for _, reqRole := range conf.RequiredRoles {
			if common.ContainsInt64Slice(currentRoles, reqRole) {
				return true
			}
		}
		return false
	}

	return true
}

func RedisKeyGuildChunkProecssing(gID int64) string {
	return "autorole_guild_chunk_processing:" + strconv.FormatInt(gID, 10)
}

func HandleGuildChunk(evt *eventsystem.EventData) {
	chunk := evt.GuildMembersChunk()
	err := common.RedisPool.Do(retryableredis.Cmd(nil, "SETEX", RedisKeyGuildChunkProecssing(chunk.GuildID), "100", "1"))
	if err != nil {
		logger.WithError(err).Error("failed marking autorole chunk processing")
	}

	config, err := GetGeneralConfig(chunk.GuildID)
	if err != nil {
		return
	}

	if config.Role == 0 {
		return
	}

	stopProcessing(chunk.GuildID)
	go assignFromGuildChunk(chunk.GuildID, config, chunk.Members)
}

func assignFromGuildChunk(guildID int64, config *GeneralConfig, members []*discordgo.Member) {
	lastTimeUpdatedBlockingKey := time.Now()
	lastTimeUpdatedConfig := time.Now()

OUTER:
	for _, m := range members {
		joinedAt, err := m.JoinedAt.Parse()
		if err != nil {
			logger.WithError(err).WithField("ts", m.JoinedAt).WithField("user", m.User.ID).WithField("guild", guildID).Error("failed parsing join timestamp")
			if config.RequiredDuration > 0 {
				continue // Need the joined_at field for this
			}
		}

		if !config.CanAssignTo(m.Roles, joinedAt) {
			continue
		}

		for _, r := range m.Roles {
			if r == config.Role {
				continue OUTER
			}
		}

		time.Sleep(time.Second * 2)

		logger.Println("assigning to ", m.User.ID, " from guild chunk event")
		err = common.AddRole(m, config.Role, guildID)
		if err != nil {
			logger.WithError(err).WithField("user", m.User.ID).WithField("guild", guildID).Error("failed adding autorole role")

			if common.IsDiscordErr(err, 50013, 10011) {
				// No perms, remove autorole
				logger.WithError(err).WithField("guild", guildID).Info("No perms to add autorole, or nonexistant, removing from config")
				config.Role = 0
				saveGeneral(guildID, config)
				return
			}
		}

		if time.Since(lastTimeUpdatedConfig) > time.Second*10 {
			// Refresh the config occasionally to make sure it dosen't go stale
			newConf, err := GetGeneralConfig(guildID)
			if err == nil {
				config = newConf
			} else {
				return
			}

			lastTimeUpdatedConfig = time.Now()

			config = newConf
			if config.Role == 0 {
				logger.WithField("guild", guildID).Info("autorole role was set to none in the middle of full retroactive assignment, cancelling")
				return
			}
		}

		if time.Since(lastTimeUpdatedBlockingKey) > time.Second*10 {
			lastTimeUpdatedBlockingKey = time.Now()

			err := common.RedisPool.Do(retryableredis.Cmd(nil, "SETEX", RedisKeyGuildChunkProecssing(guildID), "100", "1"))
			if err != nil {
				logger.WithError(err).Error("failed marking autorole chunk processing")
			}
		}
	}
}

func WorkingOnFullScan(guildID int64) bool {
	var b bool
	err := common.RedisPool.Do(retryableredis.Cmd(&b, "EXISTS", RedisKeyGuildChunkProecssing(guildID)))
	if err != nil {
		logger.WithError(err).WithField("guild", guildID).Error("failed checking WorkingOnFullScan")
		return false
	}

	return b
}

type CacheKey int

const CacheKeyConfig CacheKey = 1

func GuildCacheGetGeneralConfig(gs *dstate.GuildState) (*GeneralConfig, error) {
	v, err := gs.UserCacheFetch(CacheKeyConfig, func() (interface{}, error) {
		config, err := GetGeneralConfig(gs.ID)
		return config, err
	})

	if err != nil {
		return nil, err
	}

	return v.(*GeneralConfig), nil
}
