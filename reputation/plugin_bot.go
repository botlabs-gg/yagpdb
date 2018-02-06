package reputation

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix.v2/redis"
	"regexp"
	"strconv"
)

func (p *Plugin) InitBot() {
	commands.AddRootCommands(cmds...)
	eventsystem.AddHandler(bot.RedisWrapper(handleMessageCreate), eventsystem.EventMessageCreate)
}

var thanksRegex = regexp.MustCompile(`(?i)( |\n|^)(thanks?|danks|ty|thx|\+rep|\+ ?\<\@[0-9]*\>)( |\n|$)`)

func handleMessageCreate(evt *eventsystem.EventData) {
	msg := evt.MessageCreate
	client := bot.ContextRedis(evt.Context())

	if len(msg.Mentions) < 1 {
		return
	}

	if !thanksRegex.MatchString(evt.MessageCreate.Content) {
		return
	}

	who := msg.Mentions[0]
	if who.ID == msg.Author.ID {
		return
	}

	cs := bot.State.Channel(true, msg.ChannelID)
	if cs.IsPrivate() {
		return
	}

	conf, err := GetConfig(cs.Guild.ID())
	if err != nil || !conf.Enabled {
		return
	}

	target, err := bot.GetMember(cs.Guild.ID(), who.ID)
	sender, err2 := bot.GetMember(cs.Guild.ID(), msg.Author.ID)
	if err != nil || err2 != nil {
		if err2 != nil {
			err = err2
		}

		logrus.WithError(err).Error("Failed retrieving bot member")
		return
	}

	if err = CanModifyRep(conf, sender, target); err != nil {
		return
	}

	err = ModifyRep(conf, client, cs.Guild.ID(), sender, target, 1)
	if err != nil {
		if err == ErrCooldown {
			// Ignore this error silently
			return
		}
		logrus.WithError(err).Error("Failed giving rep")
		return
	}

	content := fmt.Sprintf("Gave +1 %s to **%s**", conf.PointsName, who.Username)
	common.BotSession.ChannelMessageSend(msg.ChannelID, common.EscapeSpecialMentions(content))
}

