package dshardmanager

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/pkg/errors"
)

const (
	VersionMajor = 0
	VersionMinor = 2
	VersionPath  = 0
)

var (
	VersionString = strconv.Itoa(VersionMajor) + "." + strconv.Itoa(VersionMinor) + "." + strconv.Itoa(VersionPath)
)

type SessionFunc func(token string) (*discordgo.Session, error)

type Manager struct {
	sync.RWMutex

	// Name of the bot, to appear before log messages as a prefix
	// and in the title of the updated status message
	Name string

	// All the shard sessions
	Sessions      []*discordgo.Session
	eventHandlers []interface{}

	// If set logs connection status events to this channel
	LogChannel int64

	// If set keeps an updated satus message in this channel
	StatusMessageChannel int64

	// The function that provides the guild counts per shard, used fro the updated status message
	// Should return a slice of guild counts, with the index being the shard number
	GuildCountsFunc func() []int

	// Called on events, by default this is set to a function that logs it to log.Printf
	// You can override this if you want another behaviour, or just set it to nil for nothing.
	OnEvent func(e *Event)

	// SessionFunc creates a new session and returns it, override the default one if you have your own
	// session settings to apply
	SessionFunc SessionFunc

	nextStatusUpdate     time.Time
	statusUpdaterStarted bool

	numShards int
	token     string

	bareSession *discordgo.Session
	started     bool
}

// New creates a new shard manager with the defaults set, after you have created this you call Manager.Start
// To start connecting
// dshardmanager.New("Bot asd", OptLogChannel(someChannel), OptLogEventsToDiscord(true, true))
func New(token string) *Manager {
	// Setup defaults
	manager := &Manager{
		token:     token,
		numShards: -1,
	}

	manager.OnEvent = manager.LogConnectionEventStd
	manager.SessionFunc = manager.StdSessionFunc

	manager.bareSession, _ = discordgo.New(token)

	return manager
}

// GetRecommendedCount gets the recommended sharding count from discord, this will also
// set the shard count internally if called
// Should not be called after calling Start(), will have undefined behaviour
func (m *Manager) GetRecommendedCount() (int, error) {
	resp, err := m.bareSession.GatewayBot()
	if err != nil {
		return 0, errors.WithMessage(err, "GetRecommendedCount()")
	}

	m.numShards = resp.Shards
	if m.numShards < 1 {
		m.numShards = 1
	}

	return m.numShards, nil
}

// GetNumShards returns the current set number of shards
func (m *Manager) GetNumShards() int {
	return m.numShards
}

// SetNumShards sets the number of shards to use, if you want to override the recommended count
// Should not be called after calling Start(), will panic
func (m *Manager) SetNumShards(n int) {
	m.Lock()
	defer m.Unlock()
	if m.started {
		panic("Can't set num shard after started")
	}

	m.numShards = n
}

// Adds an event handler to all shards
// All event handlers will be added to new sessions automatically.
func (m *Manager) AddHandler(handler interface{}) {
	m.Lock()
	defer m.Unlock()
	m.eventHandlers = append(m.eventHandlers, handler)

	if len(m.Sessions) > 0 {
		for _, v := range m.Sessions {
			v.AddHandler(handler)
		}
	}
}

// Init initializesthe manager, retreiving the recommended shard count if needed
// and initalizes all the shards
func (m *Manager) Init() error {
	m.Lock()
	if m.numShards < 1 {
		_, err := m.GetRecommendedCount()
		if err != nil {
			return errors.WithMessage(err, "Start")
		}
	}

	m.Sessions = make([]*discordgo.Session, m.numShards)
	for i := 0; i < m.numShards; i++ {
		err := m.initSession(i)
		if err != nil {
			m.Unlock()
			return errors.WithMessage(err, "initSession")
		}
	}

	if !m.statusUpdaterStarted {
		m.statusUpdaterStarted = true
		go m.statusRoutine()
	}

	m.nextStatusUpdate = time.Now()

	m.Unlock()

	return nil
}

// Start starts the shard manager, opening all gateway connections
func (m *Manager) Start() error {

	m.Lock()
	if m.Sessions == nil {
		m.Unlock()
		err := m.Init()
		if err != nil {
			return err
		}
		m.Lock()
	}

	m.Unlock()

	for i := 0; i < m.numShards; i++ {

		m.Lock()
		err := m.startSession(i)
		m.Unlock()
		if err != nil {
			return errors.WithMessage(err, fmt.Sprintf("Failed starting shard %d", i))
		}
	}

	return nil
}

// StopAll stops all the shard sessions and returns the last error that occured
func (m *Manager) StopAll() (err error) {
	m.Lock()
	for _, v := range m.Sessions {
		if e := v.Close(); e != nil {
			err = e
		}
	}
	m.Unlock()

	return
}

