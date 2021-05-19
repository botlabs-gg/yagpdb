package embeddedtracker

import (
	"container/list"
	"sync"
	"time"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot/state"
)

type TrackerConfig struct {
	ChannelMessageLen int
	ChannelMessageDur time.Duration

	ChannelMessageLimitsF func(guildID int64) (int, time.Duration)
}

type SimpleStateTracker struct {
	totalShards int64
	shards      []*SimpleStateTrackerShard
	// conf   TrackerConfig
}

func NewSimpleStateTracker(conf TrackerConfig, totalShards int64) *SimpleStateTracker {
	shards := make([]*SimpleStateTrackerShard, totalShards)
	for i := range shards {
		shards[i] = newShard(conf, i)
	}

	return &SimpleStateTracker{
		shards:      shards,
		totalShards: totalShards,
	}
}

// These are updated less frequently and so we remake the indiv lists on update
// this makes us able to just return a straight reference, since the object is effectively immutable
type SparseGuildState struct {
	Guild       *state.CachedGuild
	Channels    []*state.CachedChannel
	Roles       []*state.CachedRole
	Emojis      []*state.CachedEmoji
	VoiceStates []*discordgo.VoiceState
}

// returns a new copy of SparseGuildState and the inner Guild
func (s *SparseGuildState) copyGuildSet() *SparseGuildState {
	guildSetCopy := *s
	return &guildSetCopy
}

// returns a new copy of SparseGuildState and the inner Guild
func (s *SparseGuildState) copyGuild() *SparseGuildState {
	guildSetCopy := *s
	innerGuild := *s.Guild

	guildSetCopy.Guild = &innerGuild

	return &guildSetCopy
}

// returns a new copy of SparseGuildState and the channels slice
func (s *SparseGuildState) copyChannels() *SparseGuildState {
	guildSetCopy := *s

	guildSetCopy.Channels = make([]*state.CachedChannel, len(guildSetCopy.Channels))
	copy(guildSetCopy.Channels, s.Channels)

	return &guildSetCopy
}

// returns a new copy of SparseGuildState and the roles slice
func (s *SparseGuildState) copyRoles() *SparseGuildState {
	guildSetCopy := *s

	guildSetCopy.Roles = make([]*state.CachedRole, len(guildSetCopy.Roles))
	copy(guildSetCopy.Roles, s.Roles)

	return &guildSetCopy
}

// returns a new copy of SparseGuildState and the channels slice
func (s *SparseGuildState) copyVoiceStates() *SparseGuildState {
	guildSetCopy := *s

	guildSetCopy.VoiceStates = make([]*discordgo.VoiceState, len(guildSetCopy.VoiceStates))
	copy(guildSetCopy.VoiceStates, s.VoiceStates)

	return &guildSetCopy
}

type SimpleStateTrackerShard struct {
	mu sync.RWMutex

	shardID int

	// Key is GuildID
	guilds  map[int64]*SparseGuildState
	members map[int64][]*state.CachedMember

	// Key is ChannelID
	messages map[int64]*list.List

	conf TrackerConfig
}

func newShard(conf TrackerConfig, id int) *SimpleStateTrackerShard {
	return &SimpleStateTrackerShard{
		shardID:  id,
		guilds:   make(map[int64]*SparseGuildState),
		members:  make(map[int64][]*state.CachedMember),
		messages: make(map[int64]*list.List),
		conf:     conf,
	}
}

