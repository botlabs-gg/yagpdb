package logs

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/bot/paginatedmessages"
	"github.com/botlabs-gg/yagpdb/v2/common/config"
	"github.com/botlabs-gg/yagpdb/v2/common/run"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/botlabs-gg/yagpdb/v2/logs/models"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
)

var _ bot.BotInitHandler = (*Plugin)(nil)
var _ commands.CommandProvider = (*Plugin)(nil)

func (p *Plugin) AddCommands() {
	if confEnableUsernameTracking.GetBool() {
		commands.AddRootCommands(p, cmdLogs, cmdWhois, cmdNicknames, cmdUsernames, cmdClearNames)
	} else {
		commands.AddRootCommands(p, cmdLogs, cmdWhois)
		if run.FlagGenCmdDocs {
			for _, cmd := range []*commands.YAGCommand{cmdNicknames, cmdUsernames, cmdClearNames} {
				cmd.Description = fmt.Sprint(cmd.Description, " Disabled on the public instance.")
				commands.AddRootCommands(p, cmd)
			}
		}
	}
}

func (p *Plugin) BotInit() {
	eventsystem.AddHandlerAsyncLastLegacy(p, bot.ConcurrentEventHandler(HandleQueueEvt), eventsystem.EventGuildMemberUpdate, eventsystem.EventGuildMemberAdd, eventsystem.EventMemberFetched)
	// eventsystem.AddHandlerAsyncLastLegacy(bot.ConcurrentEventHandler(HandleGC), eventsystem.EventGuildCreate)
	eventsystem.AddHandlerAsyncLast(p, HandleMsgDelete, eventsystem.EventMessageDelete, eventsystem.EventMessageDeleteBulk)

	eventsystem.AddHandlerFirstLegacy(p, HandlePresenceUpdate, eventsystem.EventPresenceUpdate)

	go EvtProcesser()
	go EvtProcesserGCs()
}

var cmdLogs = &commands.YAGCommand{
	Cooldown:        5,
	CmdCategory:     commands.CategoryTool,
	Name:            "Logs",
	Aliases:         []string{"log"},
	Description:     "Creates a log of the last messages in the current channel.",
	LongDescription: "This includes deleted messages within an hour (or 12 hours for premium servers)",
	Arguments: []*dcmd.ArgDef{
		{Name: "Count", Default: 100, Type: &dcmd.IntArg{Min: 2, Max: 250}},
	},
	ArgSwitches: []*dcmd.ArgDef{
		{Name: "channel", Help: "Optional channel to log instead", Type: dcmd.Channel},
	},
	SlashCommandEnabled: true,
	DefaultEnabled:      false,
	RunFunc: func(cmd *dcmd.Data) (interface{}, error) {
		num := cmd.Args[0].Int()

		cID := cmd.ChannelID
		if cmd.Switch("channel").Value != nil {
			cID = cmd.Switch("channel").Value.(*dstate.ChannelState).ID

			hasPerms, err := bot.AdminOrPermMS(cmd.GuildData.CS.GuildID, cID, cmd.GuildData.MS, discordgo.PermissionSendMessages|discordgo.PermissionViewChannel|discordgo.PermissionReadMessageHistory)
			if err != nil {
				return "Failed checking permissions, please try again or join the support server.", err
			}

			if !hasPerms {
				return "You do not have permissions to send messages there", nil
			}
		}

		l, err := CreateChannelLog(cmd.Context(), nil, cmd.GuildData.GS.ID, cID, cmd.Author.Username, cmd.Author.ID, num)
		if err != nil {
			if err == ErrChannelBlacklisted {
				return "This channel is ignored from creating message logs, this can be changed in the control panel.", nil
			}

			return "", err
		}

		return CreateLink(cmd.GuildData.GS.ID, l.ID), err
	},
}