func (m *Manager) initSession(shard int) error {
	session, err := m.SessionFunc(m.token)
	if err != nil {
		return errors.WithMessage(err, "startSession.SessionFunc")
	}

	session.ShardCount = m.numShards
	session.ShardID = shard

	session.AddHandler(m.OnDiscordConnected)
	session.AddHandler(m.OnDiscordDisconnected)
	session.AddHandler(m.OnDiscordReady)
	session.AddHandler(m.OnDiscordResumed)

	// Add the user event handlers retroactively
	for _, v := range m.eventHandlers {
		session.AddHandler(v)
	}

	m.Sessions[shard] = session
	return nil
}

func (m *Manager) startSession(shard int) error {

	err := m.Sessions[shard].Open()
	if err != nil {
		return errors.Wrap(err, "startSession.Open")
	}
	m.handleEvent(EventOpen, shard, "")

	return nil
}

// SessionForGuildS is the same as SessionForGuild but accepts the guildID as a string for convenience
func (m *Manager) SessionForGuildS(guildID string) *discordgo.Session {
	// Question is, should we really ignore this error?
	// In reality, the guildID should never be invalid but...
	parsed, _ := strconv.ParseInt(guildID, 10, 64)
	return m.SessionForGuild(parsed)
}

// SessionForGuild returns the session for the specified guild
func (m *Manager) SessionForGuild(guildID int64) *discordgo.Session {
	// (guild_id >> 22) % num_shards == shard_id
	// That formula is taken from the sharding issue on the api docs repository on github
	m.RLock()
	defer m.RUnlock()
	shardID := (guildID >> 22) % int64(m.numShards)
	return m.Sessions[shardID]
}

// Session retrieves a session from the sessions map, rlocking it in the process
func (m *Manager) Session(shardID int) *discordgo.Session {
	m.RLock()
	defer m.RUnlock()
	return m.Sessions[shardID]
}

// LogConnectionEventStd is the standard connection event logger, it logs it to whatever log.output is set to.
func (m *Manager) LogConnectionEventStd(e *Event) {
	log.Printf("[Shard Manager] %s", e.String())
}

func (m *Manager) handleError(err error, shard int, msg string) bool {
	if err == nil {
		return false
	}

	m.handleEvent(EventError, shard, msg+": "+err.Error())
	return true
}

func (m *Manager) handleEvent(typ EventType, shard int, msg string) {
	if m.OnEvent == nil {
		return
	}

	evt := &Event{
		Type:      typ,
		Shard:     shard,
		NumShards: m.numShards,
		Msg:       msg,
		Time:      time.Now(),
	}

	go m.OnEvent(evt)

	if m.LogChannel != 0 {
		go m.logEventToDiscord(evt)
	}

	go func() {
		m.Lock()
		m.nextStatusUpdate = time.Now().Add(time.Second * 2)
		m.Unlock()
	}()
}

// StdSessionFunc is the standard session provider, it does nothing to the actual session
func (m *Manager) StdSessionFunc(token string) (*discordgo.Session, error) {
	s, err := discordgo.New(token)
	if err != nil {
		return nil, errors.WithMessage(err, "StdSessionFunc")
	}
	return s, nil
}

func (m *Manager) logEventToDiscord(evt *Event) {
	if evt.Type == EventError {
		return
	}

	prefix := ""
	if m.Name != "" {
		prefix = m.Name + ": "
	}

	str := evt.String()
	embed := &discordgo.MessageEmbed{
		Description: prefix + str,
		Timestamp:   evt.Time.Format(time.RFC3339),
		Color:       eventColors[evt.Type],
	}

	_, err := m.bareSession.ChannelMessageSendEmbed(m.LogChannel, embed)
	m.handleError(err, evt.Shard, "Failed sending event to discord")
}

func (m *Manager) statusRoutine() {
	if m.StatusMessageChannel == 0 {
		return
	}

	var mID int64

	// Find the initial message id and reuse that message if found
	msgs, err := m.bareSession.ChannelMessages(m.StatusMessageChannel, 50, 0, 0, 0)
	if err != nil {
		m.handleError(err, -1, "Failed requesting message history in channel")
	} else {
		for _, msg := range msgs {
			// Dunno our own bot id so best we can do is bot
			if !msg.Author.Bot || len(msg.Embeds) < 1 {
				continue
			}

			nameStr := ""
			if m.Name != "" {
				nameStr = " for " + m.Name
			}

			embed := msg.Embeds[0]
			if embed.Title == "Sharding status"+nameStr {
				// Found it sucessfully
				mID = msg.ID
				break
			}
		}
	}

	ticker := time.NewTicker(time.Second)
	for {
		select {
		case <-ticker.C:
			m.RLock()
			after := time.Now().After(m.nextStatusUpdate)
			m.RUnlock()
			if after {
				m.Lock()
				m.nextStatusUpdate = time.Now().Add(time.Minute)
				m.Unlock()

				nID, err := m.updateStatusMessage(mID)
				if !m.handleError(err, -1, "Failed updating status message") {
					mID = nID
				}
			}
		}
	}
}

