package bot

import (
	"fmt"

	"github.com/botlabs-gg/yagpdb/v2/bot/shardmemberfetcher"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
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

	// State can be nil in case of execCC done from templates that exec on non-bot contexts,
	// like youtube feeds. In this case we check if a state is nil, and if it is we fetch from discord API.
	if State != nil {
		ms := State.GetMember(guildID, userID)
		if ms != nil && ms.Member != nil {
			return ms, nil
		}
	}

	member, err := common.BotSession.GuildMember(guildID, userID)
	if err != nil {
		return nil, err
	}

	return dstate.MemberStateFromMember(member), nil
}

func GetMemberVoiceState(guildID, userID int64) (*discordgo.VoiceState, error) {
	gs := State.GetGuild(guildID)
	if gs == nil {
		return nil, fmt.Errorf("guild not in state")
	}
	vs := gs.GetVoiceState(userID)
	if vs != nil {
		return vs, nil
	}

	vs, err := common.BotSession.GuildMemberVoiceState(guildID, userID)
	if err != nil {
		return nil, err
	}
	return vs, nil
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
