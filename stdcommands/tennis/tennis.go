package tennis

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common/config"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

// Live scores come from the Live Tennis API (https://livetennisapi.com), a
// third-party commercial service. It needs an API key, so this command is
// inert until the bot's host configures one:
//
//	YAGPDB_LIVETENNISAPI_API_KEY=<key>
//
// A NOTE ON PLANS, for whoever runs this bot. The vendor's free tier allows
// 100 requests per day, which is not enough to serve a public bot: the live
// board below is refreshed at most once every liveBoardTTL (60s), so a day in
// which the command is used at least once a minute costs 24*60 = 1440
// requests. Staying inside 100/day would mean a ~14.4 minute cache, which is
// not "live" in any useful sense. Running this on a large public instance
// therefore needs a paid plan; at the vendor's PRO tier (10,000/day) the 1440
// worst case is about 14% of the daily budget, with plenty of headroom.
//
// The cost is flat in the number of guilds and users. One cached board serves
// every guild, fetchLiveBoard collapses concurrent misses into a single
// upstream request, and the player search filters that same cached board
// instead of issuing a query of its own, so it costs no requests at all. What
// bounds the quota is the cache; the per-user and per-guild Cooldown fields
// below are there to stop command spam on the Discord side.
var confAPIKey = config.RegisterOption("yagpdb.livetennisapi.api_key", "Live Tennis API key for the tennis command, from https://livetennisapi.com (a paid plan is required for a public bot, see stdcommands/tennis/tennis.go)", "")

const (
	liveBoardURL = "https://api.livetennisapi.com/api/public/v1/matches?status=live&limit=200"

	// liveBoardTTL is how long a fetched board is served to every guild before
	// another upstream request is made. See the plan note above before changing
	// it: the worst-case daily request count is 24h / liveBoardTTL.
	liveBoardTTL = time.Minute

	// maxListedMatches caps the embed so a busy day cannot blow Discord's
	// 4096-character description limit.
	maxListedMatches = 20

	embedColor = 0xc2ff4f
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

// player is one side of a match. For a doubles team, Name holds both surnames
// separated by a slash.
type player struct {
	Name string `json:"name"`
}

// score mirrors the vendor's Score object. Every field the API documents as
// nullable is modelled as a pointer, because they really are null in practice:
// a match that has just completed is served with null points and empty games
// arrays, so decoding points into a plain []string fails on live data.
type score struct {
	Games      [][]int   `json:"games"`  // player-major: [p1 per-set games, p2 per-set games]
	Points     []*string `json:"points"` // "0","15","30","40","AD", or a plain integer count in a tiebreak; entries can be null
	Server     *int      `json:"server"` // 1, 2 or null
	IsTiebreak bool      `json:"is_tiebreak"`
}

type match struct {
	Tournament string `json:"tournament"`
	Players    struct {
		P1 player `json:"p1"`
		P2 player `json:"p2"`
	} `json:"players"`
	Score *score `json:"score"` // null before the first point of a match is scored
}

type matchListResponse struct {
	Data []match `json:"data"`
}

var (
	boardMu      sync.Mutex
	cachedBoard  []match
	cachedExpiry time.Time
)

var Command = &commands.YAGCommand{
	CmdCategory:         commands.CategoryFun,
	Name:                "Tennis",
	Aliases:             []string{"atp", "wta"},
	Description:         "🎾 Shows tennis matches that are in play right now, optionally only those featuring a given player.",
	LongDescription:     "Live scores are provided by the Live Tennis API (https://livetennisapi.com) and are only available if the host of this bot has configured an API key.\n\nSet scores are shown from the first player's point of view, `●` marks the player serving, and `BP` marks a break point.",
	RunInDM:             true,
	Cooldown:            10,
	GuildScopeCooldown:  5,
	DefaultEnabled:      true,
	SlashCommandEnabled: true,
	Arguments: []*dcmd.ArgDef{
		{Name: "Player", Type: dcmd.String, Help: "Only show matches featuring this player"},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		matches, err := fetchLiveBoard()
		if err != nil {
			return nil, err
		}

		title := "🎾 Live tennis"
		if query := strings.TrimSpace(data.Args[0].Str()); query != "" {
			matches = filterByPlayer(matches, query)
			title = fmt.Sprintf("🎾 Live tennis — %q", query)

			if len(matches) == 0 {
				return fmt.Sprintf("No live match featuring %q right now.", query), nil
			}
		}

		if len(matches) == 0 {
			return "No tennis matches are in play right now.", nil
		}

		return &discordgo.MessageEmbed{
			Title:       title,
			Description: formatBoard(matches, maxListedMatches),
			Color:       embedColor,
			Timestamp:   time.Now().UTC().Format(time.RFC3339),
			Footer: &discordgo.MessageEmbedFooter{
				Text: "livetennisapi.com",
			},
		}, nil
	},
}

// fetchLiveBoard returns the live board, re-fetching it only once per
// liveBoardTTL. The lock is deliberately held across the request so that a
// burst of misses from different guilds results in one upstream call rather
// than one per caller.
func fetchLiveBoard() ([]match, error) {
	apiKey := confAPIKey.GetString()
	if apiKey == "" {
		return nil, commands.NewPublicError("The tennis command is not available: the host of this bot has not configured a Live Tennis API key.")
	}

	boardMu.Lock()
	defer boardMu.Unlock()

	if time.Now().Before(cachedExpiry) {
		return cachedBoard, nil
	}

	matches, err := requestLiveBoard(apiKey)
	if err != nil {
		return nil, err
	}

	cachedBoard = matches
	cachedExpiry = time.Now().Add(liveBoardTTL)
	return cachedBoard, nil
}

func requestLiveBoard(apiKey string) ([]match, error) {
	req, err := http.NewRequest("GET", liveBoardURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("User-Agent", "YAGPDB.xyz (https://github.com/botlabs-gg/yagpdb)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, commands.NewPublicError("The tennis command is not available: this bot's Live Tennis API key was rejected.")
	case http.StatusTooManyRequests:
		return nil, commands.NewPublicError("This bot has run out of Live Tennis API requests for now, try again later.")
	default:
		return nil, commands.NewPublicErrorF("Failed fetching live scores (HTTP %d), try again later.", resp.StatusCode)
	}

	var parsed matchListResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}

	return parsed.Data, nil
}

// filterByPlayer keeps matches where either side's name contains query,
// case-insensitively. It reads the cached board, so it costs no API requests.
func filterByPlayer(matches []match, query string) []match {
	if query == "" {
		return matches
	}

	needle := strings.ToLower(query)
	out := make([]match, 0, len(matches))
	for _, m := range matches {
		if strings.Contains(strings.ToLower(m.Players.P1.Name), needle) ||
			strings.Contains(strings.ToLower(m.Players.P2.Name), needle) {
			out = append(out, m)
		}
	}

	return out
}

// formatBoard renders the board grouped by tournament, one line per match.
func formatBoard(matches []match, limit int) string {
	shown := matches
	omitted := 0
	if len(shown) > limit {
		omitted = len(shown) - limit
		shown = shown[:limit]
	}

	byTournament := make(map[string][]match)
	for _, m := range shown {
		byTournament[m.Tournament] = append(byTournament[m.Tournament], m)
	}

	tournaments := make([]string, 0, len(byTournament))
	for name := range byTournament {
		tournaments = append(tournaments, name)
	}
	sort.Strings(tournaments)

	var b strings.Builder
	for i, name := range tournaments {
		if i > 0 {
			b.WriteString("\n")
		}
		if name == "" {
			name = "Other"
		}
		fmt.Fprintf(&b, "**%s**\n", name)
		for _, m := range byTournament[name] {
			b.WriteString(formatMatch(m))
			b.WriteString("\n")
		}
	}

	if omitted > 0 {
		fmt.Fprintf(&b, "\n*and %d more — search for a player to narrow it down*", omitted)
	}

	return b.String()
}

// formatMatch renders one match as a single line, for example
//
//	Sinner ● v Alcaraz · 6-4 3-4 · 40-AD · BP
func formatMatch(m match) string {
	p1 := surname(m.Players.P1.Name)
	p2 := surname(m.Players.P2.Name)

	if m.Score != nil {
		if server := m.Score.Server; server != nil {
			switch *server {
			case 1:
				p1 += " ●"
			case 2:
				p2 += " ●"
			}
		}
	}

	line := p1 + " v " + p2

	if sets := formatSets(m.Score); sets != "" {
		line += " · " + sets
	}

	if points := formatPoints(m.Score); points != "" {
		line += " · " + points
	}

	if isBreakPoint(m.Score) {
		line += " · **BP**"
	}

	return line
}

// surname takes the last whitespace-separated word of each name, so
// "Carlos Alcaraz" reads "Alcaraz" and the doubles team
// "Marcel Granollers / Horacio Zeballos" reads "Granollers/Zeballos".
func surname(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "?"
	}

	parts := strings.Split(name, "/")
	for i, part := range parts {
		fields := strings.Fields(part)
		if len(fields) == 0 {
			parts[i] = "?"
			continue
		}
		parts[i] = fields[len(fields)-1]
	}

	return strings.Join(parts, "/")
}