func (tracker *SimpleStateTrackerShard) HandleEvent(s *discordgo.Session, i interface{}) {

	switch evt := i.(type) {
	// Guild events
	case *discordgo.GuildCreate:
		tracker.handleGuildCreate(evt)
	case *discordgo.GuildUpdate:
		tracker.handleGuildUpdate(evt)
	case *discordgo.GuildDelete:
		tracker.handleGuildDelete(evt)

	// Member events
	case *discordgo.GuildMemberAdd:
		tracker.handleMemberCreate(evt)
	case *discordgo.GuildMemberUpdate:
		tracker.handleMemberUpdate(evt.Member)
	case *discordgo.GuildMemberRemove:
		tracker.handleMemberDelete(evt)

	// Channel events
	case *discordgo.ChannelCreate:
		tracker.handleChannelCreateUpdate(evt.Channel)
	case *discordgo.ChannelUpdate:
		tracker.handleChannelCreateUpdate(evt.Channel)
	case *discordgo.ChannelDelete:
		tracker.handleChannelDelete(evt)

	// Role events
	case *discordgo.GuildRoleCreate:
		tracker.handleRoleCreateUpdate(evt.GuildID, evt.Role)
	case *discordgo.GuildRoleUpdate:
		tracker.handleRoleCreateUpdate(evt.GuildID, evt.Role)
	case *discordgo.GuildRoleDelete:
		tracker.handleRoleDelete(evt)

	// Message events
	case *discordgo.MessageCreate:
		tracker.handleMessageCreate(evt)
	case *discordgo.MessageUpdate:
		tracker.handleMessageUpdate(evt)
	case *discordgo.MessageDelete:
		tracker.handleMessageDelete(evt)
	case *discordgo.MessageDeleteBulk:
		tracker.handleMessageDeleteBulk(evt)

	// Other
	case *discordgo.PresenceUpdate:
		tracker.handlePresenceUpdate(evt)
	case *discordgo.VoiceStateUpdate:
		tracker.handleVoiceStateUpdate(evt)
	case *discordgo.Ready:
		tracker.handleReady(evt)
	case *discordgo.GuildEmojisUpdate:
		tracker.handleEmojis(evt)
	default:
		return
	}

	// if s.Debug {
	// 	t := reflect.Indirect(reflect.ValueOf(i)).Type()
	// 	log.Printf("Handled event %s; %#v", t.Name(), i)
	// }
}

///////////////////
// Guild events
///////////////////

func (shard *SimpleStateTrackerShard) handleGuildCreate(gc *discordgo.GuildCreate) {
	shard.mu.Lock()
	defer shard.mu.Unlock()

	channels := make([]*state.CachedChannel, 0, len(gc.Channels))
	for _, v := range gc.Channels {
		channels = append(channels, state.NewCachedChannel(v))
	}

	roles := make([]*state.CachedRole, 0, len(gc.Roles))
	for _, v := range gc.Roles {
		roles = append(roles, state.NewCachedRole(v))
	}

	// voiceStates := make([]*discordgo.VoiceState, 0, len(gc.VoiceStates))
	// for _, v := range gc.VoiceStates {
	// 	voiceStates = append(voiceStates, v)
	// }

	emojis := make([]*state.CachedEmoji, 0, len(gc.Emojis))
	for _, v := range gc.Emojis {
		emojis = append(emojis, state.NewCachedEmoji(v))
	}

	guildState := &SparseGuildState{
		Guild:       state.NewCachedGuild(gc.Guild),
		Channels:    channels,
		Roles:       roles,
		Emojis:      emojis,
		VoiceStates: gc.VoiceStates,
	}

	shard.guilds[gc.ID] = guildState
}

func (shard *SimpleStateTrackerShard) handleGuildUpdate(gu *discordgo.GuildUpdate) {
	shard.mu.Lock()
	defer shard.mu.Unlock()

	newInnerGuild := state.NewCachedGuild(gu.Guild)

	if existing, ok := shard.guilds[gu.ID]; ok {
		newSparseGuild := existing.copyGuildSet()

		newInnerGuild.MemberCount = existing.Guild.MemberCount

		newSparseGuild.Guild = newInnerGuild
		shard.guilds[gu.ID] = newSparseGuild
	} else {
		shard.guilds[gu.ID] = &SparseGuildState{
			Guild: newInnerGuild,
		}
	}
}

