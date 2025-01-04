// Discordgo - Discord bindings for Go
// Available at https://github.com/bwmarrin/discordgo

// Copyright 2015-2016 Bruce Marriner <bruce@sqls.net>.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains low level functions for interacting with the Discord
// data websocket interface.

package discordgo

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/lib/gojay"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

var (
	ErrAlreadyOpen = errors.New("connection already open")
)

type GatewayIntent int

const (
	GatewayIntentGuilds GatewayIntent = 1 << 0
	// - GUILD_CREATE
	// - GUILD_UPDATE
	// - GUILD_DELETE
	// - GUILD_ROLE_CREATE
	// - GUILD_ROLE_UPDATE
	// - GUILD_ROLE_DELETE
	// - CHANNEL_CREATE
	// - CHANNEL_UPDATE
	// - CHANNEL_DELETE
	// - CHANNEL_PINS_UPDATE
	// - THREAD_CREATE
	// - THREAD_UPDATE
	// - THREAD_DELETE
	// - THREAD_LIST_SYNC
	// - THREAD_MEMBER_UPDATE
	// - THREAD_MEMBERS_UPDATE
	// - STAGE_INSTANCE_CREATE
	// - STAGE_INSTANCE_UPDATE
	// - STAGE_INSTANCE_DELETE

	GatewayIntentGuildMembers GatewayIntent = 1 << 1
	// - GUILD_MEMBER_ADD
	// - GUILD_MEMBER_UPDATE
	// - GUILD_MEMBER_REMOVE
	// - THREAD_MEMBERS_UPDATE

	GatewayIntentGuildModeration GatewayIntent = 1 << 2
	// - GUILD_AUDIT_LOG_ENTRY_CREATE
	// - GUILD_BAN_ADD
	// - GUILD_BAN_REMOVE

	GatewayIntentGuildExpressions GatewayIntent = 1 << 3
	// - GUILD_EMOJIS_UPDATE
	// - GUILD_STICKERS_UPDATE
	// - GUILD_SOUNDBOARD_SOUND_CREATE
	// - GUILD_SOUNDBOARD_SOUND_UPDATE
	// - GUILD_SOUNDBOARD_SOUND_DELETE
	// - GUILD_SOUNDBOARD_SOUNDS_UPDATE

	GatewayIntentGuildIntegrations GatewayIntent = 1 << 4
	// - GUILD_INTEGRATIONS_UPDATE
	// - INTEGRATION_CREATE
	// - INTEGRATION_UPDATE
	// - INTEGRATION_DELETE

	GatewayIntentGuildWebhooks GatewayIntent = 1 << 5
	// - WEBHOOKS_UPDATE

	GatewayIntentGuildInvites GatewayIntent = 1 << 6
	// - INVITE_CREATE
	// - INVITE_DELETE

	GatewayIntentGuildVoiceStates GatewayIntent = 1 << 7
	// - VOICE_CHANNEL_EFFECT_SEND
	// - VOICE_STATE_UPDATE

	GatewayIntentGuildPresences GatewayIntent = 1 << 8
	// - PRESENCE_UPDATE

	GatewayIntentGuildMessages GatewayIntent = 1 << 9
	// - MESSAGE_CREATE
	// - MESSAGE_UPDATE
	// - MESSAGE_DELETE
	// - MESSAGE_DELETE_BULK

	GatewayIntentGuildMessageReactions GatewayIntent = 1 << 10
	// - MESSAGE_REACTION_ADD
	// - MESSAGE_REACTION_REMOVE
	// - MESSAGE_REACTION_REMOVE_ALL
	// - MESSAGE_REACTION_REMOVE_EMOJI

	GatewayIntentGuildMessageTyping GatewayIntent = 1 << 11
	// - TYPING_START

	GatewayIntentDirectMessages GatewayIntent = 1 << 12
	// - CHANNEL_CREATE
	// - MESSAGE_CREATE
	// - MESSAGE_UPDATE
	// - MESSAGE_DELETE
	// - CHANNEL_PINS_UPDATE

	GatewayIntentDirectMessageReactions GatewayIntent = 1 << 13
	// - MESSAGE_REACTION_ADD
	// - MESSAGE_REACTION_REMOVE
	// - MESSAGE_REACTION_REMOVE_ALL
	// - MESSAGE_REACTION_REMOVE_EMOJI

	GatewayIntentDirectMessageTyping GatewayIntent = 1 << 14
	// - TYPING_START

	GatewayIntentMessageContent GatewayIntent = 1 << 15

	GatewayIntentGuildScheduledEvents GatewayIntent = 1 << 16
	// - GUILD_SCHEDULED_EVENT_CREATE
	// - GUILD_SCHEDULED_EVENT_UPDATE
	// - GUILD_SCHEDULED_EVENT_DELETE
	// - GUILD_SCHEDULED_EVENT_USER_ADD
	// - GUILD_SCHEDULED_EVENT_USER_REMOVE

	GatewayIntentAutomoderationExecution GatewayIntent = 1 << 21
	// - AUTO_MODERATION_ACTION_EXECUTION

	GatewayIntentAutomoderationConfiguration GatewayIntent = 1 << 20
	// - AUTO_MODERATION_RULE_CREATE
	// - AUTO_MODERATION_RULE_UPDATE
	// - AUTO_MODERATION_RULE_DELETE
)

// max size of buffers before they're discarded (e.g after a big incmoing event)
const MaxIntermediaryBuffersSize = 10000

// GatewayIdentifyRatelimiter is if you need some custom identify ratelimit logic (if you're running shards across processes for example)
type GatewayIdentifyRatelimiter interface {
	RatelimitIdentify(shardID int) // Called whenever an attempted identify is made, can be called from multiple goroutines at the same time
}

// Standard implementation of the GatewayIdentifyRatelimiter
type StdGatewayIdentifyRatleimiter struct {
	ch   chan bool
	once sync.Once
}