func (m *Manager) updateStatusMessage(mID int64) (int64, error) {
	content := ""

	status := m.GetFullStatus()
	for _, shard := range status.Shards {
		gwStatus := ""
		switch shard.Status {
		case discordgo.GatewayStatusConnecting:
			gwStatus = "**Connecting...**"
		case discordgo.GatewayStatusDisconnected:
			gwStatus = "**Disconnected**"
		case discordgo.GatewayStatusIdentifying:
			gwStatus = "**Identifying**"
		case discordgo.GatewayStatusResuming:
			gwStatus = "**Resuming**"
		case discordgo.GatewayStatusReady:
			gwStatus = "👌"
		default:
			gwStatus = "?"
		}

		content += fmt.Sprintf("[%d/%d]: %s (%d,%d)\n", shard.Shard, m.numShards, gwStatus, shard.NumGuilds, status.NumGuilds)
	}

	nameStr := ""
	if m.Name != "" {
		nameStr = " for " + m.Name
	}
	embed := &discordgo.MessageEmbed{
		Title:       "Sharding status" + nameStr,
		Description: content,
		Color:       0x4286f4,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	if mID == 0 {
		msg, err := m.bareSession.ChannelMessageSendEmbed(m.StatusMessageChannel, embed)
		if err != nil {
			return 0, err
		}

		return msg.ID, err
	}

	_, err := m.bareSession.ChannelMessageEditEmbed(m.StatusMessageChannel, mID, embed)
	return mID, err
}

// GetFullStatus retrieves the full status at this instant
func (m *Manager) GetFullStatus() *Status {
	var shardGuilds []int
	if m.GuildCountsFunc != nil {
		shardGuilds = m.GuildCountsFunc()
	} else {
		shardGuilds = m.StdGuildCountsFunc()
	}

	m.RLock()
	result := make([]*ShardStatus, len(m.Sessions))
	for i, shard := range m.Sessions {
		result[i] = &ShardStatus{
			Shard: i,
		}

		if shard != nil {
			result[i].Started = true

			result[i].Status = shard.GatewayManager.Status()
		}
	}
	m.RUnlock()

	totalGuilds := 0
	for shard, guilds := range shardGuilds {
		totalGuilds += guilds
		result[shard].NumGuilds = guilds
	}

	return &Status{
		Shards:    result,
		NumGuilds: totalGuilds,
	}
}

// StdGuildsFunc uses the standard states to return the guilds
func (m *Manager) StdGuildCountsFunc() []int {

	m.RLock()
	nShards := m.numShards
	result := make([]int, nShards)

	for i, session := range m.Sessions {
		if session == nil {
			continue
		}
		session.State.RLock()
		result[i] = len(session.State.Guilds)
		session.State.RUnlock()
	}

	m.RUnlock()
	return result
}

type Status struct {
	Shards    []*ShardStatus `json:"shards"`
	NumGuilds int            `json:"num_guilds"`
}

type ShardStatus struct {
	Shard     int                     `json:"shard"`
	Status    discordgo.GatewayStatus `json:"status"`
	Started   bool                    `json:"started"`
	NumGuilds int                     `json:"num_guilds"`
}

// Event holds data for an event
type Event struct {
	Type EventType

	Shard     int
	NumShards int

	Msg string

	// When this event occured
	Time time.Time
}

func (c *Event) String() string {
	prefix := ""
	if c.Shard > -1 {
		prefix = fmt.Sprintf("[%d/%d] ", c.Shard, c.NumShards)
	}

	s := fmt.Sprintf("%s%s", prefix, strings.Title(c.Type.String()))
	if c.Msg != "" {
		s += ": " + c.Msg
	}

	return s
}

type EventType int

const (
	// Sent when the connection to the gateway was established
	EventConnected EventType = iota

	// Sent when the connection is lose
	EventDisconnected

	// Sent when the connection was sucessfully resumed
	EventResumed

	// Sent on ready
	EventReady

	// Sent when Open() is called
	EventOpen

	// Sent when Close() is called
	EventClose

	// Sent when an error occurs
	EventError
)

var (
	eventStrings = map[EventType]string{
		EventOpen:         "opened",
		EventClose:        "closed",
		EventConnected:    "connected",
		EventDisconnected: "disconnected",
		EventResumed:      "resumed",
		EventReady:        "ready",
		EventError:        "error",
	}

	eventColors = map[EventType]int{
		EventOpen:         0xec58fc,
		EventClose:        0xff7621,
		EventConnected:    0x54d646,
		EventDisconnected: 0xcc2424,
		EventResumed:      0x5985ff,
		EventReady:        0x00ffbf,
		EventError:        0x7a1bad,
	}
)

func (c EventType) String() string {
	return eventStrings[c]
}
