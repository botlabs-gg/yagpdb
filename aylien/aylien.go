package aylien

import (
	"errors"
	"fmt"
	"github.com/AYLIEN/aylien_textapi_go"
	"github.com/bwmarrin/discordgo"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"log"
	"strings"
)

var (
	ErrNoMessages = errors.New("Failed finding any messages to analyze")
)

type Plugin struct {
	aylien *textapi.Client
}

func RegisterPlugin() {
	if common.Conf.AylienAppID == "" || common.Conf.AylienAppKey == "" {
		log.Println("Missing AYLIEN appid and/or key, not loading plugin")
		return
	}

	client, err := textapi.NewClient(textapi.Auth{ApplicationID: common.Conf.AylienAppID, ApplicationKey: common.Conf.AylienAppKey}, true)
	if err != nil {
		log.Println("Failed initiazing aylien client, not enabling plugin", err)
		return
	}

	p := &Plugin{
		aylien: client,
	}
	bot.RegisterPlugin(p)
}

func (p *Plugin) Name() string {
	return "ALYIEN"
}

func (p *Plugin) InitBot() {
	commands.CommandSystem.RegisterCommands(&commands.CustomCommand{
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:        "sentiment",
			Aliases:     []string{"sent"},
			Description: "Does sentiment analysys on a message or your last 5 messages longer than 3 words",
			Arguments: []*commandsystem.ArgumentDef{
				&commandsystem.ArgumentDef{Name: "what", Description: "Specify something to analyze, by default it takes your last 5 messages", Type: commandsystem.ArgumentTypeString},
			},
		},
		RunFunc: func(parsed *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			// Were working hard!
			common.BotSession.ChannelTyping(m.ChannelID)

			var responses []*textapi.SentimentResponse
			if parsed.Args[0] != nil {
				resp, err := p.aylien.Sentiment(&textapi.SentimentParams{
					Text: parsed.Args[0].Str(),
				})
				if err != nil {
					return "Error querying aylien api", err
				}
				responses = []*textapi.SentimentResponse{resp}
			} else {

				// Get the message to analyze
				msgs, err := common.GetMessages(m.ChannelID, 100)
				if err != nil {
					return "Error retrieving messages", err
				}

				if len(msgs) < 1 {
					return ErrNoMessages, ErrNoMessages
				}

				// filter out our own and longer than 3 words
				toAnalyze := make([]*discordgo.Message, 0)
				for i := len(msgs) - 1; i >= 0; i-- {
					msg := msgs[i]
					// log.Println(msg.ID, msg.ContentWithMentionsReplaced())
					if msg.Author.ID == m.Author.ID {
						if len(strings.Fields(msg.ContentWithMentionsReplaced())) > 3 {
							toAnalyze = append(toAnalyze, msg)
							if len(toAnalyze) >= 5 {
								break
							}
						}
					}
				}

				if len(toAnalyze) < 1 {
					return ErrNoMessages, ErrNoMessages
				}

				for _, msg := range toAnalyze {
					resp, err := p.aylien.Sentiment(&textapi.SentimentParams{Text: msg.ContentWithMentionsReplaced()})
					if err != nil {
						return "Error querying aylien api", err
					}

					responses = append(responses, resp)
				}
			}

			out := fmt.Sprintf("**Sentiment analysys on %d messages:**\n", len(responses))
			for _, resp := range responses {
				out += fmt.Sprintf("*%s*\nPolarity: **%s** *(Confidence: %.2f%%)* Subjectivity: **%s** *(Confidence: %.2f%%)*\n\n", resp.Text, resp.Polarity, resp.PolarityConfidence*100, resp.Subjectivity, resp.SubjectivityConfidence*100)
			}
			return out, nil
		},
	},
		// This is a fun little always positive 8ball
		&commands.CustomCommand{
			Cooldown: 5,
			SimpleCommand: &commandsystem.SimpleCommand{
				Name:        "8Ball",
				Description: "Wisdom",
				Arguments: []*commandsystem.ArgumentDef{
					&commandsystem.ArgumentDef{Name: "What to ask", Type: commandsystem.ArgumentTypeString},
				},
				RequiredArgs: 1,
			},
			RunFunc: func(cmd *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
				resp, err := p.aylien.Sentiment(&textapi.SentimentParams{Text: cmd.Args[0].Str()})
				if err != nil {
					resp = &textapi.SentimentResponse{
						Polarity:               "neutral",
						PolarityConfidence:     1,
						Subjectivity:           "subjective",
						SubjectivityConfidence: 1,
					}
				}

				switch resp.Polarity {
				case "neutral":
					return "Maybe", nil
				case "positive":
					switch {
					case resp.PolarityConfidence >= 0 && resp.PolarityConfidence < 0.5:
						return "Most likely", nil
					case resp.PolarityConfidence >= 0.5:
						return "Without a doubt", nil
					}
				case "negative":
					switch {
					case resp.PolarityConfidence >= 0 && resp.PolarityConfidence < 0.5:
						return "Not likely", nil
					case resp.PolarityConfidence >= 0.5:
						return "Definetively not", nil
					}
				}
				return "Dunno", nil
			},
		})
}