var cmdWhois = &commands.YAGCommand{
	CmdCategory: commands.CategoryTool,
	Name:        "Whois",
	Description: "Shows information about a user",
	Aliases:     []string{"whoami"},
	RunInDM:     false,
	Arguments: []*dcmd.ArgDef{
		{Name: "User", Type: &commands.MemberArg{}},
	},
	SlashCommandEnabled: true,
	DefaultEnabled:      false,
	RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
		config, err := GetConfig(common.PQ, parsed.Context(), parsed.GuildData.GS.ID)
		if err != nil {
			return nil, err
		}

		var member *dstate.MemberState
		if parsed.Args[0].Value != nil {
			member = parsed.Args[0].Value.(*dstate.MemberState)
		} else {
			member = parsed.GuildData.MS
			if sm := bot.State.GetMember(parsed.GuildData.GS.ID, member.User.ID); sm != nil {
				// Prefer state member over the one provided in the message, since it may have presence data
				member = sm
			}
		}

		var nick, joinedAtStr, joinedAtDurStr string

		if member.Member == nil {
			joinedAtStr = "Couldn't find out"
			joinedAtDurStr = "Couldn't find out"
		} else {
			parsedJoinedAt, _ := member.Member.JoinedAt.Parse()
			joinedAtStr = parsedJoinedAt.UTC().Format(time.RFC822)
			dur := time.Since(parsedJoinedAt)
			joinedAtDurStr = common.HumanizeDuration(common.DurationPrecisionHours, dur)

			if member.Member.Nick != "" {
				nick = " (" + member.Member.Nick + ")"
			}
		}

		if joinedAtDurStr == "" {
			joinedAtDurStr = "Less than an hour ago"
		}

		t := bot.SnowflakeToTime(member.User.ID)
		createdDurStr := common.HumanizeDuration(common.DurationPrecisionHours, time.Since(t))
		if createdDurStr == "" {
			createdDurStr = "Less than an hour ago"
		}

		var memberStatus string
		state := [6]string{"Playing", "Streaming", "Listening", "Watching", "Custom", "Competing"}
		if member.Presence == nil || member.Presence.Game == nil {
			memberStatus = "Has no active status, is invisible/offline or is not in the bot's cache."
		} else {
			if member.Presence.Game.Type == 4 {
				memberStatus = fmt.Sprintf("%s: %s", member.Presence.Game.Name, member.Presence.Game.State)
			} else {
				presenceName := "Unknown"
				if member.Presence.Game.Type >= 0 && len(state) > int(member.Presence.Game.Type) {
					presenceName = state[member.Presence.Game.Type]
				}

				memberStatus = fmt.Sprintf("%s: %s", presenceName, member.Presence.Game.Name)
			}
		}

		embed := &discordgo.MessageEmbed{
			Title: fmt.Sprintf("%s %s", member.User.String(), nick),
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "ID",
					Value:  discordgo.StrID(member.User.ID),
					Inline: true,
				},
				{
					Name:   "Avatar",
					Value:  "[Link](" + member.User.AvatarURL("256") + ")",
					Inline: true,
				},
				{
					Name:   "Account Created",
					Value:  t.UTC().Format(time.RFC822),
					Inline: true,
				},
				{
					Name:   "Account Age",
					Value:  createdDurStr,
					Inline: true,
				},
				{
					Name:   "Joined Server At",
					Value:  joinedAtStr,
					Inline: true,
				},
				{
					Name:   "Join Server Age",
					Value:  joinedAtDurStr,
					Inline: true,
				},
				{
					Name:   "Status",
					Value:  memberStatus,
					Inline: true,
				},
			},
			Thumbnail: &discordgo.MessageEmbedThumbnail{
				URL: member.User.AvatarURL("256"),
			},
		}

		if confEnableUsernameTracking.GetBool() {
			if config.UsernameLoggingEnabled.Bool {
				usernames, err := GetUsernames(parsed.Context(), member.User.ID, 5, 0)
				if err != nil {
					return err, err
				}

				usernamesStr := "```\n"
				for _, v := range usernames {
					usernamesStr += fmt.Sprintf("%20s: %s\n", v.CreatedAt.Time.UTC().Format(time.RFC822), v.Username.String)
				}
				usernamesStr += "```"

				embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
					Name:  "5 last usernames",
					Value: usernamesStr,
				})
			} else {
				embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
					Name:  "Usernames",
					Value: "Username tracking disabled",
				})
			}

			if config.NicknameLoggingEnabled.Bool {

				nicknames, err := GetNicknames(parsed.Context(), member.User.ID, parsed.GuildData.GS.ID, 5, 0)
				if err != nil {
					return err, err
				}

				nicknameStr := "```\n"
				if len(nicknames) < 1 {
					nicknameStr += "No nicknames tracked"
				} else {
					for _, v := range nicknames {
						nicknameStr += fmt.Sprintf("%20s: %s\n", v.CreatedAt.Time.UTC().Format(time.RFC822), v.Nickname.String)
					}
				}
				nicknameStr += "```"

				embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
					Name:  "5 last nicknames",
					Value: nicknameStr,
				})
			} else {
				embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
					Name:  "Nicknames",
					Value: "Nickname tracking disabled",
				})
			}
		}
		return embed, nil
	},
}

