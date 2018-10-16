package aylien

import (
	"errors"
	"fmt"
	"github.com/AYLIEN/aylien_textapi_go"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	log "github.com/sirupsen/logrus"
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

var _ commands.CommandProvider = (*Plugin)(nil)

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(&commands.YAGCommand{
		CmdCategory: commands.CategoryFun,
		Cooldown:    5,
		Name:        "sentiment",
		Aliases:     []string{"sent"},
		Description: "Does sentiment analysys on a message or your last 5 messages longer than 3 words",
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "text", Type: dcmd.String},
		},
		RunFunc: func(cmd *dcmd.Data) (interface{}, error) {
			var responses []*textapi.SentimentResponse
			if cmd.Args[0].Value != nil {
				resp, err := p.aylien.Sentiment(&textapi.SentimentParams{
					Text: cmd.Args[0].Str(),
				})
				if err != nil {
					return "Error querying aylien api", err
				}
				responses = []*textapi.SentimentResponse{resp}
			} else {

				// Get the message to analyze
				msgs, err := bot.GetMessages(cmd.CS.ID, 100, false)
				if err != nil {
					return "", err
				}

				if len(msgs) < 1 {
					return ErrNoMessages, ErrNoMessages
				}

				// filter out our own and longer than 3 words
				toAnalyze := make([]*discordgo.Message, 0)
				for i := len(msgs) - 1; i >= 0; i-- {
					msg := msgs[i]
					// log.Println(msg.ID, msg.ContentWithMentionsReplaced())
					if msg.Author.ID == cmd.Msg.Author.ID {
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
		// This is a fun little always positive 8ball
		&commands.YAGCommand{
			Cooldown:    2,
			CmdCategory: commands.CategoryFun,
			Name:        "8Ball",
			Description: "Wisdom",
			Arguments: []*dcmd.ArgDef{
				&dcmd.ArgDef{Name: "What to ask", Type: dcmd.String},
			},
			RequiredArgs: 1,
			RunFunc: func(cmd *dcmd.Data) (interface{}, error) {
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
						return "Definitively not", nil
					}
				}
				return "Dunno", nil
			},
		},
	)
}