func (rl *StdGatewayIdentifyRatleimiter) RatelimitIdentify(shardID int) {
	rl.once.Do(func() {
		rl.ch = make(chan bool)
		go func() {
			ticker := time.NewTicker(time.Second * 5)
			for {
				rl.ch <- true
				<-ticker.C
			}
		}()
	})

	<-rl.ch
}

// This is used at the package level because it can be used by multiple sessions
// !! Changing this after starting 1 or more gateway sessions will lead to undefined behaviour
var IdentifyRatelimiter GatewayIdentifyRatelimiter = &StdGatewayIdentifyRatleimiter{}

// GatewayOP represents a gateway operation
// see https://discordapp.com/developers/docs/topics/gateway#gateway-opcodespayloads-gateway-opcodes
type GatewayOP int

const (
	GatewayOPDispatch            GatewayOP = 0  // (Receive)
	GatewayOPHeartbeat           GatewayOP = 1  // (Send/Receive)
	GatewayOPIdentify            GatewayOP = 2  // (Send)
	GatewayOPStatusUpdate        GatewayOP = 3  // (Send)
	GatewayOPVoiceStateUpdate    GatewayOP = 4  // (Send)
	GatewayOPVoiceServerPing     GatewayOP = 5  // (Send)
	GatewayOPResume              GatewayOP = 6  // (Send)
	GatewayOPReconnect           GatewayOP = 7  // (Receive)
	GatewayOPRequestGuildMembers GatewayOP = 8  // (Send)
	GatewayOPInvalidSession      GatewayOP = 9  // (Receive)
	GatewayOPHello               GatewayOP = 10 // (Receive)
	GatewayOPHeartbeatACK        GatewayOP = 11 // (Receive)
)

type GatewayStatus int

const (
	GatewayStatusDisconnected GatewayStatus = iota
	GatewayStatusConnecting
	GatewayStatusIdentifying
	GatewayStatusResuming
	GatewayStatusReady
)

func (gs GatewayStatus) String() string {
	switch gs {
	case GatewayStatusDisconnected:
		return "Disconnected"
	case GatewayStatusConnecting:
		return "Connecting"
	case GatewayStatusIdentifying:
		return "Identifying"
	case GatewayStatusResuming:
		return "Resuming"
	case GatewayStatusReady:
		return "Ready"
	}

	return "??"
}

// GatewayConnectionManager is responsible for managing the gateway connections for a single shard
// We create a new GatewayConnection every time we reconnect to avoid a lot of synchronization needs
// and also to avoid having to manually reset the connection state, all the workers related to the old connection
// should eventually stop, and if they're late they will be working on a closed connection anyways so it dosen't matter
type GatewayConnectionManager struct {
	mu     sync.RWMutex
	openmu sync.Mutex

	voiceConnections map[int64]*VoiceConnection

	// stores sessions current Discord Gateway
	gateway string

	shardCount int
	shardID    int

	session           *Session
	currentConnection *GatewayConnection
	status            GatewayStatus

	sessionID        string
	sequence         int64
	resumeGatewayUrl string

	idCounter int

	errorStopReconnects error // set when an error occurs that should stop reconnects (such as bad token, and other things)
}

func (s *GatewayConnectionManager) SetSessionInfo(sessionID string, sequence int64, resumeGatewayUrl string) {
	s.mu.Lock()
	s.sessionID = sessionID
	s.sequence = sequence
	s.resumeGatewayUrl = resumeGatewayUrl
	s.mu.Unlock()
}

func (s *GatewayConnectionManager) GetSessionInfo() (sessionID string, sequence int64, resumeGatewayUrl string) {
	s.mu.RLock()
	sessionID = s.sessionID
	sequence = s.sequence
	resumeGatewayUrl = s.resumeGatewayUrl
	s.mu.RUnlock()
	return
}

// Open is a helper for Session.GatewayConnectionManager.Open()
func (s *Session) Open() error {
	return s.GatewayManager.Open()
}

var (
	ErrBadAuth        = errors.New("authentication failed")
	ErrInvalidIntent  = errors.New("one of the gateway intents passed was invalid")
	ErrDisabledIntent = errors.New("an intent you specified has not been enabled or not been whitelisted for")
	ErrInvalidShard   = errors.New("you specified a invalid sharding setup")
)

func (g *GatewayConnectionManager) Open() error {
	g.session.log(LogInformational, " called")

	g.openmu.Lock()
	defer g.openmu.Unlock()

	g.mu.Lock()
	if g.errorStopReconnects != nil {
		g.mu.Unlock()
		return g.errorStopReconnects
	}

	if g.currentConnection != nil {
		cc := g.currentConnection
		g.currentConnection = nil

		g.mu.Unlock()
		cc.Close()
		g.mu.Lock()
	}

	g.idCounter++

	if g.gateway == "" {
		gatewayAddr, err := g.session.Gateway()
		if err != nil {
			g.mu.Unlock()
			return err
		}
		g.gateway = gatewayAddr
	}

	g.initSharding()

	newConn := NewGatewayConnection(g, g.idCounter, g.session.Intents)

	// Opening may be a long process, with ratelimiting and whatnot
	// we wanna be able to query things like status in the meantime
	g.mu.Unlock()
	err := newConn.open(g.sessionID, g.sequence, g.resumeGatewayUrl)
	g.mu.Lock()

	g.currentConnection = newConn

	g.session.log(LogInformational, "reconnecting voice connections")
	for _, vc := range g.voiceConnections {
		go func(voiceConn *VoiceConnection, gwc *GatewayConnectionManager) {
			gwc.session.log(LogInformational, "reconnecting voice connection: %d", voiceConn.GuildID)
			voiceConn.Lock()
			voiceConn.gatewayConn = newConn
			voiceConn.Unlock()
		}(vc, g)
		// go vc.reconnect(newConn)
	}

	g.mu.Unlock()

	return err
}

