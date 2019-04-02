package logs

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/logs/models"
	// "github.com/lib/pq"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"time"
)

var (
	nicknameQueryStatement *sql.Stmt
	usernameQueryStatement *sql.Stmt
)

var _ bot.BotInitHandler = (*Plugin)(nil)
var _ commands.CommandProvider = (*Plugin)(nil)

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(cmdLogs, cmdWhois, cmdNicknames, cmdUsernames)
}

func (p *Plugin) BotInit() {
	eventsystem.AddHandler(bot.ConcurrentEventHandler(HandleQueueEvt), eventsystem.EventGuildMemberUpdate, eventsystem.EventGuildMemberAdd, eventsystem.EventMemberFetched)
	eventsystem.AddHandler(bot.ConcurrentEventHandler(HandleGC), eventsystem.EventGuildCreate)
	eventsystem.AddHandler(bot.ConcurrentEventHandler(HandleMsgDelete), eventsystem.EventMessageDelete, eventsystem.EventMessageDeleteBulk)

	eventsystem.AddHandlerBefore(HandlePresenceUpdate, eventsystem.EventPresenceUpdate, bot.StateHandlerPtr)

	var err error
	nicknameQueryStatement, err = common.PQ.Prepare("select nickname from nickname_listings where user_id=$1 AND guild_id=$2 order by id desc limit 1;")
	if err != nil {
		panic("Failed preparing nick statement: " + err.Error())
	}

	usernameQueryStatement, err = common.PQ.Prepare("select username from username_listings where user_id=$1 order by id desc limit 1;")
	if err != nil {
		panic("Failed preparing username statement: " + err.Error())
	}

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
		config, err := GetConfig(parsed.Context(), parsed.GS.ID)
		if err != nil {
			return nil, err
		}

		member := commands.ContextMS(parsed.Context())
		if parsed.Args[0].Value != nil {
			member = parsed.Args[0].Value.(*dstate.MemberState)
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
			joinedAtDurStr = "Lesss than an hour ago"
		}

		t := bot.SnowflakeToTime(member.ID)
		createdDurStr := common.HumanizeDuration(common.DurationPrecisionHours, time.Since(t))
		if createdDurStr == "" {
			createdDurStr = "Less than an hour ago"
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
					Name:   "Account created",
					Value:  t.UTC().Format(time.RFC822),
					Inline: true,
				},
				&discordgo.MessageEmbedField{
					Name:   "Account Age",
					Value:  createdDurStr,
					Inline: true,
				},
				&discordgo.MessageEmbedField{
					Name:   "Joined server at",
					Value:  joinedAtStr,
					Inline: true,
				}, &discordgo.MessageEmbedField{
					Name:   "Join server Age",
					Value:  joinedAtDurStr,
					Inline: true,
				},
			},
			Thumbnail: &discordgo.MessageEmbedThumbnail{
				URL: discordgo.EndpointUserAvatar(member.ID, member.StrAvatar()),
			},
		}

		if config.UsernameLoggingEnabled.Bool {
			usernames, err := GetUsernames(parsed.Context(), member.ID, 5)
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

			nicknames, err := GetNicknames(parsed.Context(), member.ID, parsed.GS.ID, 5)
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
	CmdCategory:     commands.CategoryTool,
	Name:            "Usernames",
	Description:     "Shows past usernames of a user.",
	LongDescription: "Only shows up to the last 25 usernames.",
	Aliases:         []string{"unames", "un"},
	RunInDM:         true,
	Arguments: []*dcmd.ArgDef{
		{Name: "User", Type: dcmd.User},
	},
	RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
		if parsed.GS != nil {
			config, err := GetConfig(parsed.Context(), parsed.GS.ID)
			if err != nil {
				return nil, err
			}

			if !config.UsernameLoggingEnabled.Bool {
				return "Username logging is disabled on this server", nil
			}
		}

		target := parsed.Msg.Author
		if parsed.Args[0].Value != nil {
			target = parsed.Args[0].Value.(*discordgo.User)
		}

		usernames, err := GetUsernames(parsed.Context(), target.ID, 25)
		if err != nil {
			return nil, err
		}

		out := fmt.Sprintf("Past username of **%s#%s** ```\n", target.Username, target.Discriminator)
		for _, v := range usernames {
			out += fmt.Sprintf("%20s: %s\n", v.CreatedAt.Time.UTC().Format(time.RFC822), v.Username.String)
		}
		out += "```"
		if len(usernames) == 25 {
			out += "\nOnly showing last 25 usernames"
		}
		return out, nil
	},
}

