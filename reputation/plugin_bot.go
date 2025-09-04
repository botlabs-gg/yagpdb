package reputation

import (
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/analytics"
	"github.com/botlabs-gg/yagpdb/v2/bot/paginatedmessages"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/reputation/models"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

var _ bot.BotInitHandler = (*Plugin)(nil)
var _ commands.CommandProvider = (*Plugin)(nil)

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(p, cmds...)
}

func (p *Plugin) BotInit() {
	eventsystem.AddHandlerAsyncLastLegacy(p, handleMessageCreate, eventsystem.EventMessageCreate)
}

var thanksRegex = regexp.MustCompile(`(?i)( |\n|^)(thanks?|danks|ty|thx|\+rep|\+ ?\<\@[0-9]*\>)( |\pP|\n|$)`)

func createRepDisabledError(g *dcmd.GuildContextData) string {
	return fmt.Sprintf("**The reputation system is disabled for this server.** Enable it at: <%s/reputation>.", web.ManageServerURL(g.GS.ID))
}

func handleMessageCreate(evt *eventsystem.EventData) {
	msg := evt.MessageCreate()

	conf, err := GetConfig(evt.Context(), msg.GuildID)
	if err != nil || !conf.Enabled || conf.DisableThanksDetection {
		return
	}

	if !bot.IsUserMessage(msg.Message) {
		return
	}

	if len(msg.Mentions) < 1 || msg.GuildID == 0 || msg.Author.Bot {
		return
	}

	if !evt.HasFeatureFlag(featureFlagThanksEnabled) {
		return
	}

	// Premium guilds can set a custom thanks regex override
	usedRegex := thanksRegex
	if conf.ThanksRegex.Valid {
		if isPrem, _ := premium.IsGuildPremium(msg.GuildID); isPrem {
			if custom, err := regexp.Compile(conf.ThanksRegex.String); err == nil {
				usedRegex = custom
			}
		}
	}

	if !usedRegex.MatchString(msg.Content) {
		return
	}

	cState := evt.CSOrThread()
	if cState == nil {
		return // No channel state, ignore
	}

	channelID := msg.ChannelID
	// Check if thanks detection is allowed in the parent channel
	if cState.Type.IsThread() {
		channelID = cState.ParentID
	}
	if !isThanksDetectionAllowedInChannel(conf, channelID) {
		return
	}

	sender := dstate.MemberStateFromMember(msg.Member)
	for _, who := range msg.Mentions {
		if who.ID == msg.Author.ID {
			continue
		}

		target, err := bot.GetMember(msg.GuildID, who.ID)
		if err != nil {
			logger.WithError(err).Error("Failed retrieving target member")
			continue
		}
		if err = CanModifyRep(conf, sender, target); err != nil {
			continue
		}
		err = ModifyRep(evt.Context(), conf, evt.GS, sender, target, 1)
		if err != nil {
			if err == ErrCooldown {
				// Ignore this error silently
				continue
			}
			logger.WithError(err).Error("Failed giving rep")
			continue
		}

		go analytics.RecordActiveUnit(msg.GuildID, &Plugin{}, "auto_add_rep")
		newScore, newRank, err := GetUserStats(msg.GuildID, who.ID)
		if err != nil {
			newScore = -1
			newRank = -1
			logger.WithError(err).Error("Failed retrieving target stats")
			continue
		}

		content := fmt.Sprintf("Gave +1 %s to **%s** (current: `#%d` - `%d`)", conf.PointsName, who.Mention(), newRank, newScore)
		common.BotSession.ChannelMessageSend(msg.ChannelID, content)
	}
}