// initSharding sets the sharding details and verifies that they are valid
func (g *GatewayConnectionManager) initSharding() {
	g.shardCount = g.session.ShardCount
	if g.shardCount < 1 {
		g.shardCount = 1
	}

	g.shardID = g.session.ShardID
	if g.shardID >= g.shardCount || g.shardID < 0 {
		g.mu.Unlock()
		panic("Invalid shardID: ID:" + strconv.Itoa(g.shardID) + " Count:" + strconv.Itoa(g.shardCount))
	}
}

// Status returns the status of the current active connection
func (g *GatewayConnectionManager) Status() GatewayStatus {
	cc := g.GetCurrentConnection()
	if cc == nil {
		return GatewayStatusDisconnected
	}

	return cc.Status()
}

func (g *GatewayConnectionManager) HeartBeatStats() (lastSend time.Time, lastAck time.Time) {
	conn := g.GetCurrentConnection()
	if conn == nil {
		return
	}

	lastSend, lastAck = conn.heartbeater.Times()
	return
}

func (g *GatewayConnectionManager) RequestGuildMembers(guildID int64, query string, limit int) {
	conn := g.GetCurrentConnection()
	if conn == nil {
		return
	}

	conn.RequestGuildMembers(&RequestGuildMembersData{
		GuildID: guildID,
		Query:   &query,
		Limit:   limit,
	})
}

func (g *GatewayConnectionManager) RequestGuildMemberByID(guildID int64, query int64, limit int) {
	conn := g.GetCurrentConnection()
	if conn == nil {
		return
	}

	conn.RequestGuildMembers(&RequestGuildMembersData{
		GuildID: guildID,
		Limit:   limit,
		UserIDs: []int64{query},
	})
}

func (g *GatewayConnectionManager) RequestGuildMembersComplex(d *RequestGuildMembersData) {
	conn := g.GetCurrentConnection()
	if conn == nil {
		return
	}

	conn.RequestGuildMembers(d)
}

func (g *GatewayConnectionManager) GetCurrentConnection() *GatewayConnection {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.currentConnection
}

type voiceChannelJoinData struct {
	GuildID   *string `json:"guild_id"`
	ChannelID *string `json:"channel_id"`
	SelfMute  bool    `json:"self_mute"`
	SelfDeaf  bool    `json:"self_deaf"`
}

// ChannelVoiceJoin joins the session user to a voice channel.
//
//	gID     : Guild ID of the channel to join.
//	cID     : Channel ID of the channel to join.
//	mute    : If true, you will be set to muted upon joining.
//	deaf    : If true, you will be set to deafened upon joining.
func (g *GatewayConnectionManager) ChannelVoiceJoin(gID, cID int64, mute, deaf bool) (voice *VoiceConnection, err error) {

	g.session.log(LogInformational, "called")
	debug.PrintStack()

	g.mu.Lock()
	voice = g.voiceConnections[gID]

	if voice == nil {
		voice = &VoiceConnection{
			gatewayConnManager: g,
			gatewayConn:        g.currentConnection,
			GuildID:            gID,
			session:            g.session,
			Connected:          make(chan bool),
			LogLevel:           g.session.LogLevel,
			Debug:              g.session.Debug,
		}

		g.voiceConnections[gID] = voice
	}
	g.mu.Unlock()

	voice.Lock()
	voice.ChannelID = cID
	voice.deaf = deaf
	voice.mute = mute
	voice.Unlock()

	strGID := StrID(gID)
	strCID := StrID(cID)

	// Send the request to Discord that we want to join the voice channel
	op := outgoingEvent{
		Operation: GatewayOPVoiceStateUpdate,
		Data:      voiceChannelJoinData{&strGID, &strCID, mute, deaf},
	}

	g.mu.Lock()
	if g.currentConnection == nil {
		g.mu.Unlock()
		return nil, errors.New("bot not connected to gateway")
	}
	cc := g.currentConnection
	g.mu.Unlock()

	cc.writer.Queue(op)

	// doesn't exactly work perfect yet.. TODO
	err = voice.waitUntilConnected()
	if err != nil {
		cc.log(LogWarning, "error waiting for voice to connect, %s", err)
		voice.Close()

		// force remove it just incase
		g.mu.Lock()
		if g.voiceConnections[gID] == voice {
			delete(g.voiceConnections, gID)
		}
		g.mu.Unlock()

		return
	}

	return
}

func (g *GatewayConnectionManager) ChannelVoiceLeave(gID int64) {
	g.mu.RLock()
	cc := g.currentConnection
	g.mu.RUnlock()

	if cc == nil {
		return
	}

	strGID := strconv.FormatInt(gID, 10)
	data := outgoingEvent{
		Operation: GatewayOPVoiceStateUpdate,
		Data:      voiceChannelJoinData{&strGID, nil, true, true},
	}

	cc.writer.Queue(data)
}

// onVoiceStateUpdate handles Voice State Update events on the data websocket.
func (g *GatewayConnectionManager) onVoiceStateUpdate(st *VoiceStateUpdate) {

	// If we don't have a connection for the channel, don't bother
	if st.ChannelID == 0 {
		return
	}

	// Check if we have a voice connection to update
	g.mu.Lock()
	voice, exists := g.voiceConnections[st.GuildID]
	if !exists {
		g.mu.Unlock()
		return
	}

	// We only care about events that are about us.
	if g.session.State.User.ID != st.UserID {
		g.mu.Unlock()
		return
	}

	if st.ChannelID == 0 {
		g.session.log(LogInformational, "Deleting VoiceConnection %d", st.GuildID)
		delete(g.voiceConnections, st.GuildID)
		g.mu.Unlock()
		return
	}

	g.mu.Unlock()

	// Store the SessionID for later use.
	voice.Lock()
	voice.UserID = st.UserID
	voice.sessionID = st.SessionID
	voice.ChannelID = st.ChannelID
	voice.Unlock()
}