func (shard *SimpleStateTrackerShard) handleGuildDelete(gd *discordgo.GuildDelete) {
	shard.mu.Lock()
	defer shard.mu.Unlock()

	if gd.Unavailable {
		if existing, ok := shard.guilds[gd.ID]; ok {
			// Note: only allowed to update guild here as that field has been copied
			newSparseGuild := existing.copyGuild()
			newSparseGuild.Guild.Available = false

			shard.guilds[gd.ID] = newSparseGuild
		}
	} else {
		if existing, ok := shard.guilds[gd.ID]; ok {
			for _, v := range existing.Channels {
				delete(shard.messages, v.ID)
			}
		}

		delete(shard.members, gd.ID)
		delete(shard.guilds, gd.ID)
	}
}

///////////////////
// Channel events
///////////////////

func (shard *SimpleStateTrackerShard) handleChannelCreateUpdate(c *discordgo.Channel) {
	shard.mu.Lock()
	defer shard.mu.Unlock()

	gs, ok := shard.guilds[c.GuildID]
	if !ok {
		return
	}

	for i, v := range gs.Channels {
		if v.ID == c.ID {
			newSparseGuild := gs.copyChannels()
			newSparseGuild.Channels[i] = state.NewCachedChannel(c)
			return
		}
	}

	// channel was not already in state, we need to add it to the channels slice
	newSparseGuild := gs.copyGuildSet()
	newSparseGuild.Channels = append(newSparseGuild.Channels, state.NewCachedChannel(c))

	shard.guilds[c.GuildID] = newSparseGuild
}

func (shard *SimpleStateTrackerShard) handleChannelDelete(c *discordgo.ChannelDelete) {
	shard.mu.Lock()
	defer shard.mu.Unlock()

	delete(shard.messages, c.ID)

	gs, ok := shard.guilds[c.GuildID]
	if !ok {
		return
	}

	for i, v := range gs.Channels {
		if v.ID == c.ID {
			newSparseGuild := gs.copyChannels()
			newSparseGuild.Channels = append(newSparseGuild.Channels[:i], newSparseGuild.Channels[i+1:]...)
			shard.guilds[c.GuildID] = newSparseGuild
			return
		}
	}
}

///////////////////
// Role events
///////////////////

func (shard *SimpleStateTrackerShard) handleRoleCreateUpdate(guildID int64, r *discordgo.Role) {
	shard.mu.Lock()
	defer shard.mu.Unlock()

	gs, ok := shard.guilds[guildID]
	if !ok {
		return
	}

	for i, v := range gs.Roles {
		if v.ID == r.ID {
			newSparseGuild := gs.copyRoles()
			newSparseGuild.Roles[i] = state.NewCachedRole(r)
			return
		}
	}

	// role was not already in state, we need to add it to the roles slice
	newSparseGuild := gs.copyGuildSet()
	newSparseGuild.Roles = append(newSparseGuild.Roles, state.NewCachedRole(r))

	shard.guilds[guildID] = newSparseGuild
}

func (shard *SimpleStateTrackerShard) handleRoleDelete(r *discordgo.GuildRoleDelete) {
	shard.mu.Lock()
	defer shard.mu.Unlock()

	gs, ok := shard.guilds[r.GuildID]
	if !ok {
		return
	}

	for i, v := range gs.Roles {
		if v.ID == r.RoleID {
			newSparseGuild := gs.copyRoles()
			newSparseGuild.Roles = append(newSparseGuild.Roles[:i], newSparseGuild.Roles[i+1:]...)
			shard.guilds[r.GuildID] = newSparseGuild
			return
		}
	}
}

///////////////////
// Member events
///////////////////