var cmdNicknames = &commands.YAGCommand{
	CmdCategory:     commands.CategoryTool,
	Name:            "Nicknames",
	Description:     "Shows past nicknames of a user.",
	LongDescription: "Only shows up to the last 25 nicknames.",

	Aliases: []string{"nn"},
	RunInDM: false,
	Arguments: []*dcmd.ArgDef{
		{Name: "User", Type: dcmd.User},
	},
	RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
		config, err := GetConfig(parsed.Context(), parsed.GS.ID)
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

		nicknames, err := GetNicknames(parsed.Context(), target.ID, parsed.GS.ID, 25)
		if err != nil {
			return nil, err
		}

		out := fmt.Sprintf("Past nicknames of **%s#%s** ```\n", target.Username, target.Discriminator)
		for _, v := range nicknames {
			out += fmt.Sprintf("%20s: %s\n", v.CreatedAt.Time.UTC().Format(time.RFC822), v.Nickname.String)
		}
		out += "```"
		if len(nicknames) == 25 {
			out += "\nOnly showing last 25 nicknames"
		}
		return out, nil
	},
}

// Mark all log messages with this id as deleted
func HandleMsgDelete(evt *eventsystem.EventData) {
	if evt.Type == eventsystem.EventMessageDelete {
		err := markLoggedMessageAsDeleted(evt.Context(), evt.MessageDelete().ID)
		if err != nil {
			logrus.WithError(err).Error("Failed marking message as deleted")
		}
		return
	}

	for _, m := range evt.MessageDeleteBulk().Messages {
		err := markLoggedMessageAsDeleted(evt.Context(), m)
		if err != nil {
			logrus.WithError(err).Error("Failed marking message as deleted")
		}
	}
}

func markLoggedMessageAsDeleted(ctx context.Context, mID int64) error {
	_, err := models.Messages(models.MessageWhere.MessageID.EQ(null.StringFrom(discordgo.StrID(mID)))).UpdateAllG(ctx,
		models.M{"deleted": true})
	return err
}

func HandlePresenceUpdate(evt *eventsystem.EventData) {
	pu := evt.PresenceUpdate()
	gs := bot.State.Guild(true, pu.GuildID)
	if gs == nil {
		go func() { evtChan <- pu }()
		return
	}

	gs.RLock()
	defer gs.RUnlock()

	ms := gs.Member(false, pu.User.ID)
	if ms == nil || !ms.PresenceSet || !ms.MemberSet {
		go func() { evtChan <- pu }()
		return
	}

	if pu.User.Username != "" {
		if pu.User.Username != ms.Username {
			go func() { evtChan <- pu }()
			return
		}
	}

	if pu.Nick != ms.Nick {
		go func() { evtChan <- pu }()
		return
	}
}

