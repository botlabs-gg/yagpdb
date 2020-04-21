package logs

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"emperror.dev/errors"
	"github.com/jonas747/yagpdb/bot/paginatedmessages"
	"github.com/jonas747/yagpdb/common/config"

	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/logs/models"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
)

var _ bot.BotInitHandler = (*Plugin)(nil)
var _ commands.CommandProvider = (*Plugin)(nil)

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(p, cmdLogs, cmdWhois, cmdNicknames, cmdUsernames, cmdMigrate, cmdClearNames)
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
		&dcmd.ArgDef{Name: "Count", Default: 100, Type: &dcmd.IntArg{Min: 2, Max: 250}},
	},
	RunFunc: func(cmd *dcmd.Data) (interface{}, error) {
		num := cmd.Args[0].Int()

		l, err := CreateChannelLog(cmd.Context(), nil, cmd.GS.ID, cmd.CS.ID, cmd.Msg.Author.Username, cmd.Msg.Author.ID, num)
		if err != nil {
			if err == ErrChannelBlacklisted {
				return "This channel is blacklisted from creating message logs, this can be changed in the control panel.", nil
			}

			return "", err
		}

		return CreateLink(cmd.GS.ID, l.ID), err
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
	RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
		config, err := GetConfig(common.PQ, parsed.Context(), parsed.GS.ID)
		if err != nil {
			return nil, err
		}

		var member *dstate.MemberState
		if parsed.Args[0].Value != nil {
			member = parsed.Args[0].Value.(*dstate.MemberState)
		} else {
			member = parsed.MS
			if sm := parsed.GS.MemberCopy(true, member.ID); sm != nil {
				// Prefer state member over the one provided in the message, since it may have presence data
				member = sm
			}
		}

		nick := ""
		if member.Nick != "" {
			nick = " (" + member.Nick + ")"
		}

		joinedAtStr := ""
		joinedAtDurStr := ""
		if !member.MemberSet {
			joinedAtStr = "Couldn't find out"
			joinedAtDurStr = "Couldn't find out"
		} else {
			joinedAtStr = member.JoinedAt.UTC().Format(time.RFC822)
			dur := time.Since(member.JoinedAt)
			joinedAtDurStr = common.HumanizeDuration(common.DurationPrecisionHours, dur)
		}

		if joinedAtDurStr == "" {
			joinedAtDurStr = "Less than an hour ago"
		}

		t := bot.SnowflakeToTime(member.ID)
		createdDurStr := common.HumanizeDuration(common.DurationPrecisionHours, time.Since(t))
		if createdDurStr == "" {
			createdDurStr = "Less than an hour ago"
		}

		var memberStatus string
		state := [4]string{"Playing", "Streaming", "Listening", "Watching"}
		if !member.PresenceSet || member.PresenceGame == nil {
			memberStatus = fmt.Sprintf("Has no active status, is invisible/offline or is not in the bot's cache.")
		} else {
			if member.PresenceGame.Type == 4 {
				memberStatus = fmt.Sprintf("%s: %s", member.PresenceGame.Name, member.PresenceGame.State)
			} else {
				memberStatus = fmt.Sprintf("%s: %s", state[member.PresenceGame.Type], member.PresenceGame.Name)
			}
		}

		embed := &discordgo.MessageEmbed{
			Title: fmt.Sprintf("%s#%04d%s", member.Username, member.Discriminator, nick),
			Fields: []*discordgo.MessageEmbedField{
				&discordgo.MessageEmbedField{
					Name:   "ID",
					Value:  discordgo.StrID(member.ID),
					Inline: true,
				},
				&discordgo.MessageEmbedField{
					Name:   "Avatar",
					Value:  "[Link](" + discordgo.EndpointUserAvatar(member.ID, member.StrAvatar()) + ")",
					Inline: true,
				},
				&discordgo.MessageEmbedField{
					Name:   "Account Created",
					Value:  t.UTC().Format(time.RFC822),
					Inline: true,
				},
				&discordgo.MessageEmbedField{
					Name:   "Account Age",
					Value:  createdDurStr,
					Inline: true,
				},
				&discordgo.MessageEmbedField{
					Name:   "Joined Server At",
					Value:  joinedAtStr,
					Inline: true,
				}, &discordgo.MessageEmbedField{
					Name:   "Join Server Age",
					Value:  joinedAtDurStr,
					Inline: true,
				},
				&discordgo.MessageEmbedField{
					Name:   "Status",
					Value:  memberStatus,
					Inline: true,
				},
			},
			Thumbnail: &discordgo.MessageEmbedThumbnail{
				URL: discordgo.EndpointUserAvatar(member.ID, member.StrAvatar()),
			},
		}

		if config.UsernameLoggingEnabled.Bool {
			usernames, err := GetUsernames(parsed.Context(), member.ID, 5, 0)
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

			nicknames, err := GetNicknames(parsed.Context(), member.ID, parsed.GS.ID, 5, 0)
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
		if parsed.GS != nil {
			config, err := GetConfig(common.PQ, parsed.Context(), parsed.GS.ID)
			if err != nil {
				return nil, err
			}

			if !config.UsernameLoggingEnabled.Bool {
				return "Username logging is disabled on this server", nil
			}

			gID = parsed.GS.ID
		}

		_, err := paginatedmessages.CreatePaginatedMessage(gID, parsed.Msg.ChannelID, 1, 0, func(p *paginatedmessages.PaginatedMessage, page int) (*discordgo.MessageEmbed, error) {
			target := parsed.Msg.Author
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

			out := fmt.Sprintf("Past username of **%s#%s** ```\n", target.Username, target.Discriminator)
			for _, v := range usernames {
				out += fmt.Sprintf("%20s: %s\n", v.CreatedAt.Time.UTC().Format(time.RFC822), v.Username.String)
			}
			out += "```"

			if len(usernames) < 1 {
				out = `No logged usernames`
			}

			embed := &discordgo.MessageEmbed{
				Color:       0x277ee3,
				Title:       "Usernames of " + target.Username + "#" + target.Discriminator,
				Description: out,
			}

			return embed, nil
		})

		return nil, err
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
		config, err := GetConfig(common.PQ, parsed.Context(), parsed.GS.ID)
		if err != nil {
			return nil, err
		}

		target := parsed.Msg.Author
		if parsed.Args[0].Value != nil {
			target = parsed.Args[0].Value.(*discordgo.User)
		}

		if !config.NicknameLoggingEnabled.Bool {
			return "Nickname logging is disabled on this server", nil
		}

		_, err = paginatedmessages.CreatePaginatedMessage(parsed.GS.ID, parsed.CS.ID, 1, 0, func(p *paginatedmessages.PaginatedMessage, page int) (*discordgo.MessageEmbed, error) {

			offset := (page - 1) * 15

			nicknames, err := GetNicknames(context.Background(), target.ID, parsed.GS.ID, 15, offset)
			if err != nil {
				return nil, err
			}

			if page > 1 && len(nicknames) < 1 {
				return nil, paginatedmessages.ErrNoResults
			}

			out := fmt.Sprintf("Past nicknames of **%s#%s** ```\n", target.Username, target.Discriminator)
			for _, v := range nicknames {
				out += fmt.Sprintf("%20s: %s\n", v.CreatedAt.Time.UTC().Format(time.RFC822), v.Nickname.String)
			}
			out += "```"

			if len(nicknames) < 1 {
				out = `No nicknames tracked`
			}

			embed := &discordgo.MessageEmbed{
				Color:       0x277ee3,
				Title:       "Nicknames of " + target.Username + "#" + target.Discriminator,
				Description: out,
			}

			return embed, nil
		})

		return nil, err
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
			_, err := common.PQ.Exec(v, parsed.Msg.Author.ID)
			if err != nil {
				return "An error occured, join the support server for help", err
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

	gs.RLock()
	defer gs.RUnlock()

	ms := gs.Member(false, pu.User.ID)
	if ms == nil || !ms.PresenceSet || !ms.MemberSet {
		queueEvt(pu)
		return
	}

	if pu.User.Username != "" && pu.User.Username != ms.Username {
		queueEvt(pu)
		return
	}

	if pu.Nick != ms.Nick {
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
	if os.Getenv("YAGPDB_LOGS_DISABLE_USERNAME_TRACKING") != "" {
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

// type UsernameListing struct {
// 	gorm.Model
// 	UserID   int64 `gorm:"index"`
// 	Username string
// }

// type NicknameListing struct {
// 	gorm.Model
// 	UserID   int64 `gorm:"index"`
// 	GuildID  string
// 	Nickname string
// }

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

	logger.Debug("User changed username, old:", lastUsername, " | new:", user.Username)

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

	logger.Debug("User changed nickname, old:", lastNickname, " | new:", nickname)

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

// func CheckNicknameBulk(gDB *gorm.DB, guildID int64, members []*discordgo.Member) {

// 	ids := make([]int64, 0, len(members))
// 	for _, v := range members {
// 		ids = append(ids, v.User.ID)
// 	}

// 	rows, err := gDB.CommonDB().Query(
// 		"select distinct on(user_id) nickname,user_id from nickname_listings where user_id = ANY ($1) AND guild_id=$2 order by user_id,id desc;", pq.Int64Array(ids), guildID)
// 	if err != nil {
// 		logger.WithError(err).Error("Failed querying current nicknames")
// 	}

// 	// Value is wether the nickname was identical
// 	queriedUsers := make(map[int64]bool)

// 	for rows.Next() {
// 		var nickname string
// 		var userID int64
// 		err = rows.Scan(&nickname, &userID)
// 		if err != nil {
// 			logger.WithError(err).Error("Error while scanning")
// 			continue
// 		}

// 		for _, member := range members {
// 			if member.User.ID == userID {
// 				if member.Nick == nickname {
// 					// Already have the last username tracked
// 					queriedUsers[userID] = true
// 				} else {
// 					queriedUsers[userID] = false
// 					logger.Debug("CHANGED Nick: ", nickname, " : ", member.Nick)
// 				}

// 				break
// 			}
// 		}
// 	}
// 	rows.Close()

// 	for _, member := range members {
// 		unchanged, queried := queriedUsers[member.User.ID]
// 		if queried && unchanged {
// 			continue
// 		}

// 		if !queried && member.Nick == "" {
// 			// don't need to be putting this in the database as the first record for the user
// 			continue
// 		}

// 		logger.Debug("User changed nickname, new: ", member.Nick)

// 		listing := NicknameListing{
// 			UserID:   member.User.ID,
// 			GuildID:  discordgo.StrID(guildID),
// 			Nickname: member.Nick,
// 		}

// 		err = gDB.Create(&listing).Error
// 		if err != nil {
// 			logger.WithError(err).Error("Failed setting nickname")
// 		}
// 	}

// }
// func CheckUsernameBulk(gDB *gorm.DB, users []*discordgo.User) {

// 	ids := make([]int64, 0, len(users))
// 	for _, v := range users {
// 		ids = append(ids, v.ID)
// 	}

// 	rows, err := gDB.CommonDB().Query(
// 		"select distinct on(user_id) username,user_id from username_listings where user_id = ANY ($1) order by user_id,id desc;", pq.Int64Array(ids))
// 	if err != nil {
// 		logger.WithError(err).Error("Failed querying current usernames")
// 	}

// 	unchangedUsers := make(map[int64]bool)

// 	for rows.Next() {
// 		var username string
// 		var userID int64
// 		err = rows.Scan(&username, &userID)
// 		if err != nil {
// 			logger.WithError(err).Error("Error while scanning")
// 			continue
// 		}

// 		// var foundUser *discordgo.User
// 		for _, user := range users {
// 			if user.ID == userID {
// 				if user.Username == username {
// 					// Already have the last username tracked
// 					unchangedUsers[userID] = true
// 				}

// 				break
// 			}
// 		}
// 	}
// 	rows.Close()

// 	for _, user := range users {
// 		if unchanged, ok := unchangedUsers[user.ID]; ok && unchanged {
// 			continue
// 		}

// 		logger.Debug("User changed username, new: ", user.Username)

// 		listing := UsernameListing{
// 			UserID:   user.ID,
// 			Username: user.Username,
// 		}

// 		err = gDB.Create(&listing).Error
// 		if err != nil {
// 			logger.WithError(err).Error("Failed setting username")
// 		}
// 	}
// }

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

		// tx := common.GORM.Begin()

		// conf, err := GetConfig(gc.GuildID)
		// if err != nil {
		// 	logger.WithError(err).Error("Failed fetching config")
		// 	continue
		// }

		// started := time.Now()

		// users := make([]*discordgo.User, len(gc.Members))
		// for i, m := range gc.Members {
		// 	users[i] = m.User
		// }

		// if conf.NicknameLoggingEnabled {
		// 	CheckNicknameBulk(tx, gc.GuildID, gc.Members)
		// }

		// if conf.UsernameLoggingEnabled {
		// 	CheckUsernameBulk(tx, users)
		// }

		// err = tx.Commit().Error
		// if err != nil {
		// 	logger.WithError(err).Error("Failed committing transaction")
		// 	continue
		// }

		// if len(gc.Members) > 100 {
		// 	logger.Infof("Checked %d members in %s", len(gc.Members), time.Since(started).String())
		// 	// Make sure this dosen't use all our resources
		// 	time.Sleep(time.Second * 25)
		// } else {
		// 	time.Sleep(time.Second * 15)
		// }
	}
}

const CacheKeyConfig bot.GSCacheKey = "logs_config"

func GetConfigCached(exec boil.ContextExecutor, gID int64) (*models.GuildLoggingConfig, error) {
	gs := bot.State.Guild(true, gID)
	if gs == nil {
		return GetConfig(exec, context.Background(), gID)
	}

	v, err := gs.UserCacheFetch(CacheKeyConfig, func() (interface{}, error) {
		conf, err := GetConfig(exec, context.Background(), gID)
		return conf, err
	})

	if err != nil {
		return nil, err
	}

	return v.(*models.GuildLoggingConfig), nil
}
