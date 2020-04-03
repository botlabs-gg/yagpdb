package aylien

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"

	textapi "github.com/AYLIEN/aylien_textapi_go"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/config"
)

var (
	ErrNoMessages = errors.New("Failed finding any messages to analyze")

	confAppID  = config.RegisterOption("yagpdb.aylienappid", "AYLIEN App ID", "")
	confAppKey = config.RegisterOption("yagpdb.aylienappkey", "AYLIEN App Key", "")

	logger = common.GetPluginLogger(&Plugin{})
)

type Plugin struct {
	aylien *textapi.Client
}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "AYLIEN",
		SysName:  "aylien",
		Category: common.PluginCategoryMisc,
	}
}

func RegisterPlugin() {
	if confAppID.GetString() == "" || confAppKey.GetString() == "" {
		logger.Warn("Missing AYLIEN appid and/or key, not loading plugin")
		return
	}

	client, err := textapi.NewClient(textapi.Auth{ApplicationID: confAppID.GetString(), ApplicationKey: confAppKey.GetString()}, true)
	if err != nil {
		logger.WithError(err).Error("Failed initializing AYLIEN client")
		return
	}

	p := &Plugin{
		aylien: client,
	}

	common.RegisterPlugin(p)
}

var _ commands.CommandProvider = (*Plugin)(nil)

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(p, &commands.YAGCommand{
		CmdCategory: commands.CategoryFun,
		Cooldown:    5,
		Name:        "Sentiment",
		Aliases:     []string{"sent"},
		Description: "Does sentiment analysis on a message or your last 5 messages longer than 3 words",
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
					return "Error querying AYLIEN API", err
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
				toAnalyze := make([]*dstate.MessageState, 0)
				for i := len(msgs) - 1; i >= 0; i-- {
					msg := msgs[i]
					// logger.Println(msg.ID, msg.ContentWithMentionsReplaced())
					if msg.Author.ID == cmd.Msg.Author.ID {
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
						return "Error querying AYLIEN API", err
					}

					responses = append(responses, resp)
				}
			}

			out := fmt.Sprintf("**Sentiment analysis on %d messages:**\n", len(responses))
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