var cmdUsernames = &commands.YAGCommand{
	CmdCategory: commands.CategoryTool,
	Name:        "Usernames",
	Description: "Shows past usernames of a user.",
	Aliases:     []string{"unames", "un"},
	RunInDM:     true,
	Arguments: []*dcmd.ArgDef{
		{Name: "User", Type: dcmd.User},
	},
	RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
		gID := int64(0)
		if parsed.GuildData != nil {
			config, err := GetConfig(common.PQ, parsed.Context(), parsed.GuildData.GS.ID)
			if err != nil {
				return nil, err
			}

			if !config.UsernameLoggingEnabled.Bool {
				return "Username logging is disabled on this server", nil
			}

			gID = parsed.GuildData.GS.ID
		}

		return paginatedmessages.NewPaginatedResponse(gID, parsed.ChannelID, 1, 0, func(p *paginatedmessages.PaginatedMessage, page int) (*discordgo.MessageEmbed, error) {
			target := parsed.Author
			if parsed.Args[0].Value != nil {
				target = parsed.Args[0].Value.(*discordgo.User)
			}

			offset := (page - 1) * 15
			usernames, err := GetUsernames(context.Background(), target.ID, 15, offset)
			if err != nil {
				return nil, err
			}

			if len(usernames) < 1 && page > 1 {
				return nil, paginatedmessages.ErrNoResults
			}

			out := fmt.Sprintf("Past username of **%s** ```\n", target.String())
			for _, v := range usernames {
				out += fmt.Sprintf("%20s: %s\n", v.CreatedAt.Time.UTC().Format(time.RFC822), v.Username.String)
			}
			out += "```"

			if len(usernames) < 1 {
				out = `No logged usernames`
			}

			embed := &discordgo.MessageEmbed{
				Color:       0x277ee3,
				Title:       "Usernames of " + target.String(),
				Description: out,
			}

			return embed, nil
		}), nil
	},
}

var cmdNicknames = &commands.YAGCommand{
	CmdCategory: commands.CategoryTool,
	Name:        "Nicknames",
	Description: "Shows past nicknames of a user.",
	Aliases:     []string{"nn"},
	RunInDM:     false,
	Arguments: []*dcmd.ArgDef{
		{Name: "User", Type: dcmd.User},
	},
	RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
		config, err := GetConfig(common.PQ, parsed.Context(), parsed.GuildData.GS.ID)
		if err != nil {
			return nil, err
		}

		target := parsed.Author
		if parsed.Args[0].Value != nil {
			target = parsed.Args[0].Value.(*discordgo.User)
		}

		if !config.NicknameLoggingEnabled.Bool {
			return "Nickname logging is disabled on this server", nil
		}

		return paginatedmessages.NewPaginatedResponse(parsed.GuildData.GS.ID, parsed.ChannelID, 1, 0, func(p *paginatedmessages.PaginatedMessage, page int) (*discordgo.MessageEmbed, error) {

			offset := (page - 1) * 15

			nicknames, err := GetNicknames(context.Background(), target.ID, parsed.GuildData.GS.ID, 15, offset)
			if err != nil {
				return nil, err
			}

			if page > 1 && len(nicknames) < 1 {
				return nil, paginatedmessages.ErrNoResults
			}

			out := fmt.Sprintf("Past nicknames of **%s** ```\n", target.String())
			for _, v := range nicknames {
				out += fmt.Sprintf("%20s: %s\n", v.CreatedAt.Time.UTC().Format(time.RFC822), v.Nickname.String)
			}
			out += "```"

			if len(nicknames) < 1 {
				out = `No nicknames tracked`
			}

			embed := &discordgo.MessageEmbed{
				Color:       0x277ee3,
				Title:       "Nicknames of " + target.String(),
				Description: out,
			}

			return embed, nil
		}), nil
	},
}

var cmdClearNames = &commands.YAGCommand{
	CmdCategory: commands.CategoryTool,
	Name:        "ResetPastNames",
	Description: "Reset your past usernames/nicknames.",
	RunInDM:     true,
	// Cooldown:    100,
	RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
		queries := []string{
			"DELETE FROM username_listings WHERE user_id=$1",
			"DELETE FROM nickname_listings WHERE user_id=$1",
			"INSERT INTO username_listings (created_at, updated_at, user_id, username) VALUES (now(), now(), $1, '<Usernames reset by user>')",
		}

		for _, v := range queries {
			_, err := common.PQ.Exec(v, parsed.Author.ID)
			if err != nil {
				return "An error occurred, join the support server for help", err
			}
		}

		return "Doneso! Your past nicknames and usernames have been cleared!", nil
	},
}