// onVoiceServerUpdate handles the Voice Server Update data websocket event.
//
// This is also fired if the Guild's voice region changes while connected
// to a voice channel.  In that case, need to re-establish connection to
// the new region endpoint.
func (g *GatewayConnectionManager) onVoiceServerUpdate(st *VoiceServerUpdate) {

	g.session.log(LogInformational, "called")

	g.mu.RLock()
	voice, exists := g.voiceConnections[st.GuildID]
	g.mu.RUnlock()

	// If no VoiceConnection exists, just skip this
	if !exists {
		return
	}

	// If currently connected to voice ws/udp, then disconnect.
	// Has no effect if not connected.
	voice.Close()

	// Store values for later use
	voice.Lock()
	voice.token = st.Token
	voice.endpoint = st.Endpoint
	voice.GuildID = st.GuildID
	voice.Unlock()

	// Open a connection to the voice server
	err := voice.open()
	if err != nil {
		g.session.log(LogError, "onVoiceServerUpdate voice.open, %s", err)
	}
}

// Close maintains backwards compatibility with old discordgo versions
// It's the same as s.GatewayManager.Close()
func (s *Session) Close() error {
	return s.GatewayManager.Close()
}

func (g *GatewayConnectionManager) Close() (err error) {
	g.mu.Lock()
	if g.currentConnection != nil {
		g.mu.Unlock()
		err = g.currentConnection.Close()
		g.mu.Lock()
		g.currentConnection = nil
	}
	g.mu.Unlock()
	return
}

func (g *GatewayConnectionManager) Reconnect(forceIdentify bool) error {
	g.mu.RLock()
	currentConn := g.currentConnection
	g.mu.RUnlock()

	if currentConn != nil {
		err := currentConn.Reconnect(forceIdentify)
		return err
	}

	return nil
}

type GatewayConnection struct {
	mu sync.Mutex

	// The parent manager
	manager *GatewayConnectionManager
	intents []GatewayIntent

	opened         bool
	workersRunning bool
	reconnecting   bool
	status         GatewayStatus

	// Stores a mapping of guild id's to VoiceConnections
	voiceConnections map[string]*VoiceConnection

	// The underlying websocket connection.
	conn net.Conn

	// stores session ID of current Gateway connection
	sessionID string

	// stores url to resume connection
	resumeGatewayUrl string

	// This gets closed when the connection closes to signal all workers to stop
	stopWorkers chan interface{}

	wsReader *wsutil.Reader

	// contains the raw message fragments until we have received them all
	readMessageBuffer *bytes.Buffer

	zlibReader             io.Reader
	jsonDecoder            *gojay.Decoder
	teeReader              io.Reader
	secondPassJsonDecoder  *json.Decoder
	secondPassGojayDecoder *gojay.Decoder
	secondPassBuf          *bytes.Buffer

	heartbeater *wsHeartBeater
	writer      *wsWriter

	connID int // A increasing id per connection from the connection manager to help identify the origin of logs

	decodedBuffer bytes.Buffer

	// so we dont need to re-allocate a event on each event
	event *Event
}

func NewGatewayConnection(parent *GatewayConnectionManager, id int, intents []GatewayIntent) *GatewayConnection {
	gwc := &GatewayConnection{
		manager:           parent,
		stopWorkers:       make(chan interface{}),
		readMessageBuffer: bytes.NewBuffer(make([]byte, 0, 0xffff)), // initial 65k buffer
		connID:            id,
		status:            GatewayStatusConnecting,
		event:             &Event{},
		secondPassBuf:     &bytes.Buffer{},
		intents:           intents,
	}

	secondPassJson := json.NewDecoder(gwc.secondPassBuf)
	gwc.secondPassJsonDecoder = secondPassJson
	gwc.secondPassGojayDecoder = gojay.NewDecoder(gwc.secondPassBuf)

	return gwc
}

func (g *GatewayConnection) concurrentReconnect(forceReIdentify bool) {
	go func() {
		err := g.Reconnect(forceReIdentify)
		if err != nil {
			g.log(LogError, "failed reconnecting to the gateway: %v", err)
		}
	}()
}

// Reconnect is a helper for Close() and Connect() and will attempt to resume if possible
func (g *GatewayConnection) Reconnect(forceReIdentify bool) error {
	g.mu.Lock()
	if g.reconnecting {
		g.mu.Unlock()
		g.log(LogInformational, "attempted to reconnect to the gateway while already reconnecting")
		return nil
	}

	g.log(LogInformational, "reconnecting to the gateway")
	debug.PrintStack()

	g.reconnecting = true

	if forceReIdentify {
		g.sessionID = ""
	}

	g.mu.Unlock()

	err := g.Close()
	if err != nil {
		return err
	}

	return g.manager.Open()
}

// ReconnectUnlessStopped will not reconnect if close was called earlier
func (g *GatewayConnection) ReconnectUnlessClosed(forceReIdentify bool) error {
	select {
	case <-g.stopWorkers:
		return nil
	default:
		return g.Reconnect(forceReIdentify)
	}
}

