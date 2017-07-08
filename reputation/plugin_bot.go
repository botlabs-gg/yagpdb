package reputation

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"regexp"
	"strconv"
)

func (p *Plugin) InitBot() {
	commands.CommandSystem.RegisterCommands(cmds...)
	eventsystem.AddHandler(bot.RedisWrapper(handleMessageCreate), eventsystem.EventMessageCreate)
}

var thanksRegex = regexp.MustCompile(`(?i)^(thanks?|danks|ty|thx|\+rep|\+\ ?\<\@)`)

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
	common.BotSession.ChannelMessageSend(msg.ChannelID, common.EscapeEveryoneMention(content))
}

var cmds = []commandsystem.CommandHandler{
	&commands.CustomCommand{
		Category: commands.CategoryFun,
		Command: &commandsystem.Command{
			Name:         "TakeRep",
			Aliases:      []string{"-", "tr", "trep"},
			Description:  "Takes away rep from someone",
			RequiredArgs: 1,
			Arguments: []*commandsystem.ArgDef{
				&commandsystem.ArgDef{Name: "User", Type: commandsystem.ArgumentUser},
				&commandsystem.ArgDef{Name: "Num", Type: commandsystem.ArgumentNumber, Default: float64(-1)},
			},
			Run: func(parsed *commandsystem.ExecData) (interface{}, error) {
				parsed.Args[1].Parsed = -parsed.Args[1].Parsed.(float64)
				return CmdGiveRep(parsed)
			},
		},
	},
	&commands.CustomCommand{
		Category: commands.CategoryFun,
		Command: &commandsystem.Command{
			Name:         "GiveRep",
			Aliases:      []string{"+", "gr", "grep"},
			Description:  "Gives rep to someone",
			RequiredArgs: 1,
			Arguments: []*commandsystem.ArgDef{
				&commandsystem.ArgDef{Name: "User", Type: commandsystem.ArgumentUser},
				&commandsystem.ArgDef{Name: "Num", Type: commandsystem.ArgumentNumber, Default: float64(1)},
			},
			Run: CmdGiveRep,
		},
	},
	&commands.CustomCommand{
		Category: commands.CategoryFun,
		Command: &commandsystem.Command{
			Name:        "Rep",
			Description: "Shows yours or the specified users current rep and rank",
			Arguments: []*commandsystem.ArgDef{
				&commandsystem.ArgDef{Name: "User", Type: commandsystem.ArgumentUser},
			},
			Run: func(parsed *commandsystem.ExecData) (interface{}, error) {
				target := parsed.Message.Author
				if parsed.Args[0] != nil {
					target = parsed.Args[0].DiscordUser()
				}

				conf, err := GetConfig(parsed.Guild.ID())
				if err != nil {
					return "An error occured finding the server config", err
				}

				score, rank, err := GetUserStats(parsed.Guild.ID(), target.ID)

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
	},
	&commands.CustomCommand{
		Category: commands.CategoryFun,
		Command: &commandsystem.Command{
			Name:        "TopRep",
			Description: "Shows top 15 rep on the server",
			Arguments: []*commandsystem.ArgDef{
				{Name: "Offset", Type: commandsystem.ArgumentNumber},
			},
			Run: func(parsed *commandsystem.ExecData) (interface{}, error) {
				offset := 0
				if parsed.Args[0] != nil {
					offset = parsed.Args[0].Int()
				}

				entries, err := TopUsers(parsed.Guild.ID(), offset, 15)
				if err != nil {
					return "Something went wrong... i may hae had one too many alcohol", err
				}

				detailed, err := DetailedLeaderboardEntries(entries)
				if err != nil {
					return "Failed filling in the detalis of the leaderboard entries", err
				}

				leaderboardURL := "https://" + common.Conf.Host + "/public/" + parsed.Guild.ID() + "/reputation/leaderboard"
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
	},
}

func CmdGiveRep(parsed *commandsystem.ExecData) (interface{}, error) {
	target := parsed.Args[0].DiscordUser()

	conf, err := GetConfig(parsed.Guild.ID())
	if err != nil {
		return "An error occured finding the server config", err
	}

	pointsName := conf.PointsName

	if target.ID == parsed.Message.Author.ID {
		return fmt.Sprintf("Can't give yourself %s... **Silly**", pointsName), nil
	}

	client := parsed.Context().Value(commands.CtxKeyRedisClient).(*redis.Client)

	sender, err := bot.GetMember(parsed.Guild.ID(), parsed.Message.Author.ID)
	receiver, err2 := bot.GetMember(parsed.Guild.ID(), target.ID)
	if err != nil || err2 != nil {
		if err2 != nil {
			err = err2
		}

		return "Failed retreiving members", err
	}

	amount := parsed.Args[1].Int()

	err = ModifyRep(conf, client, parsed.Guild.ID(), sender, receiver, int64(amount))
	if err != nil {
		if cast, ok := err.(UserError); ok {
			return cast, nil
		}

		return "Failed modifying your " + pointsName, err
	}

	newScore, newRank, err := GetUserStats(parsed.Guild.ID(), target.ID)
	if err != nil {
		newScore = -1
		newRank = -1
		logrus.WithError(err).WithField("guild", parsed.Guild.ID()).Error("Failed getting user reputation stats")
	}

	actionStr := ""
	if amount > 0 {
		actionStr = "Gave"
	} else {
		actionStr = "Took away"
		amount = -amount
	}

	msg := fmt.Sprintf("%s `%d` %s from **%s** (current: `#%d` - `%d`)", actionStr, amount, pointsName, target.Username, newRank, newScore)
	return msg, nil
}
