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
	"github.com/jonas747/yagpdb/analytics"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/jonas747/yagpdb/common/scheduledevents2"
	scheduledEventsModels "github.com/jonas747/yagpdb/common/scheduledevents2/models"
	"github.com/mediocregopher/radix/v3"
)

var _ bot.BotInitHandler = (*Plugin)(nil)
var _ bot.BotStopperHandler = (*Plugin)(nil)
var _ commands.CommandProvider = (*Plugin)(nil)

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(p, roleCommands...)
}

type assignRoleEventdata struct {
	UserID int64
	RoleID int64 // currently unused
}

func (p *Plugin) BotInit() {
	eventsystem.AddHandlerAsyncLast(p, onMemberJoin, eventsystem.EventGuildMemberAdd)
	// eventsystem.AddHandlerAsyncLast(p, HandlePresenceUpdate, eventsystem.EventPresenceUpdate)
	eventsystem.AddHandlerAsyncLastLegacy(p, handleGuildChunk, eventsystem.EventGuildMembersChunk)
	eventsystem.AddHandlerAsyncLast(p, handleGuildMemberUpdate, eventsystem.EventGuildMemberUpdate)

	scheduledevents2.RegisterHandler("autorole_assign_role", assignRoleEventdata{}, handleAssignRole)

	pubsub.AddHandler("autorole_stop_processing", HandleUpdateAutoroles, nil)
	// go runDurationChecker()
}

func (p *Plugin) StopBot(wg *sync.WaitGroup) {
	wg.Done()
}

