package shardmemberfetcher

import "github.com/jonas747/dstate"

// var shardMemberFetchers = make()

type manager struct {
	reqChan chan []MemberFetchRequest

	fetchers    []*shardMemberFetcher
	state       *dstate.State
	totalShards int64
}

func NewManager(totalShards int64, state *dstate.State) *manager {
	return &manager{
		totalShards: totalShards,
		state:       state,
	}
}

func (m *manager) GetMember(guildID, userID int64) (*dstate.MemberState, error) {
	ret, err := m.GetMembers(guildID, userID)
	if err != nil {
		return nil, err
	}

	return ret[0], nil
}

func (m *manager) GetMembers(guildID int64, userIDs ...int64) ([]*dstate.MemberState, error) {

}

func (m *manager) GetMemberJoinedAt(guildID, userID int64) (*dstate.MemberState, error) {
}

type shardMemberFetcher struct {
	reqChan chan []MemberFetchRequest

	shardID int64

	// Queue of guilds to user id's to fetch
	// fetching map[int64][]*MemberFetchRequest
	waiting map[int64][]*MemberFetchRequest

	Queue []*MemberFetchRequest
}

type GuildIDMemberPair struct {
}

type MemberFetchRequest struct {
	Member       int64
	Guild        int64
	NeedJoinedAt bool
	resp         chan MemberFetchResult
}

type MemberFetchResult struct {
	Err    error
	Member *dstate.MemberState
}