var cmds = []*commands.YAGCommand{
	{
		CmdCategory:  commands.CategoryFun,
		Name:         "TakeRep",
		Aliases:      []string{"-", "tr", "trep", "-rep"},
		Description:  "Takes away rep from someone",
		RequiredArgs: 1,
		Arguments: []*dcmd.ArgDef{
			{Name: "User", Type: dcmd.User},
			{Name: "Num", Type: dcmd.Int, Default: 1},
		},
		SlashCommandEnabled: true,
		DefaultEnabled:      false,
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			if parsed.Args[1].Int() < 1 {
				return "**rep amount should be greater than or equal to 1**", nil
			}
			parsed.Args[1].Value = -parsed.Args[1].Int()
			return CmdGiveRep(parsed)
		},
	},
	{
		CmdCategory:         commands.CategoryFun,
		Name:                "GiveRep",
		Aliases:             []string{"+", "gr", "grep", "+rep"},
		Description:         "Gives rep to someone",
		RequiredArgs:        1,
		SlashCommandEnabled: true,
		DefaultEnabled:      false,
		Arguments: []*dcmd.ArgDef{
			{Name: "User", Type: dcmd.User},
			{Name: "Num", Type: dcmd.Int, Default: 1},
		},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			if parsed.Args[1].Int() < 1 {
				return "**rep amount should be greater than or equal to 1**", nil
			}
			return CmdGiveRep(parsed)
		},
	},
	{
		CmdCategory:         commands.CategoryFun,
		Name:                "SetRep",
		Aliases:             []string{"SetRepID"}, // alias for legacy reasons, used to be a standalone command
		Description:         "Sets someones rep, this is an admin command and bypasses cooldowns and other restrictions.",
		RequiredArgs:        2,
		SlashCommandEnabled: true,
		DefaultEnabled:      false,
		Arguments: []*dcmd.ArgDef{
			{Name: "User", Type: dcmd.UserID},
			{Name: "Num", Type: dcmd.Int},
		},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			conf, err := GetConfig(parsed.Context(), parsed.GuildData.GS.ID)
			if err != nil {
				return "An error occurred while finding the server config", err
			}

			if !conf.Enabled {
				return createRepDisabledError(parsed.GuildData), nil
			}

			if !IsAdmin(parsed.GuildData.GS, parsed.GuildData.MS, conf) {
				return "You're not a reputation admin. (no manage server perms and no rep admin role)", nil
			}

			targetID := parsed.Args[0].Int64()
			targetUsername := strconv.FormatInt(targetID, 10)
			targetMember, _ := bot.GetMember(parsed.GuildData.GS.ID, targetID)
			if targetMember != nil {
				targetUsername = targetMember.User.Username
			} else {
				prevMember, err := userPresentInRepLog(targetID, parsed.GuildData.GS.ID, parsed)
				if err != nil {
					return nil, err
				}
				if !prevMember {
					return "Invalid User. This user never received/gave rep in this server", nil
				}
			}

			err = SetRep(parsed.Context(), parsed.GuildData.GS, parsed.GuildData.MS, targetMember, int64(parsed.Args[1].Int()))
			if err != nil {
				return nil, err
			}

			return fmt.Sprintf("Set **%s** %s to `%d`", targetUsername, conf.PointsName, parsed.Args[1].Int()), nil
		},
	},
	{
		CmdCategory:         commands.CategoryFun,
		Name:                "DelRep",
		Description:         "Deletes someone from the reputation list completely, this cannot be undone.",
		RequiredArgs:        1,
		SlashCommandEnabled: true,
		DefaultEnabled:      false,
		Arguments: []*dcmd.ArgDef{
			{Name: "User", Type: dcmd.UserID},
		},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			conf, err := GetConfig(parsed.Context(), parsed.GuildData.GS.ID)
			if err != nil {
				return "An error occurred while finding the server config", err
			}

			if !conf.Enabled {
				return createRepDisabledError(parsed.GuildData), nil
			}

			if !IsAdmin(parsed.GuildData.GS, parsed.GuildData.MS, conf) {
				return "You're not an reputation admin. (no manage servers perms and no rep admin role)", nil
			}

			target := parsed.Args[0].Int64()

			err = DelRep(parsed.Context(), parsed.GuildData.GS, target)
			if err != nil {
				return nil, err
			}

			return fmt.Sprintf("Deleted all of %d's %s.", target, conf.PointsName), nil
		},
	},
	{
		CmdCategory:         commands.CategoryFun,
		Name:                "RepLog",
		Aliases:             []string{"replogs"},
		Description:         "Shows the rep log for the specified user.",
		SlashCommandEnabled: true,
		DefaultEnabled:      false,
		Arguments: []*dcmd.ArgDef{
			{Name: "User", Type: dcmd.UserID},
			{Name: "Page", Type: dcmd.Int, Default: 1},
		},
		ArgumentCombos: [][]int{{}, {0}, {1}, {0, 1}},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			conf, err := GetConfig(parsed.Context(), parsed.GuildData.GS.ID)
			if err != nil {
				return "An error occurred while finding the server config", err
			}

			if !IsAdmin(parsed.GuildData.GS, parsed.GuildData.MS, conf) {
				return "You're not an reputation admin. (no manage servers perms and no rep admin role)", nil
			}

			targetID := parsed.Args[0].Int64()
			if targetID == 0 {
				targetID = parsed.Author.ID
			}

			const entriesPerPage = 20
			offset := (parsed.Args[1].Int() - 1) * entriesPerPage

			logEntries, err := models.ReputationLogs(qm.Where("guild_id = ? AND (receiver_id = ? OR sender_id = ?)", parsed.GuildData.GS.ID, targetID, targetID), qm.OrderBy("id desc"), qm.Limit(entriesPerPage), qm.Offset(offset)).AllG(parsed.Context())
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

			members, _ := bot.GetMembers(parsed.GuildData.GS.ID, membersToGrab...)

			// finally display the results
			var out strings.Builder
			out.WriteString("```\n")
			for i, entry := range logEntries {
				receiver := entry.ReceiverUsername
				sender := entry.SenderUsername

				for _, v := range members {
					if v.User.ID == entry.ReceiverID {
						receiver = v.User.String()
					}
					if v.User.ID == entry.SenderID {
						sender = v.User.String()
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
	{
		CmdCategory: commands.CategoryFun,
		Name:        "Rep",
		Description: "Shows yours or the specified users current rep and rank",
		Arguments: []*dcmd.ArgDef{
			{Name: "User", Type: dcmd.User},
		},
		SlashCommandEnabled: true,
		DefaultEnabled:      false,
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			target := parsed.Author
			if parsed.Args[0].Value != nil {
				target = parsed.Args[0].Value.(*discordgo.User)
			}

			conf, err := GetConfig(parsed.Context(), parsed.GuildData.GS.ID)
			if err != nil {
				return "An error occurred finding the server config", err
			}

			score, rank, err := GetUserStats(parsed.GuildData.GS.ID, target.ID)

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
	{
		CmdCategory: commands.CategoryFun,
		Name:        "TopRep",
		Description: "Shows rep leaderboard on the server",
		Arguments: []*dcmd.ArgDef{
			{Name: "Page", Type: dcmd.Int, Default: 0},
		},
		ArgSwitches: []*dcmd.ArgDef{
			{Name: "user", Help: "User to search for in the leaderboard", Type: dcmd.UserID},
		},
		SlashCommandEnabled: true,
		DefaultEnabled:      true,
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			page := parsed.Args[0].Int()
			if id := parsed.Switch("user").Int64(); id != 0 {
				const query = `
					SELECT pos
					FROM (
						SELECT ROW_NUMBER() OVER (ORDER BY points DESC) AS pos, user_id
						FROM reputation_users
						WHERE guild_id = $1
					) as ordered_users
					WHERE user_id = $2
				`

				var pos int
				err := common.PQ.QueryRow(query, parsed.GuildData.GS.ID, id).Scan(&pos)
				if err != nil {
					if err == sql.ErrNoRows {
						return "Could not find that user on the leaderboard", nil
					}
					return "Failed finding that user on the leaderboard, try again", err
				}

				page = (pos-1)/15 + 1 // pos and page are both one-based
			}

			if page < 1 {
				page = 1
			}

			if parsed.Context().Value(paginatedmessages.CtxKeyNoPagination) != nil {
				return topRepPager(parsed.GuildData.GS.ID, nil, page)
			}

			return paginatedmessages.NewPaginatedResponse(parsed.GuildData.GS.ID, parsed.ChannelID, page, 0, func(p *paginatedmessages.PaginatedMessage, page int) (*discordgo.MessageEmbed, error) {
				return topRepPager(parsed.GuildData.GS.ID, p, page)
			}), nil
		},
	},
}

func topRepPager(guildID int64, p *paginatedmessages.PaginatedMessage, page int) (*discordgo.MessageEmbed, error) {
	offset := (page - 1) * 15
	entries, err := TopUsers(guildID, offset, 15)
	if err != nil {
		return nil, err
	}

	detailed, err := DetailedLeaderboardEntries(guildID, entries)
	if err != nil {
		return nil, err
	}

	if len(entries) < 1 && p != nil && p.LastResponse != nil { //Dont send No Results error on first execution
		return nil, paginatedmessages.ErrNoResults
	}

	embed := &discordgo.MessageEmbed{
		Title: "Reputation leaderboard",
	}

	leaderboardURL := web.BaseURL() + "/public/" + discordgo.StrID(guildID) + "/reputation/leaderboard"
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

}

func CmdGiveRep(parsed *dcmd.Data) (interface{}, error) {
	target := parsed.Args[0].Value.(*discordgo.User)

	conf, err := GetConfig(parsed.Context(), parsed.GuildData.GS.ID)
	if err != nil {
		return nil, err
	}

	if !conf.Enabled {
		return createRepDisabledError(parsed.GuildData), nil
	}

	pointsName := conf.PointsName

	if target.ID == parsed.Author.ID {
		return fmt.Sprintf("You can't modify your own %s... **Silly**", pointsName), nil
	}

	sender := parsed.GuildData.MS
	receiver, err := bot.GetMember(parsed.GuildData.GS.ID, target.ID)
	if err != nil {
		return nil, err
	}

	amount := parsed.Args[1].Int()

	err = ModifyRep(parsed.Context(), conf, parsed.GuildData.GS, sender, receiver, int64(amount))
	if err != nil {
		if cast, ok := err.(UserError); ok {
			return cast, nil
		}

		return nil, err
	}

	newScore, newRank, err := GetUserStats(parsed.GuildData.GS.ID, target.ID)
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

// Function that checks if the given user has ever received/gave rep in the given server
func userPresentInRepLog(userID int64, guildID int64, parsed *dcmd.Data) (found bool, err error) {
	logEntries, err := models.ReputationLogs(qm.Where("guild_id = ? AND (receiver_id = ? OR sender_id = ?)", guildID, userID, userID), qm.OrderBy("id desc"), qm.Limit(1)).AllG(parsed.Context())
	if err != nil {
		return false, err
	}

	if len(logEntries) < 1 {
		return false, nil
	}
	return true, nil
}

// Checks if the thanks detection is allowed to be run in the given channel
func isThanksDetectionAllowedInChannel(config *models.ReputationConfig, channelID int64) bool {
	if len(config.BlacklistedThanksChannels) > 0 {
		if common.ContainsInt64Slice(config.BlacklistedThanksChannels, channelID) {
			return false
		}
	}
	if len(config.WhitelistedThanksChannels) > 0 {
		return common.ContainsInt64Slice(config.WhitelistedThanksChannels, channelID)
	}
	return true
}
