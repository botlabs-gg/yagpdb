package trivia

import (
	"database/sql"
	"fmt"
	"strings"

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
			embed := &discordgo.MessageEmbed{
				Title: "Trivia Rank",
			}

			if err != nil {
				if err == sql.ErrNoRows {
					embed.Description = fmt.Sprintf("<@%d> is unranked", userID)
					return embed, nil
				}
				return nil, err
			}
			field := &discordgo.MessageEmbedField{
				Name:  fmt.Sprintf("Rank #%d", rank),
				Value: fmt.Sprintf("<@%d>: Score %d | Played %d | Correct %d | Incorrect %d | Streak %d | Max Streak %d", user.UserID, user.Score, user.CorrectAnswers+user.IncorrectAnswers, user.CorrectAnswers, user.IncorrectAnswers, user.CurrentStreak, user.MaxStreak),
			}
			embed.Fields = append(embed.Fields, field)
			return embed, nil
		},
	}

	cmdLeaderboard := &commands.YAGCommand{
		Name:        "Leaderboard",
		Aliases:     []string{"lb", "top"},
		Description: "Shows the trivia leaderboard",
		CmdCategory: commands.CategoryFun,
		Arguments: []*dcmd.ArgDef{
			{Name: "Sort", Type: dcmd.String, Default: "score", Help: "Sort by score, streak, or maxstreak"},
		},
		RunFunc: func(parsed *dcmd.Data) (any, error) {
			sort := strings.ToLower(parsed.Args[0].Str())
			if sort != "streak" && sort != "maxstreak" {
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

					maxScore, maxStreak, err := GetTriviaGuildStats(p.GuildID)
					if err != nil {
						return nil, err
					}

					totalUsers, err := GetTotalTriviaUsers(p.GuildID)
					if err != nil {
						return nil, err
					}

					embed := &discordgo.MessageEmbed{
						Title:       "Trivia Leaderboard",
						Description: fmt.Sprintf("**Max Score:** %d\n**Max Streak:** %d\n\n", maxScore, maxStreak),
					}

					switch sort {
					case "streak":
						embed.Title += " (Sorted by Streak)"
					case "maxstreak":
						embed.Title += " (Sorted by Max Streak)"
					}

					for i, u := range users {
						entry := &discordgo.MessageEmbedField{}
						entry.Inline = false
						entry.Name = fmt.Sprintf("Rank #%d", offset+i+1)
						entry.Value = fmt.Sprintf("<@%d>: Score %d | Played %d | Correct %d | Incorrect %d | Streak %d | Max Streak %d", u.UserID, u.Score, u.CorrectAnswers+u.IncorrectAnswers, u.CorrectAnswers, u.IncorrectAnswers, u.CurrentStreak, u.MaxStreak)
						embed.Fields = append(embed.Fields, entry)
					}

					p.MaxPage = (totalUsers + 9) / 10

					return embed, nil
				},
			), nil
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

	commands.RegisterSlashCommandsContainer(container, true, func(gs *dstate.GuildSet) ([]int64, error) {
		return nil, nil
	})
}
