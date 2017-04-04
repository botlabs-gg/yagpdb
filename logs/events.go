package logs

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/bwmarrin/snowflake"
	"github.com/jinzhu/gorm"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"strconv"
	"time"
)

func init() {
	// Discord epoch
	snowflake.Epoch = 1420070400000
}

func (p *Plugin) InitBot() {
	eventsystem.AddHandler(bot.ConcurrentEventHandler(HandleQueueEvt), eventsystem.EventGuildMemberUpdate, eventsystem.EventGuildMemberAdd, eventsystem.EventGuildCreate, eventsystem.EventMemberFetched)
	eventsystem.AddHandler(bot.ConcurrentEventHandler(HandleMsgDelete), eventsystem.EventMessageDelete, eventsystem.EventMessageDeleteBulk)

	eventsystem.AddHandlerBefore(HandlePresenceUpdate, eventsystem.EventPresenceUpdate, bot.StateHandlerPtr)
	commands.CommandSystem.RegisterCommands(cmds...)
}

var _ bot.BotStarterHandler = (*Plugin)(nil)

func (p *Plugin) StartBot() {
	go EvtProcesser()
}

var cmds = []commandsystem.CommandHandler{
	&commands.CustomCommand{
		Cooldown: 30,
		Category: commands.CategoryTool,
		Command: &commandsystem.Command{
			Name:        "Logs",
			Aliases:     []string{"ps", "paste", "pastebin", "log"},
			Description: "Creates a log of the channels last 100 messages",
			Arguments: []*commandsystem.ArgDef{
				&commandsystem.ArgDef{Name: "Count", Type: commandsystem.ArgumentNumber},
			},
			Run: func(cmd *commandsystem.ExecData) (interface{}, error) {
				num := 100
				if cmd.Args[0] != nil {
					c := cmd.Args[0].Int()
					if c > 250 {
						return "Count can be max 250", nil
					} else if c < 1 {
						return "Why do you want me to log below 1 messages? e.e", nil
					} else {
						num = c
					}
				}

				l, err := CreateChannelLog(cmd.Channel.ID(), cmd.Message.Author.Username, cmd.Message.Author.ID, num)
				if err != nil {
					return "An error occured", err
				}

				return l.Link(), err
			},
		},
	},
	&commands.CustomCommand{
		Cooldown: 10,
		Category: commands.CategoryTool,
		Command: &commandsystem.Command{
			Name:        "Whois",
			Description: "shows infromation about a user",
			Aliases:     []string{"whoami"},
			RunInDm:     false,
			Arguments: []*commandsystem.ArgDef{
				{Name: "User", Type: commandsystem.ArgumentUser},
			},
			Run: func(parsed *commandsystem.ExecData) (interface{}, error) {
				config, err := GetConfig(parsed.Guild.ID())
				if err != nil {
					return "AAAAA", err
				}

				target := parsed.Message.Author
				if parsed.Args[0] != nil {
					target = parsed.Args[0].DiscordUser()
				}

				member, err := bot.GetMember(parsed.Guild.ID(), target.ID)
				if err != nil {
					return "An error occured fetching guild member, contact bot owner", err
				}

				nick := ""
				if member.Nick != "" {
					nick = " (" + member.Nick + ")"
				}

				joinedAtStr := ""
				joinedAtDurStr := ""
				joinedAt, err := discordgo.Timestamp(member.JoinedAt).Parse()
				if err != nil {
					joinedAtStr = "Uh oh something baddy happening parsing time"
					logrus.WithError(err).Error("Failed parsing joinedat")
				} else {
					joinedAtStr = joinedAt.UTC().Format(time.RFC822)
					dur := time.Since(joinedAt)
					joinedAtDurStr = common.HumanizeDuration(common.DurationPrecisionHours, dur)
				}
				if joinedAtDurStr == "" {
					joinedAtDurStr = "Lesss than an hour ago"
				}

				parsedId, _ := strconv.ParseInt(target.ID, 10, 64)
				flake := snowflake.ID(parsedId)
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
							Value:  target.ID,
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

					nicknames, err := GetNicknames(target.ID, parsed.Guild.ID(), 5)
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
		},
	},
	&commands.CustomCommand{
		Cooldown: 10,
		Category: commands.CategoryTool,
		Command: &commandsystem.Command{
			Name:        "Usernames",
			Description: "Shows past usernames of a user",
			Aliases:     []string{"unames", "un"},
			RunInDm:     true,
			Arguments: []*commandsystem.ArgDef{
				{Name: "User", Type: commandsystem.ArgumentUser},
			},
			Run: func(parsed *commandsystem.ExecData) (interface{}, error) {
				config, err := GetConfig(parsed.Guild.ID())
				if err != nil {
					return "AAAAA", err
				}

				target := parsed.Message.Author
				if parsed.Args[0] != nil {
					target = parsed.Args[0].DiscordUser()
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
		},
	},
	&commands.CustomCommand{
		Cooldown: 10,
		Category: commands.CategoryTool,
		Command: &commandsystem.Command{
			Name:        "Nicknames",
			Description: "Shows past nicknames of a user",
			Aliases:     []string{"nn"},
			RunInDm:     false,
			Arguments: []*commandsystem.ArgDef{
				{Name: "User", Type: commandsystem.ArgumentUser},
			},
			Run: func(parsed *commandsystem.ExecData) (interface{}, error) {
				config, err := GetConfig(parsed.Guild.ID())
				if err != nil {
					return "AAAAA", err
				}

				target := parsed.Message.Author
				if parsed.Args[0] != nil {
					target = parsed.Args[0].DiscordUser()
				}

				if !config.NicknameLoggingEnabled {
					return "Nickname logging is disabled on this server", nil
				}

				nicknames, err := GetNicknames(target.ID, parsed.Guild.ID(), 25)
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
		},
	},
}

// Mark all log messages with this id as deleted
func HandleMsgDelete(evt *eventsystem.EventData) {
	if evt.MessageDelete != nil {
		err := markLoggedMessageAsDeleted(evt.MessageDelete.ID)
		if err != nil {
			logrus.WithError(err).Error("Failed marking message as deleted")
		}
		return
	}

	for _, m := range evt.MessageDeleteBulk.Messages {
		err := markLoggedMessageAsDeleted(m)
		if err != nil {
			logrus.WithError(err).Error("Failed marking message as deleted")
		}
	}
}

func markLoggedMessageAsDeleted(mID string) error {
	return common.SQL.Model(Message{}).Where("message_id = ?", mID).Update("deleted", true).Error
}

func HandlePresenceUpdate(evt *eventsystem.EventData) {
	pu := evt.PresenceUpdate
	gs := bot.State.Guild(true, pu.GuildID)
	if gs == nil {
		go func() { evtChan <- evt }()
	}
	ms := gs.Member(true, pu.User.ID)
	if ms == nil || ms.Presence == nil || ms.Member == nil {
		go func() { evtChan <- evt }()
		return
	}

	gs.RLock()
	defer gs.RUnlock()

	if pu.User.Username != "" {
		if pu.User.Username != ms.Member.User.Username {
			go func() { evtChan <- evt }()
			return
		}
	}

	if pu.Nick != ms.Presence.Nick {
		go func() { evtChan <- evt }()
		return
	}
}

// While presence update is sent when user changes username.... MAKES NO SENSE IMO BUT WHATEVER
// Also check nickname incase the user came online
func HandleQueueEvt(evt *eventsystem.EventData) {
	evtChan <- evt.EvtInterface
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

func CheckUsername(user *discordgo.User) {
	var result UsernameListing
	err := common.SQL.Model(&result).Where(UsernameListing{UserID: MustParseID(user.ID)}).Last(&result).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		logrus.WithError(err).Error("Failed checking username for changes")
		return
	}

	if err == nil && result.Username == user.Username {
		return
	}

	logrus.Info("User changed username, old:", result.Username, "new:", user.Username)

	listing := UsernameListing{
		UserID:   MustParseID(user.ID),
		Username: user.Username,
	}

	err = common.SQL.Create(&listing).Error
	if err != nil {
		logrus.WithError(err).Error("Failed setting username")
	}
}

func CheckNickname(userID, guildID, nickname string) {
	var result NicknameListing
	err := common.SQL.Model(&result).Where(NicknameListing{UserID: MustParseID(userID), GuildID: guildID}).Last(&result).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		logrus.WithError(err).Error("Failed checking nickname for changes")
		return
	}

	if err == gorm.ErrRecordNotFound && nickname == "" {
		// don't need to be putting this in the database as the first record for the user
		return
	}

	if err == nil && result.Nickname == nickname {
		return
	}

	logrus.Info("User changed nickname, old:", result.Nickname, "new:", nickname)

	listing := NicknameListing{
		UserID:   MustParseID(userID),
		GuildID:  guildID,
		Nickname: nickname,
	}

	err = common.SQL.Create(&listing).Error
	if err != nil {
		logrus.WithError(err).Error("Failed setting nickname")
	}
}

var (
	evtChan = make(chan interface{})
)

// Queue up all the events and process them one by one, because of limited connections
func EvtProcesser() {
	for {
		e := <-evtChan

		switch t := e.(type) {
		case *discordgo.GuildCreate:
			conf, err := GetConfig(t.ID)
			if err != nil {
				logrus.WithError(err).Error("Failed fetching config")
				continue
			}

			started := time.Now()
			for _, v := range t.Members {
				if conf.NicknameLoggingEnabled {
					CheckNickname(v.User.ID, t.Guild.ID, v.Nick)
				}
				if conf.UsernameLoggingEnabled {
					CheckUsername(v.User)
				}
			}
			logrus.Infof("Checked %d members in %s", len(t.Members), time.Since(started).String())
		case *discordgo.PresenceUpdate:
			conf, err := GetConfig(t.GuildID)
			if err != nil {
				logrus.WithError(err).Error("Failed fetching config")
				continue
			}

			if conf.NicknameLoggingEnabled {
				CheckNickname(t.User.ID, t.GuildID, t.Presence.Nick)
			}

			if conf.UsernameLoggingEnabled {
				if t.User.Username != "" {
					CheckUsername(t.User)
				}
			}
		case *discordgo.GuildMemberUpdate:
			conf, err := GetConfig(t.GuildID)
			if err != nil {
				logrus.WithError(err).Error("Failed fetching config")
				continue
			}
			if conf.NicknameLoggingEnabled {
				CheckNickname(t.User.ID, t.GuildID, t.Nick)
			}
		case *discordgo.GuildMemberAdd:
			conf, err := GetConfig(t.GuildID)
			if err != nil {
				logrus.WithError(err).Error("Failed fetching config")
				continue
			}
			if conf.UsernameLoggingEnabled {
				CheckUsername(t.User)
			}
		case *discordgo.Member:
			conf, err := GetConfig(t.GuildID)
			if err != nil {
				logrus.WithError(err).Error("Failed fetching config")
				continue
			}

			if conf.NicknameLoggingEnabled {
				CheckNickname(t.User.ID, t.GuildID, t.Nick)
			}

			if conf.UsernameLoggingEnabled {
				CheckUsername(t.User)
			}
		}
	}
}
