package discorddata

import (
	"sort"
	"strconv"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/bot/botrest"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/karlseguin/ccache"
	"golang.org/x/oauth2"
)

type Plugin struct{}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "web_discorddata",
		SysName:  "web_discorddata",
		Category: common.PluginCategoryMisc,
	}
}

var logger = common.GetPluginLogger(&Plugin{})

func RegisterPlugin() {
	common.RegisterPlugin(&Plugin{})
}

var applicationCache = ccache.New(ccache.Configure().MaxSize(10000).ItemsToPrune(100))

func keySession(raw string) string {
	return "discord_session:" + raw
}

func GetSession(token string, sessionFetcher func(string) (*oauth2.Token, error)) (*discordgo.Session, error) {
	result, err := applicationCache.Fetch(keySession(token), time.Minute*60, func() (interface{}, error) {
		decoded, err := sessionFetcher(token)
		if err != nil {
			return nil, errors.WithStackIf(err)
		}

		session, err := discordgo.New(decoded.Type() + " " + decoded.AccessToken)
		if err != nil {
			return nil, errors.WithStackIf(err)
		}

		return session, nil
	})
	if err != nil {
		return nil, err
	}

	return result.Value().(*discordgo.Session), nil
}

func EvictSession(token string) {
	applicationCache.Delete(keySession(token))
}

func keyUserInfo(token string) string {
	return "user_info_token:" + token
}

func GetUserInfo(token string, session *discordgo.Session) (*discordgo.User, error) {
	result, err := applicationCache.Fetch(keyUserInfo(token), time.Minute*10, func() (interface{}, error) {
		user, err := session.UserMe()
		if err != nil {
			return nil, errors.WithStackIf(err)
		}

		return user, nil
	})

	if err != nil {
		return nil, err
	}

	return result.Value().(*discordgo.User), nil
}

func keyFullGuild(guildID int64) string {
	return "full_guild:" + strconv.FormatInt(guildID, 10)
}

// GetFullGuild returns the guild from either:
// 1. Application cache
// 2. Botrest
// 3. Discord api
//
// It will will also make sure channels are included in the event we fall back to the discord API
func GetFullGuild(guildID int64) (*dstate.GuildSet, error) {
	result, err := applicationCache.Fetch(keyFullGuild(guildID), time.Minute*10, func() (interface{}, error) {
		gs, err := botrest.GetGuild(guildID)
		if err != nil {
			// fall back to discord API

			guild, err := common.BotSession.Guild(guildID)
			if err != nil {
				return nil, err
			}

			// we also need to include channels as they're not included in the guild response
			channels, err := common.BotSession.GuildChannels(guildID)
			if err != nil {
				return nil, err
			}

			// does the API guarantee the order? i actually have no idea lmao
			sort.Sort(common.DiscordChannels(channels))
			sort.Sort(common.DiscordRoles(guild.Roles))
			guild.Channels = channels

			gs = dstate.GuildSetFromGuild(guild)
		}

		return gs, nil
	})

	if err != nil {
		return nil, err
	}

	return result.Value().(*dstate.GuildSet), nil
}

func keyGuildMember(guildID int64, userID int64) string {
	return "guild_member:" + strconv.FormatInt(guildID, 10) + ":" + strconv.FormatInt(userID, 10)
}

func GetMember(guildID, userID int64) (*discordgo.Member, error) {
	result, err := applicationCache.Fetch(keyGuildMember(guildID, userID), time.Minute*10, func() (interface{}, error) {

		results, err := botrest.GetMembers(guildID, userID)

		var m *discordgo.Member
		if len(results) > 0 {
			m = results[0]
		}

		if err != nil || m == nil {
			// fallback to discord api
			m, err = common.BotSession.GuildMember(guildID, userID)
			if err != nil {
				return nil, err
			}
		}

		return m, nil
	})

	if err != nil {
		return nil, err
	}

	return result.Value().(*discordgo.Member), nil
}