// While presence update is sent when user changes username.... MAKES NO SENSE IMO BUT WHATEVER
// Also check nickname incase the user came online
func HandleQueueEvt(evt *eventsystem.EventData) {
	evtChan <- evt.EvtInterface
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

func CheckUsername(exec boil.ContextExecutor, ctx context.Context, usernameStmt *sql.Stmt, user *discordgo.User) {
	var lastUsername string
	row := usernameStmt.QueryRow(user.ID)
	err := row.Scan(&lastUsername)

	if err == nil && lastUsername == user.Username {
		// Not changed
		return
	}

	logrus.Debug("[logs] User changed username, old:", lastUsername, " | new:", user.Username)

	listing := &models.UsernameListing{
		UserID:   null.Int64From(user.ID),
		Username: null.StringFrom(user.Username),
	}

	err = listing.Insert(ctx, exec, boil.Infer())
	if err != nil {
		logrus.WithError(err).WithField("user", user.ID).Error("[logs] failed setting last username")
	}
}

func CheckNickname(exec boil.ContextExecutor, ctx context.Context, nicknameStmt *sql.Stmt, userID, guildID int64, nickname string) {
	var lastNickname string
	row := nicknameStmt.QueryRow(userID, guildID)
	err := row.Scan(&lastNickname)
	if err == sql.ErrNoRows && nickname == "" {
		// don't need to be putting this in the database as the first record for the user
		return
	}

	if err == nil && lastNickname == nickname {
		// Not changed
		return
	}

	logrus.Debug("[logs] User changed nickname, old:", lastNickname, " | new:", nickname)

	listing := &models.NicknameListing{
		UserID:   null.Int64From(userID),
		GuildID:  null.StringFrom(discordgo.StrID(guildID)),
		Nickname: null.StringFrom(nickname),
	}

	err = listing.Insert(ctx, exec, boil.Infer())
	if err != nil {
		logrus.WithError(err).WithField("guild", guildID).WithField("user", userID).Error("[logs] failed setting last nickname")
	}
}

// func CheckNicknameBulk(gDB *gorm.DB, guildID int64, members []*discordgo.Member) {

// 	ids := make([]int64, 0, len(members))
// 	for _, v := range members {
// 		ids = append(ids, v.User.ID)
// 	}

// 	rows, err := gDB.CommonDB().Query(
// 		"select distinct on(user_id) nickname,user_id from nickname_listings where user_id = ANY ($1) AND guild_id=$2 order by user_id,id desc;", pq.Int64Array(ids), guildID)
// 	if err != nil {
// 		logrus.WithError(err).Error("Failed querying current nicknames")
// 	}

// 	// Value is wether the nickname was identical
// 	queriedUsers := make(map[int64]bool)

// 	for rows.Next() {
// 		var nickname string
// 		var userID int64
// 		err = rows.Scan(&nickname, &userID)
// 		if err != nil {
// 			logrus.WithError(err).Error("Error while scanning")
// 			continue
// 		}

// 		for _, member := range members {
// 			if member.User.ID == userID {
// 				if member.Nick == nickname {
// 					// Already have the last username tracked
// 					queriedUsers[userID] = true
// 				} else {
// 					queriedUsers[userID] = false
// 					logrus.Debug("CHANGED Nick: ", nickname, " : ", member.Nick)
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

// 		logrus.Debug("User changed nickname, new: ", member.Nick)

// 		listing := NicknameListing{
// 			UserID:   member.User.ID,
// 			GuildID:  discordgo.StrID(guildID),
// 			Nickname: member.Nick,
// 		}

// 		err = gDB.Create(&listing).Error
// 		if err != nil {
// 			logrus.WithError(err).Error("Failed setting nickname")
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
// 		logrus.WithError(err).Error("Failed querying current usernames")
// 	}

// 	unchangedUsers := make(map[int64]bool)

// 	for rows.Next() {
// 		var username string
// 		var userID int64
// 		err = rows.Scan(&username, &userID)
// 		if err != nil {
// 			logrus.WithError(err).Error("Error while scanning")
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

// 		logrus.Debug("User changed username, new: ", user.Username)

// 		listing := UsernameListing{
// 			UserID:   user.ID,
// 			Username: user.Username,
// 		}

// 		err = gDB.Create(&listing).Error
// 		if err != nil {
// 			logrus.WithError(err).Error("Failed setting username")
// 		}
// 	}
// }

var (
	evtChan   = make(chan interface{})
	evtChanGC = make(chan *LightGC)
)

// Queue up all the events and process them one by one, because of limited connections
func EvtProcesser() {
	for {
		e := <-evtChan

		switch t := e.(type) {
		case *discordgo.PresenceUpdate:
			conf, err := GetConfig(context.Background(), t.GuildID)
			if err != nil {
				logrus.WithError(err).Error("Failed fetching config")
				continue
			}

			if conf.NicknameLoggingEnabled.Bool {
				CheckNickname(common.PQ, context.Background(), nicknameQueryStatement, t.User.ID, t.GuildID, t.Presence.Nick)
			}

			if conf.UsernameLoggingEnabled.Bool {
				if t.User.Username != "" {
					CheckUsername(common.PQ, context.Background(), usernameQueryStatement, t.User)
				}
			}
		case *discordgo.GuildMemberUpdate:
			conf, err := GetConfig(context.Background(), t.GuildID)
			if err != nil {
				logrus.WithError(err).Error("Failed fetching config")
				continue
			}
			if conf.NicknameLoggingEnabled.Bool {
				CheckNickname(common.PQ, context.Background(), nicknameQueryStatement, t.User.ID, t.GuildID, t.Nick)
			}
		case *discordgo.GuildMemberAdd:
			conf, err := GetConfig(context.Background(), t.GuildID)
			if err != nil {
				logrus.WithError(err).Error("Failed fetching config")
				continue
			}
			if conf.UsernameLoggingEnabled.Bool {
				CheckUsername(common.PQ, context.Background(), usernameQueryStatement, t.User)
			}
		case *discordgo.Member:
			conf, err := GetConfig(context.Background(), t.GuildID)
			if err != nil {
				logrus.WithError(err).Error("Failed fetching config")
				continue
			}

			if conf.NicknameLoggingEnabled.Bool {
				CheckNickname(common.PQ, context.Background(), nicknameQueryStatement, t.User.ID, t.GuildID, t.Nick)
			}

			if conf.UsernameLoggingEnabled.Bool {
				CheckUsername(common.PQ, context.Background(), usernameQueryStatement, t.User)
			}
		}
	}
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
		// 	logrus.WithError(err).Error("Failed fetching config")
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
		// 	logrus.WithError(err).Error("Failed committing transaction")
		// 	continue
		// }

		// if len(gc.Members) > 100 {
		// 	logrus.Infof("Checked %d members in %s", len(gc.Members), time.Since(started).String())
		// 	// Make sure this dosen't use all our resources
		// 	time.Sleep(time.Second * 25)
		// } else {
		// 	time.Sleep(time.Second * 15)
		// }
	}
}
