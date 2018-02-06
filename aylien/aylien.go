package aylien

import (
	"errors"
	"fmt"
	"github.com/AYLIEN/aylien_textapi_go"
	log "github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"math/rand"
	"os"
	"strings"
)

var (
	ErrNoMessages = errors.New("Failed finding any messages to analyze")

	appID  = os.Getenv("YAGPDB_AYLIENAPPID")
	appKey = os.Getenv("YAGPDB_AYLIENAPPKEY")
)

type Plugin struct {
	aylien *textapi.Client
}

func RegisterPlugin() {
	if appID == "" || appKey == "" {
		log.Warn("Missing AYLIEN appid and/or key, not loading plugin")
		return
	}

	client, err := textapi.NewClient(textapi.Auth{ApplicationID: appID, ApplicationKey: appKey}, true)
	if err != nil {
		log.WithError(err).Error("Failed initializing aylien client")
		return
	}

	p := &Plugin{
		aylien: client,
	}

	common.RegisterPlugin(p)
}

func (p *Plugin) Name() string {
	return "ALYIEN"
}

func (p *Plugin) InitBot() {
	commands.CommandSystem.RegisterCommands(&commands.CustomCommand{
		Category: commands.CategoryFun,
		Cooldown: 5,
		Command: &commandsystem.Command{
			Name:        "sentiment",
			Aliases:     []string{"sent"},
			Description: "Does sentiment analysys on a message or your last 5 messages longer than 3 words",
			Arguments: []*commandsystem.ArgDef{
				&commandsystem.ArgDef{Name: "what", Description: "Specify something to analyze, by default it takes your last 5 messages", Type: commandsystem.ArgumentString},
			},
			Run: func(cmd *commandsystem.ExecData) (interface{}, error) {
				// Were working hard!
				common.BotSession.ChannelTyping(cmd.Channel.ID())

				var responses []*textapi.SentimentResponse
				if cmd.Args[0] != nil {
					resp, err := p.aylien.Sentiment(&textapi.SentimentParams{
						Text: cmd.Args[0].Str(),
					})
					if err != nil {
						return "Error querying aylien api", err
					}
					responses = []*textapi.SentimentResponse{resp}
				} else {

					// Get the message to analyze
					msgs, err := bot.GetMessages(cmd.Channel.ID(), 100, false)
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
						if msg.Author.ID == cmd.Message.Author.ID {
							if len(strings.Fields(msg.ContentWithMentionsReplaced())) > 3 {
								toAnalyze = append(toAnalyze, msg.Message)
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
	},
		// This is a fun little always positive 8ball
		&commands.CustomCommand{
			Cooldown: 2,
			Category: commands.CategoryFun,
			Command: &commandsystem.Command{
				Name:        "8Ball",
				Description: "Wisdom",
				Arguments: []*commandsystem.ArgDef{
					&commandsystem.ArgDef{Name: "What to ask", Type: commandsystem.ArgumentString},
				},
				RequiredArgs: 1,
				Run: func(cmd *commandsystem.ExecData) (interface{}, error) {
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
						if rand.Intn(2) > 0 {
							return "Yes", nil
						} else {
							return "No", nil
						}
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
			},
		},
	)
}