func (shard *SimpleStateTrackerShard) handleMemberCreate(m *discordgo.GuildMemberAdd) {
	shard.mu.Lock()
	defer shard.mu.Unlock()

	gs, ok := shard.guilds[m.GuildID]
	if !ok {
		return
	}

	newSparseGuild := gs.copyGuild()
	newSparseGuild.Guild.MemberCount++
	shard.guilds[m.GuildID] = newSparseGuild

	shard.innerHandleMemberUpdate(m.Member)
}

func (shard *SimpleStateTrackerShard) handleMemberUpdate(m *discordgo.Member) {
	shard.mu.Lock()
	defer shard.mu.Unlock()
	shard.innerHandleMemberUpdate(m)
}

// assumes state is locked
func (shard *SimpleStateTrackerShard) innerHandleMemberUpdate(m *discordgo.Member) {

	members, ok := shard.members[m.GuildID]
	if !ok {
		// intialize slice
		shard.members[m.GuildID] = []*state.CachedMember{state.NewCachedMember(m)}
		return
	}

	for i, v := range members {
		if v.ID == m.User.ID {
			// replace in slice
			members[i] = state.NewCachedMember(m)
			return
		}
	}

	// member was not already in state, we need to add it to the members slice
	members = append(members, state.NewCachedMember(m))
	shard.members[m.GuildID] = members
}

func (shard *SimpleStateTrackerShard) handleMemberDelete(mr *discordgo.GuildMemberRemove) {
	shard.mu.Lock()
	defer shard.mu.Unlock()

	// Update the memebr count
	gs, ok := shard.guilds[mr.GuildID]
	if !ok {
		return
	}

	newGS := gs.copyGuild()
	newGS.Guild.MemberCount--
	shard.guilds[mr.GuildID] = newGS

	// remove member from state
	if members, ok := shard.members[mr.GuildID]; ok {
		for i, v := range members {
			if v.ID == mr.User.ID {
				shard.members[mr.GuildID] = append(members[:i], members[i+1:]...)
				return
			}
		}
	}

}

///////////////////
// Message events
///////////////////

func (shard *SimpleStateTrackerShard) handleMessageCreate(m *discordgo.MessageCreate) {
	shard.mu.Lock()
	defer shard.mu.Unlock()

	if m.GuildID == 0 {
		return
	}

	if cl, ok := shard.messages[m.ChannelID]; ok {
		cl.PushBack(state.NewCachedMessage(m.Message))

		// clean up overflow
		limit := shard.conf.ChannelMessageLen
		if shard.conf.ChannelMessageLimitsF != nil {
			limit, _ = shard.conf.ChannelMessageLimitsF(m.GuildID)
		}

		if limit > 0 {
			overflow := cl.Len() - limit
			for i := overflow; i > 0; i-- {
				cl.Remove(cl.Front())
			}
		}
	} else {
		cl := list.New()
		cl.PushBack(state.NewCachedMessage(m.Message))
		shard.messages[m.ChannelID] = cl
	}
}

func (shard *SimpleStateTrackerShard) handleMessageUpdate(m *discordgo.MessageUpdate) {
	shard.mu.Lock()
	defer shard.mu.Unlock()

	if m.GuildID == 0 {
		return
	}

	if cl, ok := shard.messages[m.ChannelID]; ok {
		for e := cl.Back(); e != nil; e = e.Prev() {
			// do something with e.Value
			cast := e.Value.(*state.CachedMessage)
			if cast.ID == m.ID {
				// Update the message
				cop := *cast

				if m.Content != "" {
					cop.Content = m.Content
				}

				if m.Mentions != nil {
					cop.Mentions = make([]discordgo.User, len(m.Mentions))
					for i, v := range m.Mentions {
						cop.Mentions[i] = *v
					}
				}
				if m.Embeds != nil {
					cop.Embeds = make([]discordgo.MessageEmbed, len(m.Embeds))
					for i, v := range m.Embeds {
						cop.Embeds[i] = *v
					}
				}

				if m.Attachments != nil {
					cop.Attachments = make([]discordgo.MessageAttachment, len(m.Attachments))
					for i, v := range m.Attachments {
						cop.Attachments[i] = *v
					}
				}

				if m.Author != nil {
					cop.Author = *m.Author
				}

				if m.MentionRoles != nil {
					cop.MentionRoles = m.MentionRoles
				}

				e.Value = &cop
				// m.parseTimes(msg.Timestamp, msg.EditedTimestamp)
			}
		}
	}
}

