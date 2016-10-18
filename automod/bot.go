package automod

// import (
// 	"github.com/Sirupsen/logrus"
// 	"github.com/fzzy/radix/redis"
// 	"github.com/jonas747/discordgo"
// 	"github.com/jonas747/yagpdb/bot"
// 	"github.com/jonas747/yagpdb/common"
// 	"net/url"
// 	"strings"
// )

// func (p *Plugin) InitBot() {
// 	common.BotSession.AddHandler(bot.CustomMessageCreate(HandleMessageCreate))
// }

// func HandleMessageCreate(s *discordgo.Session, evt *discordgo.MessageCreate, client *redis.Client) {
// 	channel := common.LogGetChannel(evt.ChannelID)
// 	if channel == nil {
// 		return
// 	}

// 	// TODO cache configs
// 	enabled, _ := client.Cmd("GET", KEYENABLED(channel.GuildID)).Bool()
// 	if !enabled {
// 		logrus.Info("Not enabled")
// 		return
// 	}

// 	rules, err := GetRules(channel.GuildID, client)
// 	if err != nil {
// 		logrus.WithError(err).Error("Error retrieving automod rules")
// 		return
// 	}

// 	lits, err := GetLists(channel.GuildID, client)
// 	if err != nil {
// 		logrus.WithError(err).Error("Error retreiving automod lists")
// 		return
// 	}

// }

// // Return true if it should get deleted, and optionally the rule it belongs to
// func checkWordsWebsites(evt *discordgo.MessageCreate, lists *ListConfig, rules []*Rule) (bool, *Rule) {
// 	fields := strings.Fields(strings.ToLower(evt.Message.Content))

// 	matchWord := false
// 	matchSite := false

// OUTER:
// 	for _, field := range fields {
// 		for _, word := range lists.BannedWords {
// 			if field == word {
// 				matchWord = true
// 				break OUTER
// 			}
// 		}

// 		u, err := url.ParseRequestURI(field)
// 		if err != nil {
// 			logrus.WithError(err).Error("Failed parsing url")
// 			continue
// 		}

// 		for _, site := range lists.BannedWebsites {
// 			if u.Host == site {
// 				matchSite = true
// 				break OUTER
// 			}
// 		}
// 	}

// 	if !matchWord && !matchSite {
// 		return false, nil
// 	}

// 	logrus.Info("Found word or site match", matchWord, matchSite)

// 	for _, v := range rules {

// 	}
// }
// func checkSpam(evt *discordgo.MessageCreate, rules []*Rule) {}