// Mark all log messages with this id as deleted
func HandleMsgDelete(evt *eventsystem.EventData) (retry bool, err error) {
	if evt.Type == eventsystem.EventMessageDelete {
		err := markLoggedMessageAsDeleted(evt.Context(), evt.MessageDelete().ID)
		if err != nil {
			return true, errors.WithStackIf(err)
		}

		return false, nil
	}

	for _, m := range evt.MessageDeleteBulk().Messages {
		err := markLoggedMessageAsDeleted(evt.Context(), m)
		if err != nil {
			return true, errors.WithStackIf(err)
		}
	}

	return false, nil
}

func markLoggedMessageAsDeleted(ctx context.Context, mID int64) error {
	_, err := models.Messages2s(models.Messages2Where.ID.EQ(mID)).UpdateAllG(ctx,
		models.M{"deleted": true})
	return err
}

func HandlePresenceUpdate(evt *eventsystem.EventData) {
	pu := evt.PresenceUpdate()
	gs := evt.GS

	ms := bot.State.GetMember(gs.ID, pu.User.ID)
	if ms == nil || ms.Presence == nil || ms.Member == nil {
		queueEvt(pu)
		return
	}

	if pu.User.Username != "" && pu.User.Username != ms.User.Username {
		queueEvt(pu)
		return
	}
}

// While presence update is sent when user changes username.... MAKES NO SENSE IMO BUT WHATEVER
// Also check nickname incase the user came online
func HandleQueueEvt(evt *eventsystem.EventData) {
	queueEvt(evt.EvtInterface)
}

func queueEvt(evt interface{}) {
	if !confEnableUsernameTracking.GetBool() {
		return
	}

	select {
	case evtChan <- evt:
		return
	default:
		go func() {
			evtChan <- evt
		}()
	}
}

func HandleGC(evt *eventsystem.EventData) {
	gc := evt.GuildCreate()
	evtChanGC <- &LightGC{
		GuildID: gc.ID,
		Members: gc.Members,
	}
}

func CheckUsername(exec boil.ContextExecutor, ctx context.Context, usernameStmt *sql.Stmt, user *discordgo.User) error {
	var lastUsername string
	row := usernameStmt.QueryRow(user.ID)
	err := row.Scan(&lastUsername)

	if err == nil && lastUsername == user.Username {
		// Not changed
		return nil
	}

	if err != nil && err != sql.ErrNoRows {
		// Other error
		return nil
	}

	listing := &models.UsernameListing{
		UserID:   null.Int64From(user.ID),
		Username: null.StringFrom(user.Username),
	}

	err = listing.Insert(ctx, exec, boil.Infer())
	if err != nil {
		logger.WithError(err).WithField("user", user.ID).Error("failed setting last username")
	}

	return err
}

func CheckNickname(exec boil.ContextExecutor, ctx context.Context, nicknameStmt *sql.Stmt, userID, guildID int64, nickname string) error {

	var lastNickname string
	row := nicknameStmt.QueryRow(userID, guildID)
	err := row.Scan(&lastNickname)
	if err == sql.ErrNoRows && nickname == "" {
		// don't need to be putting this in the database as the first record for the user
		return nil
	}

	if err == nil && lastNickname == nickname {
		// Not changed
		return nil
	}

	if err != sql.ErrNoRows && err != nil {
		return err
	}

	listing := &models.NicknameListing{
		UserID:   null.Int64From(userID),
		GuildID:  null.StringFrom(discordgo.StrID(guildID)),
		Nickname: null.StringFrom(nickname),
	}

	err = listing.Insert(ctx, exec, boil.Infer())
	if err != nil {
		logger.WithError(err).WithField("guild", guildID).WithField("user", userID).Error("failed setting last nickname")
	}

	return err
}

var (
	evtChan   = make(chan interface{}, 1000)
	evtChanGC = make(chan *LightGC)
)

type UserGuildPair struct {
	GuildID int64
	User    *discordgo.User
}

var confEnableUsernameTracking = config.RegisterOption("yagpdb.enable_username_tracking", "Enable username tracking", true)