// Close closes the gateway connection
func (g *GatewayConnection) Close() error {
	g.mu.Lock()

	g.status = GatewayStatusDisconnected

	sidCop := g.sessionID
	seqCop := atomic.LoadInt64(g.heartbeater.sequence)

	// If were not actually connected then do nothing
	wasRunning := g.workersRunning
	g.workersRunning = false
	if g.conn == nil {
		if wasRunning {
			close(g.stopWorkers)
		}

		g.mu.Unlock()

		g.manager.mu.Lock()
		g.manager.sessionID = sidCop
		g.manager.sequence = seqCop
		g.manager.mu.Unlock()
		return nil
	}

	g.log(LogInformational, "closing gateway connection")

	// copy these here to later be assigned to the manager for possible resuming

	g.mu.Unlock()

	if wasRunning {
		// Send the close frame
		frame := ws.NewCloseFrame(ws.NewCloseFrameBody(
			4000, "o7",
		))

		frame = ws.MaskFrameInPlace(frame)
		compiled := ws.MustCompileFrame(frame)
		g.writer.QueueClose(compiled)

		close(g.stopWorkers)

		started := time.Now()

		// Wait for discord to close connnection
		for {
			time.Sleep(time.Millisecond * 100)
			g.mu.Lock()
			if g.conn == nil {
				g.mu.Unlock()
				break
			}

			// Yes, this actually does happen...
			if time.Since(started) > time.Second*5 {
				g.log(LogWarning, "dead connection")
				g.conn.Close()
				g.mu.Unlock()
				break
			}
			g.mu.Unlock()
		}

		g.mu.Lock()
		sidCop = g.sessionID
		seqCop = atomic.LoadInt64(g.heartbeater.sequence)
		g.mu.Unlock()

		g.manager.mu.Lock()
		g.manager.sessionID = sidCop
		g.manager.sequence = seqCop
		g.manager.mu.Unlock()
	}

	return nil
}

func newUpdateStatusData(activityType ActivityType, statusType Status, statusText, streamingUrl string) *UpdateStatusData {
	usd := &UpdateStatusData{
		AFK:    false,
		Status: statusType,
	}
	now := int(time.Now().Unix())
	if statusType == StatusIdle {
		usd.IdleSince = &now
	}

	if statusText != "" {
		usd.Activity = &Activity{
			Name:  statusText,
			State: statusText,
			Type:  activityType,
			URL:   streamingUrl,
		}
	}
	return usd
}

// UpdateStatus is used to update the user's status.
// Set the custom status to statusText.
// Set the online status to statusType.
func (s *Session) UpdateStatus(activityType ActivityType, statusType Status, statusText, streamingUrl string) (err error) {
	if streamingUrl != "" {
		activityType = ActivityTypeStreaming
	}
	return s.UpdateStatusComplex(*newUpdateStatusData(activityType, statusType, statusText, streamingUrl))
}

// UpdatePlayingStatus is used to update the user's playing status.
// Set the game being played to status.
// Set the online status to statusType.
func (s *Session) UpdatePlayingStatus(statusText string, statusType Status) (err error) {
	return s.UpdateStatusComplex(*newUpdateStatusData(ActivityTypePlaying, statusType, statusText, ""))
}

// UpdateStreamingStatus is used to update the user's streaming status.
// Set the name of the stream to status.
// Set the online status to statusType.
// Set the stream URL to url.
func (s *Session) UpdateStreamingStatus(statusText string, statusType Status, url string) (err error) {
	return s.UpdateStatusComplex(*newUpdateStatusData(ActivityTypeStreaming, statusType, statusText, url))
}

// UpdateListeningStatus is used to update the user's listening status
// Set what the user is listening to to status.
// Set the online status to statusType.
func (s *Session) UpdateListeningStatus(statusText string, statusType Status) (err error) {
	return s.UpdateStatusComplex(*newUpdateStatusData(ActivityTypeListening, statusType, statusText, ""))
}

// UpdateWatchingStatus is used to update the user's watching status
// Set what the user is watching to status.
// Set the online status to statusType.
func (s *Session) UpdateWatchingStatus(statusText string, statusType Status) (err error) {
	return s.UpdateStatusComplex(*newUpdateStatusData(ActivityTypeWatching, statusType, statusText, ""))
}

// UpdateCustomStatus is used to update the user's custom status
// Set the user's custom text to status.
// Set the online status to statusType.
func (s *Session) UpdateCustomStatus(statusText string, statusType Status) (err error) {
	return s.UpdateStatusComplex(*newUpdateStatusData(ActivityTypeCustom, statusType, statusText, ""))
}

// UpdateCompetingStatus is used to update the user's competing status
// Set what the user is competing in to status.
// Set the online status to statusType.
func (s *Session) UpdateCompetingStatus(statusText string, statusType Status) (err error) {
	return s.UpdateStatusComplex(*newUpdateStatusData(ActivityTypeCompeting, statusType, statusText, ""))
}

func (s *Session) UpdateStatusComplex(usd UpdateStatusData) (err error) {
	curConn := s.GatewayManager.GetCurrentConnection()
	if curConn == nil {
		return errors.New("no gateway connection")
	}

	curConn.UpdateStatusComplex(usd)
	return nil
}

// UpdateStatusComplex allows for sending the raw status update data untouched by discordgo.
func (g *GatewayConnection) UpdateStatusComplex(usd UpdateStatusData) {
	g.writer.Queue(outgoingEvent{Operation: GatewayOPStatusUpdate, Data: usd})
}

// Status returns the current status of the connection
func (g *GatewayConnection) Status() (st GatewayStatus) {
	g.mu.Lock()
	st = g.status
	g.mu.Unlock()
	return
}

// Connect connects to the discord gateway and starts handling frames
func (g *GatewayConnection) open(sessionID string, sequence int64, resumeGatewayUrl string) error {
	g.mu.Lock()
	if g.opened {
		g.mu.Unlock()
		return ErrAlreadyOpen
	}

	var conn net.Conn
	var err error

	for {
		gatewayUrl := g.manager.gateway
		// if this is an intended resume, use the resume gateway url provided by discord
		if sessionID != "" && resumeGatewayUrl != "" {
			gatewayUrl = resumeGatewayUrl
		}
		conn, _, _, err = ws.Dial(context.TODO(), gatewayUrl+"?v="+APIVersion+"&encoding=json&compress=zlib-stream")
		if err != nil {
			if conn != nil {
				conn.Close()
			}
			g.log(LogError, "Failed opening connection to the gateway, retrying in 5 seconds: %v", err)
			time.Sleep(time.Second * 5)
			continue
		}

		break
	}

	g.log(LogInformational, "Connected to the gateway websocket")
	go g.manager.session.handleEvent(connectEventType, &Connect{})

	g.conn = conn
	g.opened = true
	g.wsReader = wsutil.NewClientSideReader(conn)

	err = g.startWorkers()
	if err != nil {
		return err
	}

	g.sessionID = sessionID

	g.mu.Unlock()

	if sessionID == "" {
		return g.identify()
	} else {
		g.heartbeater.UpdateSequence(sequence)
		return g.resume(sessionID, sequence)
	}
}

