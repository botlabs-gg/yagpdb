package trivia

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/paginatedmessages"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
)

func (p *Plugin) AddCommands() {
	cmdStart := &commands.YAGCommand{
		Name:        "Start",
		Aliases:     []string{"s"},
		Description: "Starts a trivia session",
		CmdCategory: commands.CategoryFun,
		RunFunc: func(parsed *dcmd.Data) (any, error) {
			err := manager.NewTrivia(parsed.GuildData.GS.ID, parsed.ChannelID)
			if err != nil {
				logger.WithError(err).Error("Failed to create new trivia")
				if err == ErrSessionInChannel {
					return "There's already a trivia session in this channel", nil
				}
				return "Failed Running Trivia, unknown error", nil
			}
			return nil, nil
		},
	}

	cmdRank := &commands.YAGCommand{
		Name:        "Rank",
		Description: "Shows your trivia rank",
		ArgSwitches: []*dcmd.ArgDef{
			{Name: "user", Type: dcmd.UserID, Help: "Optional User to check rank for", Default: 0},
		},
		CmdCategory: commands.CategoryFun,
		RunFunc: func(data *dcmd.Data) (any, error) {
			var userID int64
			if data.Switches["user"].Int64() != 0 {
				userID = data.Switches["user"].Int64()
			} else {
				userID = data.Author.ID
			}
			user, rank, err := GetTriviaUser(data.GuildData.GS.ID, userID)
			if err != nil {
				if err == sql.ErrNoRows {
					return fmt.Sprintf("<@%d> is unranked. Play some trivia to get a rank!", userID), nil
				}
				return nil, err
			}

			username := fmt.Sprintf("%d", userID)
			thumbnail := "https://opentdb.com/images/logo-banner.png"
			if member, err := bot.GetMember(data.GuildData.GS.ID, userID); err == nil {
				username = member.User.Username
				thumbnail = member.User.AvatarURL("128")
			}

			var emoji string
			switch rank {
			case 1:
				emoji = "🥇"
			case 2:
				emoji = "🥈"
			case 3:
				emoji = "🥉"
			default:
				emoji = "🏅"
			}

			totalPlayed := user.CorrectAnswers + user.IncorrectAnswers
			embed := &discordgo.MessageEmbed{
				Title:       fmt.Sprintf("%s Trivia Rank: %s", emoji, username),
				Color:       0xFFD700, // Gold
				Description: fmt.Sprintf("**Rank**: #%d \n**Score**: %d", rank, user.Score),
				Fields: []*discordgo.MessageEmbedField{
					{Name: "Questions", Value: fmt.Sprintf("🎮 Total Played: **%d**\n✅ Correct: **%d**\n❌ Incorrect: **%d**\n",
						totalPlayed, user.CorrectAnswers, user.IncorrectAnswers), Inline: true},
					{Name: "Stats", Value: fmt.Sprintf("🔥 Streak: **%d**\n⚡ Max Streak: **%d**\n🏆 Win Rate: **%.1f%%**",
						user.CurrentStreak, user.MaxStreak, float64(user.CorrectAnswers)/float64(totalPlayed)*100), Inline: true},
				},
				Thumbnail: &discordgo.MessageEmbedThumbnail{
					URL: thumbnail,
				},
			}

			return embed, nil
		},
	}

	cmdLeaderboard := &commands.YAGCommand{
		Name:        "Leaderboard",
		Aliases:     []string{"lb", "top"},
		Description: "Shows the trivia leaderboard",
		CmdCategory: commands.CategoryFun,
		Arguments: []*dcmd.ArgDef{
			{
				Name: "Sort", Type: dcmd.String, Default: "score", Choices: []*discordgo.ApplicationCommandOptionChoice{
					{Name: "Score", Value: "score"},
					{Name: "Max Streak", Value: "maxstreak"},
					{Name: "Streak", Value: "streak"},
					{Name: "Correct Answers", Value: "correct"},
					{Name: "Incorrect Answers", Value: "incorrect"},
				}, Help: "Sort by score, streak, or maxstreak"},
		},
		RunFunc: func(parsed *dcmd.Data) (any, error) {
			sort := strings.ToLower(parsed.Args[0].Str())
			if sort != "streak" && sort != "maxstreak" && sort != "correct" && sort != "incorrect" {
				sort = "score"
			}

			return paginatedmessages.NewPaginatedResponse(
				parsed.GuildData.GS.ID,
				parsed.ChannelID,
				1,
				0,
				func(p *paginatedmessages.PaginatedMessage, page int) (*discordgo.MessageEmbed, error) {
					offset := (page - 1) * 10
					users, err := GetTopTriviaUsers(p.GuildID, 10, offset, sort)
					if err != nil {
						return nil, err
					}

					if len(users) == 0 && page > 1 {
						return nil, paginatedmessages.ErrNoResults
					}

					maxScore, currentStreak, maxStreak, maxCorrect, maxIncorrect, err := GetTriviaGuildStats(p.GuildID)
					if err != nil {
						return nil, err
					}

					totalUsers, err := GetTotalTriviaUsers(p.GuildID)
					if err != nil {
						return nil, err
					}
					titleemoji := "🥇🥈🏅"
					if sort == "incorrect" {
						titleemoji = "🤡🤡🤡"
					}

					embed := &discordgo.MessageEmbed{
						Title:       titleemoji + " Trivia Leaderboard",
						Color:       0xFFD700, // Gold
						Description: "👑 **Server Best**",
						Fields: []*discordgo.MessageEmbedField{
							{Name: "Highest Score:", Value: fmt.Sprintf("**%d**", maxScore), Inline: true},
							{Name: "Longest Current Streak", Value: fmt.Sprintf("**%d**", currentStreak), Inline: true},
							{Name: "Longest Streak Ever", Value: fmt.Sprintf("**%d**", maxStreak), Inline: true},
							{Name: "Most Correct Answers", Value: fmt.Sprintf("**%d**", maxCorrect), Inline: true},
							{Name: "Most Incorrect Answers", Value: fmt.Sprintf("**%d**", maxIncorrect), Inline: true},
						},
					}

					switch sort {
					case "streak":
						embed.Title += " (By Streak)"
					case "maxstreak":
						embed.Title += " (By Max Streak)"
					case "correct":
						embed.Title += " (By Correct Answers)"
					case "incorrect":
						embed.Title += " (By Incorrect Answers)"
					case "score":
						embed.Title += " (By Score)"
					}
					emojiList := []string{"🥇", "🥈", "🥉"}
					if sort == "incorrect" {
						emojiList = []string{"🤡", "💩", "🗿"}
					}
					for i, u := range users {
						emoji := ""
						rank := offset + i + 1
						if rank <= len(emojiList) {
							emoji = emojiList[rank-1] + " "
						}
						entry := &discordgo.MessageEmbedField{}
						entry.Inline = false
						entry.Name = fmt.Sprintf("%sRank #%d", emoji, rank)
						entry.Value = fmt.Sprintf("**<@%d>**: Score **%d** | Played **%d** | Correct **%d** | Incorrect **%d** | Streak **%d** | Max Streak **%d**", u.UserID, u.Score, u.CorrectAnswers+u.IncorrectAnswers, u.CorrectAnswers, u.IncorrectAnswers, u.CurrentStreak, u.MaxStreak)
						embed.Fields = append(embed.Fields, entry)
					}

					p.MaxPage = (totalUsers + 9) / 10

					return embed, nil
				},
			), nil
		},
	}

	cmdLbReset := &commands.YAGCommand{
		Name:                "ResetLeaderboard",
		Description:         "Resets the trivia leaderboard for the server",
		RequireDiscordPerms: []int64{discordgo.PermissionManageGuild},
		CmdCategory:         commands.CategoryFun,
		RunFunc: func(data *dcmd.Data) (any, error) {
			ResetTriviaLeaderboard(data.GuildData.GS.ID)
			return "Leaderboard reset", nil
		},
	}

	container, _ := commands.CommandSystem.Root.Sub("Trivia", "triv")
	container.Description = "Trivia commands"
	container.NotFound = func(data *dcmd.Data) (any, error) {
		if data.TraditionalTriggerData != nil {
			if strings.TrimSpace(data.TraditionalTriggerData.MessageStrippedPrefix) == "" {
				return cmdStart.RunFunc(data)
			}
		} else if data.TriggerType == dcmd.TriggerTypeSlashCommands {
			return cmdStart.RunFunc(data)
		}
		return commands.CommonContainerNotFoundHandler(container, "")(data)
	}

	container.AddCommand(cmdStart, cmdStart.GetTrigger())
	container.AddCommand(cmdRank, cmdRank.GetTrigger())
	container.AddCommand(cmdLeaderboard, cmdLeaderboard.GetTrigger())
	container.AddCommand(cmdLbReset, cmdLbReset.GetTrigger())

	commands.RegisterSlashCommandsContainer(container, true, func(gs *dstate.GuildSet) ([]int64, error) {
		return nil, nil
	})
}