// formatSets renders the per-set games from the first player's point of view.
// The vendor's games array is player-major -- games[0] is player one's games
// in each set and games[1] is player two's -- so [[6,3],[4,4]] reads "6-4 3-4".
func formatSets(s *score) string {
	if s == nil || len(s.Games) < 2 {
		return ""
	}

	p1, p2 := s.Games[0], s.Games[1]
	sets := min(len(p1), len(p2))

	out := make([]string, 0, sets)
	for i := 0; i < sets; i++ {
		out = append(out, fmt.Sprintf("%d-%d", p1[i], p2[i]))
	}

	return strings.Join(out, " ")
}

// formatPoints renders the game in progress, or "" when it is not known. In a
// tiebreak the two values are a running integer count rather than tennis
// scores, so they are labelled to tell the notations apart.
func formatPoints(s *score) string {
	if s == nil || len(s.Points) < 2 {
		return ""
	}

	p1, p2 := s.Points[0], s.Points[1]
	if p1 == nil || p2 == nil {
		return ""
	}

	if s.IsTiebreak {
		return fmt.Sprintf("TB %s-%s", *p1, *p2)
	}

	return fmt.Sprintf("%s-%s", *p1, *p2)
}

// isBreakPoint reports whether the receiver is one point from winning the
// game: they hold advantage, or they are at 40 while the server is not. It is
// never true in a tiebreak, where there is no server's game to break, and it
// is false whenever the score or the server is unknown.
func isBreakPoint(s *score) bool {
	if s == nil || s.IsTiebreak || s.Server == nil || len(s.Points) < 2 {
		return false
	}

	var receiverIdx int
	switch *s.Server {
	case 1:
		receiverIdx = 1
	case 2:
		receiverIdx = 0
	default:
		return false
	}
	serverIdx := 1 - receiverIdx

	receiver, server := s.Points[receiverIdx], s.Points[serverIdx]
	if receiver == nil || server == nil {
		return false
	}

	switch *receiver {
	case "AD":
		return true
	case "40":
		switch *server {
		case "0", "15", "30":
			return true
		}
	}

	return false
}