// startWorkers starts the background workers for reading, receiving and heartbeating
func (g *GatewayConnection) startWorkers() error {
	// The writer
	writerWorker := newWSWriter(g.conn, g.manager.session, g.stopWorkers)
	g.writer = writerWorker
	go writerWorker.Run()

	// The heartbeater, this is started after we receive hello
	g.heartbeater = &wsHeartBeater{
		stop:        g.stopWorkers,
		writer:      g.writer,
		receivedAck: true,
		sequence:    new(int64),
		onNoAck: func() {
			if g.conn == nil {
				return
			}
			g.log(LogError, "No heartbeat ack received since sending last heartbeast, reconnecting... ip: %s", g.conn.RemoteAddr().String())
			err := g.ReconnectUnlessClosed(false)
			if err != nil {
				g.log(LogError, "Failed reconnecting to the gateway: %v", err)
			}
		},
	}

	// Start the event reader
	go g.reader()

	g.workersRunning = true

	return nil
}

// reader reads incmoing messages from the gateway
func (g *GatewayConnection) reader() {

	// The buffer that is used to read into the bytes buffer
	// We need to control the amount read so we can't use buffer.ReadFrom directly
	// TODO: write it directly into the bytes buffer somehow to avoid a uneeded copy?
	intermediateBuffer := make([]byte, 0xffff)

	for {
		header, err := g.wsReader.NextFrame()
		if err != nil {
			g.readerError(err, "error reading next gateway message: %v", err)
			return
		}

		for readAmount := int64(0); readAmount < header.Length; {

			n, err := g.wsReader.Read(intermediateBuffer)
			if err != nil && (int64(n)+readAmount) != header.Length {
				g.readerError(err, "error reading the next websocket frame into intermediate buffer (n %d, l %d, hl %d): %v", n, readAmount, header.Length, err)
				return
			}

			if n != 0 {
				// g.log(LogInformational, base64.URLEncoding.EncodeToString(intermediateBuffer[:n]))
				g.readMessageBuffer.Write(intermediateBuffer[:n])
			}

			readAmount += int64(n)
		}

		if !header.Fin {
			continue // read the rest
		}

		g.handleReadFrame(header)
	}
}

func (g *GatewayConnection) readerError(err error, msgf string, args ...interface{}) {
	// There was an error reading the next frame, close the connection and trigger a reconnect
	g.mu.Lock()
	g.conn.Close()
	g.conn = nil
	g.mu.Unlock()

	select {
	case <-g.stopWorkers:
		// A close/reconnect was triggered somewhere else, do nothing
	default:
		go g.onError(err, msgf, args...)
	}

	go g.manager.session.handleEvent(disconnectEventType, &Disconnect{})

}

var (
	endOfPacketSuffix = []byte{0x0, 0x0, 0xff, 0xff}
)

// handleReadFrame handles a copmletely read frame
func (g *GatewayConnection) handleReadFrame(header ws.Header) {
	if header.OpCode == ws.OpClose {
		g.handleCloseFrame(g.readMessageBuffer.Bytes())
		g.readMessageBuffer.Reset()
		return
	}

	// TODO: Handle these properly
	if header.OpCode != ws.OpText && header.OpCode != ws.OpBinary {
		g.readMessageBuffer.Reset()
		g.log(LogError, "Don't know how to respond to websocket frame type: 0x%x", header.OpCode)
		return
	}

	// Package is not long enough to keep the end of message suffix, so we need to wait for more data
	if header.Length < 4 {
		return
	}

	// Check if it's the end of the message, the frame should have the suffix 0x00 0x00 0xff 0xff
	raw := g.readMessageBuffer.Bytes()
	tail := raw[len(raw)-4:]

	if !bytes.Equal(tail, endOfPacketSuffix) {
		g.log(LogInformational, "Not the end %d", len(tail))
		return
	}

	g.handleReadMessage()
}

// handleCloseFrame handles a close frame
func (g *GatewayConnection) handleCloseFrame(data []byte) {
	code := binary.BigEndian.Uint16(data)
	var msg string
	if len(data) > 2 {
		msg = string(data[2:])
	}

	g.log(LogError, "got close frame, code: %d, Msg: %q", code, msg)

	go func() {

		if code != 4004 && code != 4013 && code != 4014 && code != 4010 {
			err := g.ReconnectUnlessClosed(false)
			if err != nil {
				g.log(LogError, "failed reconnecting to the gateway: %v", err)
			}

			return
		}

		g.manager.mu.Lock()

		switch code {
		case 4004:
			g.manager.errorStopReconnects = ErrBadAuth
			g.log(LogError, "Authentication failed")
		case 4013:
			g.manager.errorStopReconnects = ErrInvalidIntent
			g.log(LogError, "Invalid intent passed to gateway open")
		case 4014:
			g.manager.errorStopReconnects = ErrDisabledIntent
			g.log(LogError, "Disabled or not whitelisted for one of the intents passed to gateway open")
		case 4010:
			g.manager.errorStopReconnects = ErrInvalidShard
			g.log(LogError, "Invalid shard specified")
		}

		g.manager.mu.Unlock()

		g.Close()
	}()
}

