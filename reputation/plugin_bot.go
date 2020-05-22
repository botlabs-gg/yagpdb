package reputation

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/analytics"
	"github.com/jonas747/yagpdb/bot/paginatedmessages"

	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/reputation/models"
	"github.com/jonas747/yagpdb/web"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

var _ bot.BotInitHandler = (*Plugin)(nil)
var _ commands.CommandProvider = (*Plugin)(nil)

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(p, cmds...)
}

func (p *Plugin) BotInit() {
	eventsystem.AddHandlerAsyncLastLegacy(p, handleMessageCreate, eventsystem.EventMessageCreate)
}

var thanksRegex = regexp.MustCompile(`(?i)( |\n|^)(thanks?\pP*|danks|ty|thx|\+rep|\+ ?\<\@[0-9]*\>)( |\n|$)`)

func handleMessageCreate(evt *eventsystem.EventData) {
	msg := evt.MessageCreate()

	if !bot.IsNormalUserMessage(msg.Message) {
		return
	}

	if len(msg.Mentions) < 1 || msg.GuildID == 0 || msg.Author.Bot {
		return
	}

	if !thanksRegex.MatchString(msg.Content) {
		return
	}

	who := msg.Mentions[0]
	if who.ID == msg.Author.ID {
		return
	}

	if !evt.HasFeatureFlag(featureFlagThanksEnabled) {
		return
	}

	conf, err := GetConfig(evt.Context(), msg.GuildID)
	if err != nil || !conf.Enabled || conf.DisableThanksDetection {
		return
	}

	target, err := bot.GetMember(msg.GuildID, who.ID)
	sender := dstate.MSFromDGoMember(evt.GS, msg.Member)
	if err != nil {
		logger.WithError(err).Error("Failed retrieving target member")
		return
	}

	if err = CanModifyRep(conf, sender, target); err != nil {
		return
	}

	err = ModifyRep(evt.Context(), conf, msg.GuildID, sender, target, 1)
	if err != nil {
		if err == ErrCooldown {
			// Ignore this error silently
			return
		}
		logger.WithError(err).Error("Failed giving rep")
		return
	}

	go analytics.RecordActiveUnit(msg.GuildID, &Plugin{}, "auto_add_rep")

	content := fmt.Sprintf("Gave +1 %s to **%s**", conf.PointsName, who.Mention())
	common.BotSession.ChannelMessageSend(msg.ChannelID, content)
}