var cmds = []*commands.YAGCommand{
	&commands.YAGCommand{
		CmdCategory:  commands.CategoryFun,
		Name:         "TakeRep",
		Aliases:      []string{"-", "tr", "trep"},
		Description:  "Takes away rep from someone",
		RequiredArgs: 1,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "User", Type: dcmd.User},
			&dcmd.ArgDef{Name: "Num", Type: dcmd.Int, Default: 1},
		},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			parsed.Args[1].Value = -parsed.Args[1].Int()
			return CmdGiveRep(parsed)
		},
	},
	&commands.YAGCommand{
		CmdCategory:  commands.CategoryFun,
		Name:         "GiveRep",
		Aliases:      []string{"+", "gr", "grep"},
		Description:  "Gives rep to someone",
		RequiredArgs: 1,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "User", Type: dcmd.User},
			&dcmd.ArgDef{Name: "Num", Type: dcmd.Int, Default: 1},
		},
		RunFunc: CmdGiveRep,
	},
	&commands.YAGCommand{
		CmdCategory:  commands.CategoryFun,
		Name:         "SetRep",
		Description:  "Sets someones rep, this is an admin command and bypasses cooldowns and other restrictions.",
		RequiredArgs: 2,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "User", Type: dcmd.User},
			&dcmd.ArgDef{Name: "Num", Type: dcmd.Int},
		},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			conf, err := GetConfig(parsed.GS.ID())
			if err != nil {
				return "An error occured while finding the server config", err
			}

			member, _ := bot.GetMember(parsed.GS.ID(), parsed.Msg.Author.ID)
			parsed.GS.RLock()

			if !IsAdmin(parsed.GS, member, conf) {
				parsed.GS.RUnlock()
				return "You're not an reputation admin. (no manage servers perms and no rep admin role)", nil
			}
			parsed.GS.RUnlock()

			target := parsed.Args[0].Value.(*discordgo.User)
			err = SetRep(common.MustParseInt(parsed.GS.ID()), common.MustParseInt(member.User.ID), common.MustParseInt(target.ID), int64(parsed.Args[1].Int()))
			if err != nil {
				return "Failed setting rep, contact bot owner", err
			}

			return fmt.Sprintf("Set **%s** %s to `%d`", target.Username, conf.PointsName, parsed.Args[1].Int()), nil
		},
	},
	&commands.YAGCommand{
		CmdCategory: commands.CategoryFun,
		Name:        "Rep",
		Description: "Shows yours or the specified users current rep and rank",
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "User", Type: dcmd.User},
		},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			target := parsed.Msg.Author
			if parsed.Args[0].Value != nil {
				target = parsed.Args[0].Value.(*discordgo.User)
			}

			conf, err := GetConfig(parsed.GS.ID())
			if err != nil {
				return "An error occured finding the server config", err
			}

			score, rank, err := GetUserStats(parsed.GS.ID(), target.ID)

			if err != nil {
				if err == ErrUserNotFound {
					rank = -1
				} else {
					return "Error retrieving stats", err
				}
			}

			rankStr := "#ω"
			if rank != -1 {
				rankStr = strconv.FormatInt(int64(rank), 10)
			}

			return fmt.Sprintf("**%s**: **%d** %s (#**%s**)", target.Username, score, conf.PointsName, rankStr), nil
		},
	},
	&commands.YAGCommand{
		CmdCategory: commands.CategoryFun,
		Name:        "TopRep",
		Description: "Shows top 15 rep on the server",
		Arguments: []*dcmd.ArgDef{
			{Name: "Offset", Type: dcmd.Int, Default: 0},
		},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			offset := parsed.Args[0].Int()

			entries, err := TopUsers(parsed.GS.ID(), offset, 15)
			if err != nil {
				return "Something went wrong... i may hae had one too many alcohol", err
			}

			detailed, err := DetailedLeaderboardEntries(parsed.GS.ID(), entries)
			if err != nil {
				return "Failed filling in the detalis of the leaderboard entries", err
			}

			leaderboardURL := "https://" + common.Conf.Host + "/public/" + parsed.GS.ID() + "/reputation/leaderboard"
			out := "```\n# -- Points -- User\n"
			for _, v := range detailed {
				user := v.Username
				if user == "" {
					user = "unknown ID:" + strconv.FormatInt(v.UserID, 10)
				}
				out += fmt.Sprintf("#%02d: %6d - %s\n", v.Rank, v.Points, user)
			}
			out += "```\n" + "Full leaderboard: <" + leaderboardURL + ">"

			return out, nil
		},
	},
}

func CmdGiveRep(parsed *dcmd.Data) (interface{}, error) {
	target := parsed.Args[0].Value.(*discordgo.User)

	conf, err := GetConfig(parsed.GS.ID())
	if err != nil {
		return "An error occured finding the server config", err
	}

	pointsName := conf.PointsName

	if target.ID == parsed.Msg.Author.ID {
		return fmt.Sprintf("Can't give yourself %s... **Silly**", pointsName), nil
	}

	client := parsed.Context().Value(commands.CtxKeyRedisClient).(*redis.Client)

	sender, err := bot.GetMember(parsed.GS.ID(), parsed.Msg.Author.ID)
	receiver, err2 := bot.GetMember(parsed.GS.ID(), target.ID)
	if err != nil || err2 != nil {
		if err2 != nil {
			err = err2
		}

		return "Failed retreiving members", err
	}

	amount := parsed.Args[1].Int()

	err = ModifyRep(conf, client, parsed.GS.ID(), sender, receiver, int64(amount))
	if err != nil {
		if cast, ok := err.(UserError); ok {
			return cast, nil
		}

		return "Failed modifying your " + pointsName, err
	}

	newScore, newRank, err := GetUserStats(parsed.GS.ID(), target.ID)
	if err != nil {
		newScore = -1
		newRank = -1
		logrus.WithError(err).WithField("guild", parsed.GS.ID()).Error("Failed getting user reputation stats")
	}

	actionStr := ""
	targetStr := "to"
	if amount > 0 {
		actionStr = "Gave"
	} else {
		actionStr = "Took away"
		amount = -amount
		targetStr = "from"
	}

	msg := fmt.Sprintf("%s `%d` %s %s **%s** (current: `#%d` - `%d`)", actionStr, amount, pointsName, targetStr, target.Username, newRank, newScore)
	return msg, nil
}