// handleReadMessage is called when we have received a full message
// it decodes the message into an event using a shared zlib context
func (g *GatewayConnection) handleReadMessage() {

	readLen := g.readMessageBuffer.Len()

	if g.zlibReader == nil {
		// We initialize the zlib reader here as opposed to in NewGatewayConntection because
		// zlib.NewReader apperently needs the header straight away, or it will block forever
		zr, err := zlib.NewReader(g.readMessageBuffer)
		if err != nil {
			go g.onError(err, "failed creating zlib reader")
			return

		}

		g.zlibReader = zr

		g.teeReader = io.TeeReader(zr, &g.decodedBuffer)
		g.jsonDecoder = gojay.NewDecoder(g.teeReader)
	}

	defer g.decodedBuffer.Reset()

	err := g.jsonDecoder.Decode(g.event)
	g.jsonDecoder.Reset()
	// g.log(LogInformational, "%s", g.decodedBuffer.String())
	if err != nil {
		go g.onError(err, "failed decoding incoming gateway event: %s", g.decodedBuffer.String())
		return
	}

	if g.decodedBuffer.Cap() > MaxIntermediaryBuffersSize && readLen < MaxIntermediaryBuffersSize {
		maybeThrowawayBytesBuf(&g.decodedBuffer, MaxIntermediaryBuffersSize)
		g.jsonDecoder = gojay.NewDecoder(g.teeReader)
	}

	if readLen < MaxIntermediaryBuffersSize {
		maybeThrowawayBytesBuf(g.readMessageBuffer, MaxIntermediaryBuffersSize)
	}

	g.handleEvent(g.event)
}

// handleEvent handles a event received from the reader
func (g *GatewayConnection) handleEvent(event *Event) {
	g.heartbeater.UpdateSequence(event.Sequence)

	var err error

	switch event.Operation {
	case GatewayOPDispatch:
		err = g.handleDispatch(event)
	case GatewayOPHeartbeat:
		g.log(LogInformational, "sending heartbeat immediately in response to OP1")
		go g.heartbeater.SendBeat()
	case GatewayOPReconnect:
		g.log(LogWarning, "got OP7 reconnect, re-connecting.")
		g.concurrentReconnect(false)
	case GatewayOPInvalidSession:
		time.Sleep(time.Second * time.Duration(rand.Intn(4)+1))

		if len(event.RawData) == 4 {
			// d == true, we can resume
			g.log(LogWarning, "got OP9 invalid session, re-indetifying. (resume) d: %v", string(event.RawData))
			g.concurrentReconnect(false)
		} else {
			g.log(LogWarning, "got OP9 invalid session, re-connecting. (no resume) d: %v", string(event.RawData))
			g.concurrentReconnect(true)
		}
	case GatewayOPHello:
		err = g.handleHello(event)
	case GatewayOPHeartbeatACK:
		g.heartbeater.ReceivedAck()
	default:
		g.log(LogWarning, "unknown operation (%d, %q): ", event.Operation, event.Type, string(event.RawData))
	}

	if err != nil {

		g.log(LogError, "error handling event (%d, %q)", event.Operation, event.Type)
	}
}

func (g *GatewayConnection) handleHello(event *Event) error {
	var h helloData
	err := json.Unmarshal(event.RawData, &h)
	if err != nil {
		return err
	}

	g.log(LogInformational, "receivied hello, heartbeat_interval: %d, _trace: %v", h.HeartbeatInterval, h.Trace)

	go g.heartbeater.Run(time.Duration(h.HeartbeatInterval) * time.Millisecond)

	return nil
}

func (g *GatewayConnection) handleDispatch(e *Event) error {

	size := len(e.RawData)

	// Map event to registered event handlers and pass it along to any registered handlers.
	if eh, ok := registeredInterfaceProviders[e.Type]; ok {
		e.Struct = eh.New()

		// Attempt to unmarshal our event.
		if gojayDec, ok := e.Struct.(gojay.UnmarshalerJSONObject); ok {
			// g.log(LogInformational, "Unmarshalling %s using gojay, %s", e.Type, g.secondPassBuf.String())
			g.secondPassBuf.Write(e.RawData)

			if err := g.secondPassGojayDecoder.Decode(gojayDec); err != nil {
				g.log(LogError, "error unmarshalling %s (gojay) event, %s, %s", e.Type, err, string(e.RawData))
			}

			g.secondPassGojayDecoder.Reset()
		} else {
			g.secondPassBuf.Write(e.RawData)
			if err := g.secondPassJsonDecoder.Decode(e.Struct); err != nil {
				g.log(LogError, "error unmarshalling %s event, %s, %s", e.Type, err, string(e.RawData))
			}
		}

		// if err := g.secondPassJsonDecoder.Decode(e.Struct); err != nil {
		// 	g.log(LogError, "error unmarshalling %s event, %s", e.Type, err)
		// }

		if rdy, ok := e.Struct.(*Ready); ok {
			g.handleReady(rdy)
		} else if r, ok := e.Struct.(*Resumed); ok {
			g.handleResumed(r)
		}

		// Send event to any registered event handlers for it's type.
		// Because the above doesn't cancel this, in case of an error
		// the struct could be partially populated or at default values.
		// However, most errors are due to a single field and I feel
		// it's better to pass along what we received than nothing at all.
		// TODO: Think about that decision :)
		// Either way, READY events must fire, even with errors.
		g.manager.session.handleEvent(e.Type, e.Struct)
	} else {
		g.log(LogWarning, "unknown event: Op: %d, Seq: %d, Type: %s, Data: %s", e.Operation, e.Sequence, e.Type, string(e.RawData))
	}

	if g.secondPassBuf.Cap() > MaxIntermediaryBuffersSize && size < MaxIntermediaryBuffersSize {
		maybeThrowawayBytesBuf(g.secondPassBuf, MaxIntermediaryBuffersSize)
		g.secondPassJsonDecoder = json.NewDecoder(g.secondPassBuf)
	}

	// For legacy reasons, we send the raw event also, this could be useful for handling unknown events.
	// Jonas: I've disabled this because i've dubbed it useless, events should be added in the library, handling unknown events elsewhere is added uneeded complexity
	// also we reuse the event object now so we can't
	// g.manager.session.handleEvent(eventEventType, e)

	return nil
}

