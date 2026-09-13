package tennis

import (
	"encoding/json"
	"strings"
	"testing"
)

func pts(values ...string) []*string {
	out := make([]*string, 0, len(values))
	for _, v := range values {
		if v == "" {
			out = append(out, nil)
			continue
		}
		s := v
		out = append(out, &s)
	}
	return out
}

func serverPtr(n int) *int { return &n }

func TestIsBreakPoint(t *testing.T) {
	tests := []struct {
		name  string
		score *score
		want  bool
	}{
		{"nil score", nil, false},
		{"nil server", &score{Points: pts("40", "AD")}, false},

		{"receiver holds advantage, p1 serving", &score{Points: pts("AD", "40"), Server: serverPtr(2)}, true},
		{"receiver holds advantage, p2 serving", &score{Points: pts("40", "AD"), Server: serverPtr(1)}, true},

		{"receiver at 40, server at 0", &score{Points: pts("0", "40"), Server: serverPtr(1)}, true},
		{"receiver at 40, server at 15", &score{Points: pts("15", "40"), Server: serverPtr(1)}, true},
		{"receiver at 40, server at 30", &score{Points: pts("30", "40"), Server: serverPtr(1)}, true},
		{"receiver at 40, server at 30, other side serving", &score{Points: pts("40", "30"), Server: serverPtr(2)}, true},

		{"deuce is not a break point", &score{Points: pts("40", "40"), Server: serverPtr(1)}, false},
		{"server holds advantage", &score{Points: pts("AD", "40"), Server: serverPtr(1)}, false},
		{"server at game point", &score{Points: pts("40", "30"), Server: serverPtr(1)}, false},
		{"start of game", &score{Points: pts("0", "0"), Server: serverPtr(1)}, false},

		// A tiebreak has no server's game to break, and its points are a
		// running integer count that must not be read as tennis scores.
		{"tiebreak, receiver ahead", &score{Points: pts("5", "6"), Server: serverPtr(1), IsTiebreak: true}, false},
		{"tiebreak that happens to read 40", &score{Points: pts("30", "40"), Server: serverPtr(1), IsTiebreak: true}, false},

		// Completed matches are served with null points.
		{"both points null", &score{Points: pts("", ""), Server: serverPtr(1)}, false},
		{"receiver point null", &score{Points: pts("30", ""), Server: serverPtr(1)}, false},
		{"server point null", &score{Points: pts("", "40"), Server: serverPtr(1)}, false},
		{"empty points array", &score{Points: pts(), Server: serverPtr(1)}, false},
		{"single point entry", &score{Points: pts("40"), Server: serverPtr(1)}, false},
		{"nil points array", &score{Server: serverPtr(1)}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBreakPoint(tt.score); got != tt.want {
				t.Errorf("isBreakPoint() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatSets(t *testing.T) {
	tests := []struct {
		name  string
		score *score
		want  string
	}{
		{"nil score", nil, ""},
		// games is player-major: [[6,3],[4,4]] is 6-4 in the first set and 3-4
		// in the second, NOT 6-3 and 4-4.
		{"two sets", &score{Games: [][]int{{6, 3}, {4, 4}}}, "6-4 3-4"},
		{"one set in progress", &score{Games: [][]int{{2}, {1}}}, "2-1"},
		{"five sets", &score{Games: [][]int{{6, 3, 6, 4}, {4, 6, 3, 6}}}, "6-4 3-6 6-3 4-6"},
		{"completed match with empty games", &score{Games: [][]int{{}, {}}}, ""},
		{"no games at all", &score{}, ""},
		{"only one side present", &score{Games: [][]int{{6, 3}}}, ""},
		// Never index past the shorter side.
		{"ragged games arrays", &score{Games: [][]int{{6, 3}, {4}}}, "6-4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatSets(tt.score); got != tt.want {
				t.Errorf("formatSets() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatPoints(t *testing.T) {
	tests := []struct {
		name  string
		score *score
		want  string
	}{
		{"nil score", nil, ""},
		{"game in progress", &score{Points: pts("30", "40")}, "30-40"},
		{"advantage", &score{Points: pts("AD", "40")}, "AD-40"},
		{"tiebreak is labelled", &score{Points: pts("5", "6"), IsTiebreak: true}, "TB 5-6"},
		{"null points", &score{Points: pts("", "")}, ""},
		{"one null point", &score{Points: pts("30", "")}, ""},
		{"empty points", &score{Points: pts()}, ""},
		{"nil points", &score{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatPoints(tt.score); got != tt.want {
				t.Errorf("formatPoints() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSurname(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"Carlos Alcaraz", "Alcaraz"},
		{"Jannik Sinner", "Sinner"},
		{"Felix Auger-Aliassime", "Auger-Aliassime"},
		{"Jan-Lennard Struff", "Struff"},
		{"Alcaraz", "Alcaraz"},
		{"  Iga  Swiatek  ", "Swiatek"},
		{"Marcel Granollers / Horacio Zeballos", "Granollers/Zeballos"},
		{"", "?"},
		{"   ", "?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := surname(tt.name); got != tt.want {
				t.Errorf("surname(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestFormatMatch(t *testing.T) {
	newMatch := func(p1, p2 string, s *score) match {
		var m match
		m.Players.P1.Name = p1
		m.Players.P2.Name = p2
		m.Score = s
		return m
	}

	tests := []struct {
		name  string
		match match
		want  string
	}{
		{
			"break point against the first player",
			newMatch("Jannik Sinner", "Carlos Alcaraz", &score{
				Games:  [][]int{{6, 3}, {4, 4}},
				Points: pts("30", "40"),
				Server: serverPtr(1),
			}),
			"Sinner ● v Alcaraz · 6-4 3-4 · 30-40 · **BP**",
		},
		{
			"second player serving, no break point",
			newMatch("Jannik Sinner", "Carlos Alcaraz", &score{
				Games:  [][]int{{6}, {4}},
				Points: pts("15", "30"),
				Server: serverPtr(2),
			}),
			"Sinner v Alcaraz ● · 6-4 · 15-30",
		},
		{
			"tiebreak never marks a break point",
			newMatch("Jannik Sinner", "Carlos Alcaraz", &score{
				Games:      [][]int{{6}, {6}},
				Points:     pts("6", "7"),
				Server:     serverPtr(1),
				IsTiebreak: true,
			}),
			"Sinner ● v Alcaraz · 6-6 · TB 6-7",
		},
		{
			"completed match with null points and empty games",
			newMatch("Jannik Sinner", "Carlos Alcaraz", &score{
				Games:  [][]int{{}, {}},
				Points: pts("", ""),
			}),
			"Sinner v Alcaraz",
		},
		{
			"match with no score yet",
			newMatch("Jannik Sinner", "Carlos Alcaraz", nil),
			"Sinner v Alcaraz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatMatch(tt.match); got != tt.want {
				t.Errorf("formatMatch() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilterByPlayer(t *testing.T) {
	newMatch := func(p1, p2 string) match {
		var m match
		m.Players.P1.Name = p1
		m.Players.P2.Name = p2
		return m
	}

	board := []match{
		newMatch("Jannik Sinner", "Carlos Alcaraz"),
		newMatch("Iga Swiatek", "Aryna Sabalenka"),
		newMatch("Novak Djokovic", "Daniil Medvedev"),
	}

	tests := []struct {
		name  string
		query string
		want  int
	}{
		{"empty query returns everything", "", 3},
		{"surname", "Alcaraz", 1},
		{"case insensitive", "swiatek", 1},
		{"matches the second player", "Medvedev", 1},
		{"partial name", "Saba", 1},
		{"first name", "Novak", 1},
		{"no such player", "Federer", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filterByPlayer(board, tt.query); len(got) != tt.want {
				t.Errorf("filterByPlayer(%q) returned %d matches, want %d", tt.query, len(got), tt.want)
			}
		})
	}
}

func TestFormatBoardGroupsAndTruncates(t *testing.T) {
	newMatch := func(tournament, p1, p2 string) match {
		var m match
		m.Tournament = tournament
		m.Players.P1.Name = p1
		m.Players.P2.Name = p2
		return m
	}

	board := []match{
		newMatch("US Open", "Jannik Sinner", "Carlos Alcaraz"),
		newMatch("Challenger Como", "A Player", "B Player"),
		newMatch("US Open", "Iga Swiatek", "Aryna Sabalenka"),
	}

	got := formatBoard(board, 10)

	// Tournaments become headers, sorted, and each one carries its matches.
	if strings.Index(got, "**Challenger Como**") > strings.Index(got, "**US Open**") {
		t.Errorf("tournaments are not sorted:\n%s", got)
	}
	if strings.Count(got, "**US Open**") != 1 {
		t.Errorf("US Open should appear once as a header:\n%s", got)
	}
	for _, want := range []string{"Sinner v Alcaraz", "Swiatek v Sabalenka", "Player v Player"} {
		if !strings.Contains(got, want) {
			t.Errorf("board is missing %q:\n%s", want, got)
		}
	}

	// Over the limit, the rest are counted rather than dropped silently.
	truncated := formatBoard(board, 2)
	if !strings.Contains(truncated, "and 1 more") {
		t.Errorf("expected a truncation notice:\n%s", truncated)
	}
}

// The API serves null for points entries, for server, and for the whole score
// object. Decoding has to survive all three, which is why those fields are
// pointers rather than values.
func TestDecodeListResponse(t *testing.T) {
	const body = `{
	  "data": [
	    {
	      "tournament": "US Open",
	      "players": {"p1": {"name": "Jannik Sinner"}, "p2": {"name": "Carlos Alcaraz"}},
	      "score": {"sets": [1, 0], "games": [[6, 3], [4, 4]], "points": ["30", "40"], "server": 1, "is_tiebreak": false}
	    },
	    {
	      "tournament": "Challenger Como",
	      "players": {"p1": {"name": "A Player"}, "p2": {"name": "B Player"}},
	      "score": {"sets": [2, 1], "games": [[], []], "points": [null, null], "server": null, "is_tiebreak": false}
	    },
	    {
	      "tournament": "ITF Cairo",
	      "players": {"p1": {"name": "C Player"}, "p2": {"name": "D Player"}},
	      "score": null
	    }
	  ],
	  "meta": {"limit": 200, "offset": 0, "count": 3, "total": 3, "has_more": false}
	}`

	var parsed matchListResponse
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("decoding a realistic payload failed: %v", err)
	}

	if len(parsed.Data) != 3 {
		t.Fatalf("decoded %d matches, want 3", len(parsed.Data))
	}

	// A match in play.
	live := parsed.Data[0]
	if live.Score == nil {
		t.Fatal("score of the live match decoded as nil")
	}
	if live.Score.Server == nil || *live.Score.Server != 1 {
		t.Errorf("server = %v, want 1", live.Score.Server)
	}
	// games is player-major, so this is 6-4 3-4 and not 6-3 4-4.
	if got := formatSets(live.Score); got != "6-4 3-4" {
		t.Errorf("formatSets() = %q, want %q", got, "6-4 3-4")
	}
	if !isBreakPoint(live.Score) {
		t.Error("30-40 on player one's serve should be a break point")
	}

	// A match whose points and server came back null.
	nulls := parsed.Data[1]
	if nulls.Score == nil {
		t.Fatal("score with null members decoded as nil")
	}
	if len(nulls.Score.Points) != 2 {
		t.Fatalf("decoded %d points, want 2", len(nulls.Score.Points))
	}
	for i, p := range nulls.Score.Points {
		if p != nil {
			t.Errorf("points[%d] = %q, want nil", i, *p)
		}
	}
	if nulls.Score.Server != nil {
		t.Errorf("server = %v, want nil", *nulls.Score.Server)
	}
	if got := formatMatch(nulls); got != "Player v Player" {
		t.Errorf("formatMatch() = %q, want %q", got, "Player v Player")
	}

	// A match with no score object at all.
	if parsed.Data[2].Score != nil {
		t.Error("a null score should decode to a nil pointer")
	}
	if got := formatMatch(parsed.Data[2]); got != "Player v Player" {
		t.Errorf("formatMatch() = %q, want %q", got, "Player v Player")
	}
}

func TestFormatBoardEmptyTournamentName(t *testing.T) {
	var m match
	m.Players.P1.Name = "Jannik Sinner"
	m.Players.P2.Name = "Carlos Alcaraz"

	got := formatBoard([]match{m}, 10)
	if !strings.Contains(got, "**Other**") {
		t.Errorf("an unnamed tournament should be grouped under Other:\n%s", got)
	}
}