// Queue up all the events and process them one by one, because of limited connections
func EvtProcesser() {

	queuedMembers := make([]*discordgo.Member, 0)
	queuedUsers := make([]*UserGuildPair, 0)

	ticker := time.NewTicker(time.Second * 10)

	enabled := confEnableUsernameTracking.GetBool()

	for {
		select {
		case e := <-evtChan:
			if !enabled {
				continue
			}

			switch t := e.(type) {
			case *discordgo.PresenceUpdate:
				if t.User.Username == "" {
					continue
				}

				queuedUsers = append(queuedUsers, &UserGuildPair{GuildID: t.GuildID, User: t.User})
			case *discordgo.GuildMemberUpdate:
				queuedMembers = append(queuedMembers, t.Member)
			case *discordgo.GuildMemberAdd:
				queuedMembers = append(queuedMembers, t.Member)
			case *discordgo.Member:
				queuedMembers = append(queuedMembers, t)
			}
		case <-ticker.C:
			if !enabled {
				continue
			}

			started := time.Now()
			err := ProcessBatch(queuedUsers, queuedMembers)
			logger.Debugf("Updated %d members and %d users in %s", len(queuedMembers), len(queuedUsers), time.Since(started).String())
			if err == nil {
				// reset the slices
				queuedUsers = queuedUsers[:0]
				queuedMembers = queuedMembers[:0]
			} else {
				logger.WithError(err).Error("failed batch updating usernames and nicknames")
			}
		}
	}
}

func ProcessBatch(users []*UserGuildPair, members []*discordgo.Member) error {
	configs := make([]*models.GuildLoggingConfig, 0)

	err := common.SqlTX(func(tx *sql.Tx) error {
		nickStatement, err := tx.Prepare("select nickname from nickname_listings where user_id=$1 AND guild_id=$2 order by id desc limit 1;")
		if err != nil {
			return errors.WrapIf(err, "nick stmnt prepare")
		}

		usernameStatement, err := tx.Prepare("select username from username_listings where user_id=$1 order by id desc limit 1;")
		if err != nil {
			return errors.WrapIf(err, "username stmnt prepare")
		}

		// first find all the configs
	OUTERUSERS:
		for _, v := range users {
			for _, c := range configs {
				if c.GuildID == v.GuildID {
					continue OUTERUSERS
				}
			}

			config, err := GetConfigCached(tx, v.GuildID)
			if err != nil {
				return errors.WrapIf(err, "users_configs")
			}

			configs = append(configs, config)
		}

	OUTERMEMBERS:
		for _, v := range members {
			for _, c := range configs {
				if c.GuildID == v.GuildID {
					continue OUTERMEMBERS
				}
			}

			config, err := GetConfigCached(tx, v.GuildID)
			if err != nil {
				return errors.WrapIf(err, "members_configs")
			}

			configs = append(configs, config)
		}

		// update users first
	OUTERUSERS_UPDT:
		for _, v := range users {
			// check if username logging is disabled
			for _, c := range configs {
				if c.GuildID == v.GuildID {
					if !c.UsernameLoggingEnabled.Bool {
						continue OUTERUSERS_UPDT
					}

					break
				}
			}

			err = CheckUsername(tx, context.Background(), usernameStatement, v.User)
			if err != nil {
				return errors.WrapIf(err, "user username check")
			}
		}

		// update members
		for _, v := range members {
			checkNick := false
			checkUser := false

			// find config
			for _, c := range configs {
				if c.GuildID == v.GuildID {
					checkNick = c.NicknameLoggingEnabled.Bool
					checkUser = c.UsernameLoggingEnabled.Bool
					break
				}
			}

			if !checkNick && !checkUser {
				continue
			}

			err = CheckUsername(tx, context.Background(), usernameStatement, v.User)
			if err != nil {
				return errors.WrapIf(err, "members username check")
			}

			err = CheckNickname(tx, context.Background(), nickStatement, v.User.ID, v.GuildID, v.Nick)
			if err != nil {
				return errors.WrapIf(err, "members nickname check")
			}
		}

		return nil
	})

	return err
}

type LightGC struct {
	GuildID int64
	Members []*discordgo.Member
}

func EvtProcesserGCs() {
	for {
		<-evtChanGC
	}
}

var configCache = common.CacheSet.RegisterSlot("logs_config", nil, int64(0))

func GetConfigCached(exec boil.ContextExecutor, gID int64) (*models.GuildLoggingConfig, error) {
	gs := bot.State.GetGuild(gID)
	if gs == nil {
		return GetConfig(exec, context.Background(), gID)
	}

	v, err := configCache.GetCustomFetch(gs.ID, func(key interface{}) (interface{}, error) {
		conf, err := GetConfig(exec, context.Background(), gID)
		return conf, err
	})

	if err != nil {
		return nil, err
	}

	return v.(*models.GuildLoggingConfig), nil
}
