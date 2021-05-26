package bot

import (
	"github.com/jonas747/dstate/v2"
	"github.com/jonas747/yagpdb/bot/shardmemberfetcher"
	"github.com/jonas747/yagpdb/common"
)

var (
	memberFetcher *shardmemberfetcher.Manager
)

// GetMember will either return a member from state or fetch one from the member fetcher and then put it in state
func GetMember(guildID, userID int64) (*dstate.MemberState, error) {
	if memberFetcher != nil {
		return memberFetcher.GetMember(guildID, userID)
	}

	// fallback to this
	gs := State.Guild(true, guildID)
	if gs == nil {
		return nil, ErrGuildNotFound
	}

	cop := gs.MemberCopy(true, userID)
	if cop != nil && cop.MemberSet {
		return cop, nil
	}

	member, err := common.BotSession.GuildMember(guildID, userID)
	if err != nil {
		return nil, err
	}

	ms := dstate.MSFromDGoMember(gs, member)
	return ms, nil
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

// GetMemberJoinedAt is the same as GetMember but it ensures the JoinedAt field is present
func GetMemberJoinedAt(guildID, userID int64) (*dstate.MemberState, error) {
	if memberFetcher != nil {
		return memberFetcher.GetMember(guildID, userID)
	}

	gs := State.Guild(true, guildID)
	if gs == nil {
		return nil, ErrGuildNotFound
	}

	cop := gs.MemberCopy(true, userID)
	if cop != nil && cop.MemberSet && !cop.JoinedAt.IsZero() {
		return cop, nil
	}

	member, err := common.BotSession.GuildMember(guildID, userID)
	if err != nil {
		return nil, err
	}

	ms := dstate.MSFromDGoMember(gs, member)
	return ms, nil
}