var roleCommands = []*commands.YAGCommand{
	&commands.YAGCommand{
		CmdCategory: commands.CategoryDebug,
		Name:        "Roledbg",
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
	gs := bot.State.Guild(true, event.TargetGuildInt)
	if gs != nil {
		gs.UserCacheDel(CacheKeyConfig)
	}
}

// HandlePresenceUpdate makes sure the member with joined_at is available for the relevant guilds
// TODO: Figure out a solution that scales better
// func HandlePresenceUpdate(evt *eventsystem.EventData) (retry bool, err error) {
// 	p := evt.PresenceUpdate()
// 	if p.Status == discordgo.StatusOffline {
// 		return
// 	}

// 	gs := evt.GS

// 	gs.RLock()
// 	m := gs.Member(false, p.User.ID)
// 	if m != nil && m.MemberSet {
// 		gs.RUnlock()
// 		return false, nil
// 	}
// 	gs.RUnlock()

// 	config, err := GuildCacheGetGeneralConfig(gs)
// 	if err != nil {
// 		return true, errors.WithStackIf(err)
// 	}

// 	if !config.OnlyOnJoin && config.Role != 0 {
// 		go bot.GetMember(gs.ID, p.User.ID)
// 	}

// 	return false, nil
// }

func saveGeneral(guildID int64, config *GeneralConfig) {

	err := common.SetRedisJson(KeyGeneral(guildID), config)
	if err != nil {
		logger.WithError(err).Error("Failed saving autorole config")
	} else {
		pubsub.Publish("autorole_stop_processing", guildID, nil)
	}
}

func onMemberJoin(evt *eventsystem.EventData) (retry bool, err error) {
	addEvt := evt.GuildMemberAdd()

	config, err := GuildCacheGetGeneralConfig(evt.GS)
	if err != nil {
		return true, errors.WithStackIf(err)
	}

	if config.Role == 0 || evt.GS.Role(true, config.Role) == nil {
		return
	}

	// ms := evt.GS.MemberCopy(true, addEvt.User.ID)
	// if ms == nil {
	// 	logger.Error("Member not found in add event")
	// 	return
	// }

	if config.RequiredDuration < 1 && config.CanAssignTo(addEvt.Roles, time.Now()) {
		_, retry, err = assignRole(config, addEvt.GuildID, addEvt.User.ID)
		return retry, err
	}

	if config.RequiredDuration > 0 && !config.OnlyOnJoin {
		err = scheduledevents2.ScheduleEvent("autorole_assign_role", addEvt.GuildID,
			time.Now().Add(time.Minute*time.Duration(config.RequiredDuration)), &assignRoleEventdata{UserID: addEvt.User.ID})
		return bot.CheckDiscordErrRetry(err), err
	}

	return false, nil
}

func assignRole(config *GeneralConfig, guildID int64, targetID int64) (disabled bool, retry bool, err error) {
	analytics.RecordActiveUnit(guildID, &Plugin{}, "assigned_role")
	err = common.BotSession.GuildMemberRoleAdd(guildID, targetID, config.Role)
	if err != nil {
		switch code, _ := common.DiscordError(err); code {
		case discordgo.ErrCodeUnknownMember:
			logger.WithError(err).Error("Unknown member when trying to assign role")
		case discordgo.ErrCodeMissingPermissions, discordgo.ErrCodeMissingAccess, discordgo.ErrCodeUnknownRole:
			logger.WithError(err).Warn("disabling autorole from error")
			cop := *config
			cop.Role = 0
			saveGeneral(guildID, &cop)
			return true, false, nil
		default:
			return false, bot.CheckDiscordErrRetry(err), err
		}
	}

	return false, false, nil
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

func handleGuildChunk(evt *eventsystem.EventData) {
	chunk := evt.GuildMembersChunk()
	err := common.RedisPool.Do(radix.Cmd(nil, "SETEX", RedisKeyGuildChunkProecssing(chunk.GuildID), "100", "1"))
	if err != nil {
		logger.WithError(err).Error("failed marking autorole chunk processing")
	}

	config, err := GetGeneralConfig(chunk.GuildID)
	if err != nil {
		return
	}

	if config.Role == 0 || config.OnlyOnJoin {
		return
	}

	go assignFromGuildChunk(chunk.GuildID, config, chunk.Members)
}

func assignFromGuildChunk(guildID int64, config *GeneralConfig, members []*discordgo.Member) {
	lastTimeUpdatedBlockingKey := time.Now()
	lastTimeUpdatedConfig := time.Now()

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

		// already has role
		if common.ContainsInt64Slice(m.Roles, config.Role) {
			continue
		}

		time.Sleep(time.Second * 2)

		logger.Println("assigning to ", m.User.ID, " from guild chunk event")

		disabled, _, err := assignRole(config, guildID, m.User.ID)
		if err != nil {
			logger.WithError(err).WithField("user", m.User.ID).WithField("guild", guildID).Error("failed adding autorole role")
		}
		if disabled {
			break
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

			err := common.RedisPool.Do(radix.Cmd(nil, "SETEX", RedisKeyGuildChunkProecssing(guildID), "100", "1"))
			if err != nil {
				logger.WithError(err).Error("failed marking autorole chunk processing")
			}
		}
	}
}

func WorkingOnFullScan(guildID int64) bool {
	var b bool
	err := common.RedisPool.Do(radix.Cmd(&b, "EXISTS", RedisKeyGuildChunkProecssing(guildID)))
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

func handleAssignRole(evt *scheduledEventsModels.ScheduledEvent, data interface{}) (retry bool, err error) {
	config, err := GetGeneralConfig(evt.GuildID)
	if err != nil {
		return true, nil
	}

	if config.Role == 0 || config.OnlyOnJoin {
		// settings changed after they joined
		return false, nil
	}

	dataCast := data.(*assignRoleEventdata)

	member, err := bot.GetMemberJoinedAt(evt.GuildID, dataCast.UserID)
	if err != nil {
		if common.IsDiscordErr(err, discordgo.ErrCodeUnknownMember) {
			return false, nil
		}

		return bot.CheckDiscordErrRetry(err), err
	}

	memberDuration := time.Now().Sub(member.JoinedAt)
	if memberDuration < time.Duration(config.RequiredDuration)*time.Minute {
		// settings may have been changed, re-schedule

		err = scheduledevents2.ScheduleEvent("autorole_assign_role", evt.GuildID,
			time.Now().Add(time.Minute*time.Duration(config.RequiredDuration)), &assignRoleEventdata{UserID: dataCast.UserID})
		return bot.CheckDiscordErrRetry(err), err
	}

	if !config.CanAssignTo(member.Roles, member.JoinedAt) {
		// some other reason they can't get the role, such as whitelist or ignore roles
		return false, nil
	}

	go analytics.RecordActiveUnit(evt.GuildID, &Plugin{}, "assigned_role")

	_, retry, err = assignRole(config, evt.GuildID, dataCast.UserID)
	return retry, err
}

func handleGuildMemberUpdate(evt *eventsystem.EventData) (retry bool, err error) {
	update := evt.GuildMemberUpdate()
	config, err := GuildCacheGetGeneralConfig(evt.GS)
	if err != nil {
		return true, errors.WithStackIf(err)
	}

	if config.Role == 0 || config.OnlyOnJoin || evt.GS.Role(true, config.Role) == nil {
		return false, nil
	}

	if common.ContainsInt64Slice(update.Member.Roles, config.Role) {
		return false, nil
	}

	if !config.CanAssignTo(update.Member.Roles, time.Time{}) {
		return false, nil
	}

	if config.RequiredDuration > 0 {
		// check the autorole duration
		ms, err := bot.GetMemberJoinedAt(update.GuildID, update.User.ID)
		if err != nil {
			return bot.CheckDiscordErrRetry(err), errors.WithStackIf(err)
		}

		if time.Since(ms.JoinedAt) < time.Duration(config.RequiredDuration)*time.Minute {
			// haven't been a member long enough
			return false, nil
		}
	}

	go analytics.RecordActiveUnit(update.GuildID, &Plugin{}, "assigned_role")

	// if we branched here then all the checks passed and they should be assigned the role
	_, retry, err = assignRole(config, update.GuildID, update.User.ID)
	return retry, err
}