var cmds = []*commands.YAGCommand{
	&commands.YAGCommand{
		CmdCategory:  commands.CategoryFun,
		Name:         "TakeRep",
		Aliases:      []string{"-", "tr", "trep", "-rep"},
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
		Aliases:      []string{"+", "gr", "grep", "+rep"},
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
		Aliases:      []string{"SetRepID"}, // alias for legacy reasons, used to be a standalone command
		Description:  "Sets someones rep, this is an admin command and bypasses cooldowns and other restrictions.",
		RequiredArgs: 2,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "User", Type: dcmd.UserID},
			&dcmd.ArgDef{Name: "Num", Type: dcmd.Int},
		},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			conf, err := GetConfig(parsed.Context(), parsed.GS.ID)
			if err != nil {
				return "An error occured while finding the server config", err
			}

			if !IsAdmin(parsed.GS, parsed.MS, conf) {
				return "You're not a reputation admin. (no manage server perms and no rep admin role)", nil
			}

			targetID := parsed.Args[0].Int64()
			targetUsername := strconv.FormatInt(targetID, 10)
			targetMember, _ := bot.GetMember(parsed.GS.ID, targetID)
			if targetMember != nil {
				targetUsername = targetMember.Username
			}

			err = SetRep(parsed.Context(), parsed.GS.ID, parsed.MS.ID, targetID, int64(parsed.Args[1].Int()))
			if err != nil {
				return nil, err
			}

			return fmt.Sprintf("Set **%s** %s to `%d`", targetUsername, conf.PointsName, parsed.Args[1].Int()), nil
		},
	},
	&commands.YAGCommand{
		CmdCategory:  commands.CategoryFun,
		Name:         "DelRep",
		Description:  "Deletes someone from the reputation list completely, this cannot be undone.",
		RequiredArgs: 1,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "User", Type: dcmd.UserID},
		},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			conf, err := GetConfig(parsed.Context(), parsed.GS.ID)
			if err != nil {
				return "An error occured while finding the server config", err
			}

			if !IsAdmin(parsed.GS, parsed.MS, conf) {
				return "You're not an reputation admin. (no manage servers perms and no rep admin role)", nil
			}

			target := parsed.Args[0].Int64()

			err = DelRep(parsed.Context(), parsed.GS.ID, target)
			if err != nil {
				return nil, err
			}

			return fmt.Sprintf("Deleted all of %d's %s.", target, conf.PointsName), nil
		},
	},
	&commands.YAGCommand{
		CmdCategory:  commands.CategoryFun,
		Name:         "RepLog",
		Aliases:      []string{"replogs"},
		Description:  "Shows the rep log for the specified user.",
		RequiredArgs: 1,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "User", Type: dcmd.UserID},
			&dcmd.ArgDef{Name: "Page", Type: dcmd.Int, Default: 1},
		},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			conf, err := GetConfig(parsed.Context(), parsed.GS.ID)
			if err != nil {
				return "An error occured while finding the server config", err
			}

			if !IsAdmin(parsed.GS, parsed.MS, conf) {
				return "You're not an reputation admin. (no manage servers perms and no rep admin role)", nil
			}

			targetID := parsed.Args[0].Int64()

			const entriesPerPage = 20
			offset := (parsed.Args[1].Int() - 1) * entriesPerPage

			logEntries, err := models.ReputationLogs(qm.Where("guild_id = ? AND (receiver_id = ? OR sender_id = ?)", parsed.GS.ID, targetID, targetID), qm.OrderBy("id desc"), qm.Limit(entriesPerPage), qm.Offset(offset)).AllG(parsed.Context())
			if err != nil {
				return nil, err
			}

			if len(logEntries) < 1 {
				return "No entries", nil
			}

			// grab the up to date info on as many users as we can
			membersToGrab := make([]int64, 1, len(logEntries))
			membersToGrab[0] = targetID

		OUTER:
			for _, entry := range logEntries {
				for _, v := range membersToGrab {
					if entry.ReceiverID == targetID {
						if v == entry.SenderID {
							continue OUTER
						}
					} else {
						if v == entry.ReceiverID {
							continue OUTER
						}
					}
				}

				if entry.ReceiverID == targetID {
					membersToGrab = append(membersToGrab, entry.SenderID)
				} else {
					membersToGrab = append(membersToGrab, entry.ReceiverID)
				}
			}

			members, _ := bot.GetMembers(parsed.GS.ID, membersToGrab...)

			// finally display the results
			var out strings.Builder
			out.WriteString("```\n")
			for i, entry := range logEntries {
				receiver := entry.ReceiverUsername
				sender := entry.SenderUsername

				for _, v := range members {
					if v.ID == entry.ReceiverID {
						receiver = v.Username + "#" + v.StrDiscriminator()
					}
					if v.ID == entry.SenderID {
						sender = v.Username + "#" + v.StrDiscriminator()
					}
				}

				if receiver == "" {
					receiver = discordgo.StrID(entry.ReceiverID)
				}

				if sender == "" {
					sender = discordgo.StrID(entry.SenderID)
				}

				f := "#%2d: %-15s: %s gave %s: %d points"
				if entry.SetFixedAmount {
					f = "#%2d: %-15s: %s set %s points to: %d"
				}
				out.WriteString(fmt.Sprintf(f, i+offset+1, entry.CreatedAt.UTC().Format("02 Jan 06 15:04"), sender, receiver, entry.Amount))
				out.WriteRune('\n')
			}

			out.WriteString("```\n")
			out.WriteString(fmt.Sprint("Page ", parsed.Args[1].Int()))

			return out.String(), nil
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

			conf, err := GetConfig(parsed.Context(), parsed.GS.ID)
			if err != nil {
				return "An error occured finding the server config", err
			}

			score, rank, err := GetUserStats(parsed.GS.ID, target.ID)

			if err != nil {
				if err == ErrUserNotFound {
					rank = -1
				} else {
					return nil, err
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
		Description: "Shows rep leaderboard on the server",
		Arguments: []*dcmd.ArgDef{
			{Name: "Page", Type: dcmd.Int, Default: 0},
		},
		RunFunc: paginatedmessages.PaginatedCommand(0, func(parsed *dcmd.Data, p *paginatedmessages.PaginatedMessage, page int) (*discordgo.MessageEmbed, error) {
			offset := (page - 1) * 15
			entries, err := TopUsers(parsed.GS.ID, offset, 15)
			if err != nil {
				return nil, err
			}

			detailed, err := DetailedLeaderboardEntries(parsed.GS.ID, entries)
			if err != nil {
				return nil, err
			}

			if len(entries) < 1 && p != nil && p.LastResponse != nil { //Dont send No Results error on first execution
				return nil, paginatedmessages.ErrNoResults
			}

			embed := &discordgo.MessageEmbed{
				Title: "Reputation leaderboard",
			}

			leaderboardURL := web.BaseURL() + "/public/" + discordgo.StrID(parsed.GS.ID) + "/reputation/leaderboard"
			out := "```\n# -- Points -- User\n"
			for _, v := range detailed {
				user := v.Username
				if user == "" {
					user = "unknown ID:" + strconv.FormatInt(v.UserID, 10)
				}
				out += fmt.Sprintf("#%02d: %6d - %s\n", v.Rank, v.Points, user)
			}
			out += "```\n" + "Full leaderboard: <" + leaderboardURL + ">"

			embed.Description = out

			return embed, nil

		}),
	},
}

func CmdGiveRep(parsed *dcmd.Data) (interface{}, error) {
	target := parsed.Args[0].Value.(*discordgo.User)

	conf, err := GetConfig(parsed.Context(), parsed.GS.ID)
	if err != nil {
		return nil, err
	}

	pointsName := conf.PointsName

	if target.ID == parsed.Msg.Author.ID {
		return fmt.Sprintf("You can't modify your own %s... **Silly**", pointsName), nil
	}

	sender := parsed.MS
	receiver, err := bot.GetMember(parsed.GS.ID, target.ID)
	if err != nil {
		return nil, err
	}

	amount := parsed.Args[1].Int()

	err = ModifyRep(parsed.Context(), conf, parsed.GS.ID, sender, receiver, int64(amount))
	if err != nil {
		if cast, ok := err.(UserError); ok {
			return cast, nil
		}

		return nil, err
	}

	newScore, newRank, err := GetUserStats(parsed.GS.ID, target.ID)
	if err != nil {
		newScore = -1
		newRank = -1
		return nil, err
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