func (shard *SimpleStateTrackerShard) handleMessageDelete(m *discordgo.MessageDelete) {
	shard.mu.Lock()
	defer shard.mu.Unlock()

	if m.GuildID == 0 {
		return
	}

	if cl, ok := shard.messages[m.ChannelID]; ok {
		for e := cl.Back(); e != nil; e = e.Prev() {
			cast := e.Value.(*state.CachedMessage)

			if cast.ID == m.ID {
				cl.Remove(e)
				return
			}
		}
	}
}

func (shard *SimpleStateTrackerShard) handleMessageDeleteBulk(m *discordgo.MessageDeleteBulk) {
	shard.mu.Lock()
	defer shard.mu.Unlock()

	if m.GuildID == 0 {
		return
	}

	if cl, ok := shard.messages[m.ChannelID]; ok {
		for e := cl.Back(); e != nil; e = e.Prev() {
			cast := e.Value.(*state.CachedMessage)

			for _, delID := range m.Messages {
				if delID == cast.ID {
					// since we remove it, we need to update our looping cursor aswell
					newNext := e.Next()

					cl.Remove(e)

					if newNext == nil {
						e = cl.Back()
					} else {
						e = newNext
					}

					break
				}
			}
		}
	}
}

///////////////////
// MISC events
///////////////////

func (shard *SimpleStateTrackerShard) handlePresenceUpdate(p *discordgo.PresenceUpdate) {
	shard.mu.Lock()
	defer shard.mu.Unlock()
}

func (shard *SimpleStateTrackerShard) handleVoiceStateUpdate(p *discordgo.VoiceStateUpdate) {
	shard.mu.Lock()
	defer shard.mu.Unlock()

	gs, ok := shard.guilds[p.GuildID]
	if !ok {
		return
	}

	newGS := gs.copyVoiceStates()
	for i, v := range newGS.VoiceStates {
		if v.UserID == p.UserID {
			if p.ChannelID == 0 {
				// Left voice chat entirely, remove us
				newGS.VoiceStates = append(newGS.VoiceStates[:i], newGS.VoiceStates[i+1:]...)
			} else {
				// just changed state
				newGS.VoiceStates[i] = p.VoiceState
			}
			return
		}
	}
}

func (shard *SimpleStateTrackerShard) handleReady(p *discordgo.Ready) {
	shard.mu.Lock()
	defer shard.mu.Unlock()

	shard.reset()

	for _, v := range p.Guilds {
		shard.guilds[v.ID] = &SparseGuildState{
			Guild: state.NewCachedGuild(v),
		}
	}
}

func (shard *SimpleStateTrackerShard) handleEmojis(e *discordgo.GuildEmojisUpdate) {
	shard.mu.Lock()
	defer shard.mu.Unlock()

	gs, ok := shard.guilds[e.GuildID]
	if !ok {
		return
	}

	newGS := gs.copyGuildSet()
	newGS.Emojis = make([]*state.CachedEmoji, 0, len(e.Emojis))
	for _, v := range e.Emojis {
		newGS.Emojis = append(newGS.Emojis, state.NewCachedEmoji(v))
	}
	shard.guilds[e.GuildID] = newGS
}

// assumes state is locked
func (shard *SimpleStateTrackerShard) reset() {
	shard.guilds = make(map[int64]*SparseGuildState)
	shard.members = make(map[int64][]*state.CachedMember)
	shard.messages = make(map[int64]*list.List)
}
