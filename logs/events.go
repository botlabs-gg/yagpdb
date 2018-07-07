package logs

import (
	"database/sql"
	"fmt"
	"github.com/bwmarrin/snowflake"
	"github.com/jinzhu/gorm"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/sirupsen/logrus"
	"runtime"
	"time"
)

var (
	nicknameQueryStatement *sql.Stmt
	usernameQueryStatement *sql.Stmt
)

func init() {
	// Discord epoch
	snowflake.Epoch = 1420070400000
}

func (p *Plugin) InitBot() {
	eventsystem.AddHandler(bot.ConcurrentEventHandler(HandleQueueEvt), eventsystem.EventGuildMemberUpdate, eventsystem.EventGuildMemberAdd, eventsystem.EventMemberFetched)
	eventsystem.AddHandler(bot.ConcurrentEventHandler(HandleGC), eventsystem.EventGuildCreate)
	eventsystem.AddHandler(bot.ConcurrentEventHandler(HandleMsgDelete), eventsystem.EventMessageDelete, eventsystem.EventMessageDeleteBulk)

	eventsystem.AddHandlerBefore(HandlePresenceUpdate, eventsystem.EventPresenceUpdate, bot.StateHandlerPtr)

	commands.AddRootCommands(cmdLogs, cmdWhois, cmdNicknames, cmdUsernames)

	var err error
	nicknameQueryStatement, err = common.PQ.Prepare("select nickname from nickname_listings where user_id=$1 AND guild_id=$2 order by id desc limit 1;")
	if err != nil {
		panic("Failed preparing statement: " + err.Error())
	}

	usernameQueryStatement, err = common.PQ.Prepare("select username from username_listings where user_id=$1 order by id desc limit 1;")
	if err != nil {
		panic("Failed preparing statement: " + err.Error())
	}
}

var _ bot.BotStarterHandler = (*Plugin)(nil)

func (p *Plugin) StartBot() {
	go EvtProcesser()
	go EvtProcesserGCs()
}

var cmdLogs = &commands.YAGCommand{
	Cooldown:        5,
	CmdCategory:     commands.CategoryTool,
	Name:            "Logs",
	Aliases:         []string{"log"},
	Description:     "Creates a log of the last messages in the current channel",
	LongDescription: "This includes deleted messages within an hour",
	Arguments: []*dcmd.ArgDef{
		&dcmd.ArgDef{Name: "Count", Default: 100, Type: &dcmd.IntArg{Min: 2, Max: 250}},
	},
	RunFunc: func(cmd *dcmd.Data) (interface{}, error) {
		num := cmd.Args[0].Int()

		l, err := CreateChannelLog(nil, cmd.GS.ID(), cmd.CS.ID, cmd.Msg.Author.Username, cmd.Msg.Author.ID, num)
		if err != nil {
			if err == ErrChannelBlacklisted {
				return "This channel is blacklisted from creating message logs, this can be changed in the control panel.", nil
			}
			return "An error occured", err
		}

		return l.Link(), err
	},
}

