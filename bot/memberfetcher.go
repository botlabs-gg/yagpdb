package bot

import (
	"github.com/botlabs-gg/yagpdb/v2/bot/shardmemberfetcher"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
)

var (
	memberFetcher *shardmemberfetcher.Manager
)

// GetMember will either return a member from state or fetch one from the member fetcher and then put it in state
func GetMember(guildID, userID int64) (*dstate.MemberState, error) {
	if memberFetcher != nil {
		return memberFetcher.GetMember(guildID, userID)
	}

	ms := State.GetMember(guildID, userID)
	if ms != nil && ms.Member != nil {
		return ms, nil
	}

	member, err := common.BotSession.GuildMember(guildID, userID)
	if err != nil {
		return nil, err
	}

	return dstate.MemberStateFromMember(member), nil
}

// GetMembers is the same as GetMember but with multiple members
func GetMembers(guildID int64, userIDs ...int64) ([]*dstate.MemberState, error) {
	if memberFetcher != nil {
		return memberFetcher.GetMembers(guildID, userIDs...)
	}

	// fall back to something really slow
	result := make([]*dstate.MemberState, 0, len(userIDs))
	for _, v := range userIDs {
		r, err := GetMember(guildID, v)
		if err != nil {
			continue
		}

		result = append(result, r)
	}

	return result, nil
}