// maybeThrowawayBytesBuf will recreate b if the capacity is above maxSize
// usefull because sometimes buffers only need to be big for a single event, then its kinda pointless to keep them around forever
func maybeThrowawayBytesBuf(b *bytes.Buffer, maxSize int) {
	if b.Cap() > maxSize {
		*b = bytes.Buffer{}
	}
}

func (g *GatewayConnection) handleReady(r *Ready) {
	g.log(LogInformational, "received ready")
	g.mu.Lock()
	g.sessionID = r.SessionID
	g.status = GatewayStatusReady
	g.resumeGatewayUrl = r.ResumeGatewayUrl
	// Ensure the gatewayUrl always has a trailing slash.
	// MacOS will fail to connect if we add query params without a trailing slash on the base domain.
	if !strings.HasSuffix(g.resumeGatewayUrl, "/") {
		g.resumeGatewayUrl += "/"
	}
	g.mu.Unlock()

	g.writer.readyRecv <- true

	g.manager.SetSessionInfo(r.SessionID, 0, r.ResumeGatewayUrl)
}

func (g *GatewayConnection) handleResumed(r *Resumed) {
	g.log(LogInformational, "received resumed")
	g.mu.Lock()
	g.status = GatewayStatusReady
	g.mu.Unlock()
}

func (g *GatewayConnection) identify() error {
	properties := identifyProperties{
		OS:              runtime.GOOS,
		Browser:         "Discordgo-jonas747_fork v" + VERSION,
		Device:          "",
		Referer:         "",
		ReferringDomain: "",
	}

	var intents *int
	if len(g.intents) > 0 {
		compiled := 0
		for _, v := range g.intents {
			compiled |= int(v)
		}
		intents = &compiled
	}

	data := identifyData{
		Token:          g.manager.session.Token,
		Properties:     properties,
		LargeThreshold: 250,
		// Compress:      g.manager.session.Compress, // this is no longer needed since we use zlib-steam anyways
		Shard:              nil,
		GuildSubscriptions: true,
		Intents:            intents,
	}

	if g.manager.shardCount > 1 {
		data.Shard = &[2]int{g.manager.shardID, g.manager.shardCount}
	}

	op := outgoingEvent{
		Operation: 2,
		Data:      data,
	}

	// check if we need to wait before identifying
	IdentifyRatelimiter.RatelimitIdentify(g.manager.shardID)

	g.log(LogInformational, "Sending identify")

	g.mu.Lock()
	g.status = GatewayStatusIdentifying
	g.mu.Unlock()

	g.writer.Queue(op)

	return nil
}

func (g *GatewayConnection) resume(sessionID string, sequence int64) error {
	op := outgoingEvent{
		Operation: GatewayOPResume,
		Data: &resumeData{
			Token:     g.manager.session.Token,
			SessionID: sessionID,
			Sequence:  sequence,
		},
	}

	g.log(LogInformational, "Sending resume")

	g.mu.Lock()
	g.status = GatewayStatusResuming
	g.mu.Unlock()

	g.writer.Queue(op)

	return nil
}

func (g *GatewayConnection) RequestGuildMembers(d *RequestGuildMembersData) {
	op := outgoingEvent{
		Operation: GatewayOPRequestGuildMembers,
		Data:      d,
	}

	g.log(LogInformational, "Sending request guild members")

	g.writer.Queue(op)
}

func (g *GatewayConnection) onError(err error, msgf string, args ...interface{}) {
	g.log(LogError, "%s: %s", fmt.Sprintf(msgf, args...), err.Error())
	if err := g.ReconnectUnlessClosed(false); err != nil {
		g.log(LogError, "Failed reconnecting to the gateway: %v", err)
	}
}

func (g *GatewayConnection) log(msgL int, msgf string, args ...interface{}) {
	if GatewayLogger != nil {
		GatewayLogger(g.manager.shardID, g.connID, msgL, msgf, args...)
		return
	}

	if msgL > g.manager.session.LogLevel {
		return
	}

	prefix := fmt.Sprintf("[S%d:CID%d]: ", g.manager.shardID, g.connID)
	msglog(msgL, 2, prefix+msgf, args...)
}

type outgoingEvent struct {
	Operation GatewayOP   `json:"op"`
	Type      string      `json:"t,omitempty"`
	Data      interface{} `json:"d,omitempty"`
}

type identifyData struct {
	Token              string             `json:"token"`
	Properties         identifyProperties `json:"properties"`
	LargeThreshold     int                `json:"large_threshold"`
	Compress           bool               `json:"compress"`
	GuildSubscriptions bool               `json:"guild_subscriptions"`
	Shard              *[2]int            `json:"shard,omitempty"`
	Intents            *int               `json:"intents,omitempty"`
}

type identifyProperties struct {
	OS              string `json:"$os"`
	Browser         string `json:"$browser"`
	Device          string `json:"$device"`
	Referer         string `json:"$referer"`
	ReferringDomain string `json:"$referring_domain"`
}

type helloData struct {
	HeartbeatInterval int64    `json:"heartbeat_interval"` // the interval (in milliseconds) the client should heartbeat with
	Trace             []string `json:"_trace"`
}

type resumeData struct {
	Token     string `json:"token"`
	SessionID string `json:"session_id"`
	Sequence  int64  `json:"seq"`
}

type UpdateStatusData struct {
	IdleSince *int      `json:"since"`
	Activity  *Activity `json:"game"`
	AFK       bool      `json:"afk"`
	Status    Status    `json:"status"`
}

type RequestGuildMembersData struct {
	GuildID   int64 `json:"guild_id,string"`
	Limit     int   `json:"limit"`
	Presences bool  `json:"presences"`

	Query   *string `json:"query,omitempty"`
	UserIDs IDSlice `json:"user_ids,omitempty"`
	Nonce   string  `json:"nonce"`
}