var cmdWhois = &commands.YAGCommand{
	CmdCategory: commands.CategoryTool,
	Name:        "Whois",
	Description: "shows information about a user",
	Aliases:     []string{"whoami"},
	RunInDM:     false,
	Arguments: []*dcmd.ArgDef{
		{Name: "User", Type: dcmd.User},
	},
	RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
		config, err := GetConfig(parsed.GS.ID())
		if err != nil {
			return "Failed retrieving config for this server", err
		}

		target := parsed.Msg.Author
		if parsed.Args[0].Value != nil {
			target = parsed.Args[0].Value.(*discordgo.User)
		}

		member, err := bot.GetMember(parsed.GS.ID(), target.ID)
		if err != nil {
			return "An error occured fetching guild member, contact bot owner", err
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

		flake := snowflake.ID(target.ID)
		t := time.Unix(flake.Time()/1000, 0)
		createdDurStr := common.HumanizeDuration(common.DurationPrecisionHours, time.Since(t))
		if createdDurStr == "" {
			createdDurStr = "Less than an hour ago"
		}
		embed := &discordgo.MessageEmbed{
			Title: fmt.Sprintf("%s#%s%s", target.Username, target.Discriminator, nick),
			Fields: []*discordgo.MessageEmbedField{
				&discordgo.MessageEmbedField{
					Name:   "ID",
					Value:  discordgo.StrID(target.ID),
					Inline: true,
				},
				&discordgo.MessageEmbedField{
					Name:   "Avatar",
					Value:  "[Link](" + discordgo.EndpointUserAvatar(target.ID, target.Avatar) + ")",
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
				URL: discordgo.EndpointUserAvatar(target.ID, target.Avatar),
			},
		}

		if config.UsernameLoggingEnabled {
			usernames, err := GetUsernames(target.ID, 5)
			if err != nil {
				return err, err
			}

			usernamesStr := "```\n"
			for _, v := range usernames {
				usernamesStr += fmt.Sprintf("%20s: %s\n", v.CreatedAt.UTC().Format(time.RFC822), v.Username)
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

		if config.NicknameLoggingEnabled {

			nicknames, err := GetNicknames(target.ID, parsed.GS.ID(), 5)
			if err != nil {
				return err, err
			}

			nicknameStr := "```\n"
			if len(nicknames) < 1 {
				nicknameStr += "No nicknames tracked"
			} else {
				for _, v := range nicknames {
					nicknameStr += fmt.Sprintf("%20s: %s\n", v.CreatedAt.UTC().Format(time.RFC822), v.Nickname)
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
	Description: "Shows past usernames of a user",
	Aliases:     []string{"unames", "un"},
	RunInDM:     true,
	Arguments: []*dcmd.ArgDef{
		{Name: "User", Type: dcmd.User},
	},
	RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
		config, err := GetConfig(parsed.GS.ID())
		if err != nil {
			return "AAAAA", err
		}

		target := parsed.Msg.Author
		if parsed.Args[0].Value != nil {
			target = parsed.Args[0].Value.(*discordgo.User)
		}

		if !config.UsernameLoggingEnabled {
			return "Username logging is disabled on this server", nil
		}

		usernames, err := GetUsernames(target.ID, 25)
		if err != nil {
			return err, err
		}

		out := fmt.Sprintf("Past username of **%s#%s** ```\n", target.Username, target.Discriminator)
		for _, v := range usernames {
			out += fmt.Sprintf("%20s: %s\n", v.CreatedAt.UTC().Format(time.RFC822), v.Username)
		}
		out += "```"
		if len(usernames) == 25 {
			out += "\nOnly showing last 25 usernames"
		}
		return out, nil
	},
}

var cmdNicknames = &commands.YAGCommand{
	CmdCategory: commands.CategoryTool,
	Name:        "Nicknames",
	Description: "Shows past nicknames of a user",
	Aliases:     []string{"nn"},
	RunInDM:     false,
	Arguments: []*dcmd.ArgDef{
		{Name: "User", Type: dcmd.User},
	},
	RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
		config, err := GetConfig(parsed.GS.ID())
		if err != nil {
			return "AAAAA", err
		}

		target := parsed.Msg.Author
		if parsed.Args[0].Value != nil {
			target = parsed.Args[0].Value.(*discordgo.User)
		}

		if !config.NicknameLoggingEnabled {
			return "Nickname logging is disabled on this server", nil
		}

		nicknames, err := GetNicknames(target.ID, parsed.GS.ID(), 25)
		if err != nil {
			return err, err
		}

		out := fmt.Sprintf("Past nicknames of **%s#%s** ```\n", target.Username, target.Discriminator)
		for _, v := range nicknames {
			out += fmt.Sprintf("%20s: %s\n", v.CreatedAt.UTC().Format(time.RFC822), v.Nickname)
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
		err := markLoggedMessageAsDeleted(evt.MessageDelete().ID)
		if err != nil {
			logrus.WithError(err).Error("Failed marking message as deleted")
		}
		return
	}

	for _, m := range evt.MessageDeleteBulk().Messages {
		err := markLoggedMessageAsDeleted(m)
		if err != nil {
			logrus.WithError(err).Error("Failed marking message as deleted")
		}
	}
}

func markLoggedMessageAsDeleted(mID int64) error {
	return common.GORM.Model(Message{}).Where("message_id = ?", mID).Update("deleted", true).Error
}

func HandlePresenceUpdate(evt *eventsystem.EventData) {
	pu := evt.PresenceUpdate()
	gs := bot.State.Guild(true, pu.GuildID)
	if gs == nil {
		go func() { evtChan <- evt }()
		return
	}

	gs.RLock()
	ms := gs.Member(false, pu.User.ID)
	if ms == nil || !ms.PresenceSet || !ms.MemberSet {
		gs.RUnlock()
		go func() { evtChan <- evt }()
		return
	}

	if pu.User.Username != "" {
		if pu.User.Username != ms.Username {
			gs.RUnlock()
			go func() { evtChan <- evt }()
			return
		}
	}

	if pu.Nick != ms.Nick {
		gs.RUnlock()
		go func() { evtChan <- evt }()
		return
	}

	gs.RUnlock()
}

// While presence update is sent when user changes username.... MAKES NO SENSE IMO BUT WHATEVER
// Also check nickname incase the user came online
func HandleQueueEvt(evt *eventsystem.EventData) {
	evtChan <- evt.EvtInterface
}

func HandleGC(evt *eventsystem.EventData) {
	evtChanGC <- evt.EvtInterface
}

type UsernameListing struct {
	gorm.Model
	UserID   int64 `gorm:"index"`
	Username string
}

type NicknameListing struct {
	gorm.Model
	UserID   int64 `gorm:"index"`
	GuildID  string
	Nickname string
}

func CheckUsername(gDB *gorm.DB, usernameStmt *sql.Stmt, user *discordgo.User) {
	var lastUsername string
	row := usernameStmt.QueryRow(user.ID)
	err := row.Scan(&lastUsername)
	if err == nil && lastUsername == user.Username {
		// Not changed
		return
	}

	logrus.Debug("User changed username, old:", lastUsername, " | new:", user.Username)

	listing := UsernameListing{
		UserID:   user.ID,
		Username: user.Username,
	}

	err = gDB.Create(&listing).Error
	if err != nil {
		logrus.WithError(err).Error("Failed setting username")
	}
}

func CheckNickname(gDB *gorm.DB, nicknameStmt *sql.Stmt, userID, guildID int64, nickname string) {
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

	logrus.Debug("User changed nickname, old:", lastNickname, " | new:", nickname)

	listing := NicknameListing{
		UserID:   userID,
		GuildID:  discordgo.StrID(guildID),
		Nickname: nickname,
	}

	err = gDB.Create(&listing).Error
	if err != nil {
		logrus.WithError(err).Error("Failed setting nickname")
	}
}

var (
	evtChan   = make(chan interface{})
	evtChanGC = make(chan interface{})
)

// Queue up all the events and process them one by one, because of limited connections
func EvtProcesser() {
	for {
		e := <-evtChan

		switch t := e.(type) {
		case *discordgo.PresenceUpdate:
			conf, err := GetConfig(t.GuildID)
			if err != nil {
				logrus.WithError(err).Error("Failed fetching config")
				continue
			}

			if conf.NicknameLoggingEnabled {
				CheckNickname(common.GORM, nicknameQueryStatement, t.User.ID, t.GuildID, t.Presence.Nick)
			}

			if conf.UsernameLoggingEnabled {
				if t.User.Username != "" {
					CheckUsername(common.GORM, usernameQueryStatement, t.User)
				}
			}
		case *discordgo.GuildMemberUpdate:
			conf, err := GetConfig(t.GuildID)
			if err != nil {
				logrus.WithError(err).Error("Failed fetching config")
				continue
			}
			if conf.NicknameLoggingEnabled {
				CheckNickname(common.GORM, nicknameQueryStatement, t.User.ID, t.GuildID, t.Nick)
			}
		case *discordgo.GuildMemberAdd:
			conf, err := GetConfig(t.GuildID)
			if err != nil {
				logrus.WithError(err).Error("Failed fetching config")
				continue
			}
			if conf.UsernameLoggingEnabled {
				CheckUsername(common.GORM, usernameQueryStatement, t.User)
			}
		case *discordgo.Member:
			conf, err := GetConfig(t.GuildID)
			if err != nil {
				logrus.WithError(err).Error("Failed fetching config")
				continue
			}

			if conf.NicknameLoggingEnabled {
				CheckNickname(common.GORM, nicknameQueryStatement, t.User.ID, t.GuildID, t.Nick)
			}

			if conf.UsernameLoggingEnabled {
				CheckUsername(common.GORM, usernameQueryStatement, t.User)
			}
		}
	}
}

func EvtProcesserGCs() {
	for {
		e := <-evtChanGC

		switch t := e.(type) {
		case *discordgo.GuildCreate:
			tx := common.GORM.Begin()

			rawTX := tx.CommonDB().(*sql.Tx)
			nickStmt := rawTX.Stmt(nicknameQueryStatement)
			usernameStmt := rawTX.Stmt(usernameQueryStatement)
			// nickStmt := rawTX.Stmt(rawTX)

			conf, err := GetConfig(t.ID)
			if err != nil {
				logrus.WithError(err).Error("Failed fetching config")
				continue
			}

			started := time.Now()
			for _, v := range t.Members {
				if conf.NicknameLoggingEnabled {
					CheckNickname(tx, nickStmt, v.User.ID, t.Guild.ID, v.Nick)
				}
				if conf.UsernameLoggingEnabled {
					CheckUsername(tx, usernameStmt, v.User)
				}
			}

			err = tx.Commit().Error
			if err != nil {
				logrus.WithError(err).Error("Failed committing transaction")
				continue
			}

			if len(t.Members) > 100 {
				logrus.Infof("Checked %d members in %s", len(t.Members), time.Since(started).String())
				// Make sure this dosen't use all our resources
				time.Sleep(time.Millisecond * 10)
			} else {
				runtime.Gosched()
			}
		}
	}
}
