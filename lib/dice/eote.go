package dice

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
)

type EoteDie struct {
	Type string
	S    int // success
	A    int // advantage
	T    int // triumph
	D    int // despair
	F    int // force
}

func (d *EoteDie) Add(o EoteDie) {
	d.S += o.S
	d.A += o.A
	d.T += o.T
	d.D += o.D
	d.F += o.F
}

func (d EoteDie) String() string {
	results := make([]string, 0, 3)

	if d.S > 0 {
		results = append(results, strings.Repeat("s", d.S))
	} else if d.S < 0 {
		results = append(results, strings.Repeat("f", -d.S))
	}

	if d.A > 0 {
		results = append(results, strings.Repeat("a", d.A))
	} else if d.A < 0 {
		results = append(results, strings.Repeat("d", -d.A))
	}

	if d.F > 0 {
		results = append(results, strings.Repeat("L", d.F))
	} else if d.F < 0 {
		results = append(results, strings.Repeat("D", -d.F))
	}

	if d.T > 0 {
		results = append(results, strings.Repeat("T", d.T))
	}

	if d.D > 0 {
		results = append(results, strings.Repeat("D", d.D))
	}

	return fmt.Sprintf("%s[%s]", d.Type, strings.Join(results, ""))
}

type EoteResult struct {
	basicRollResult
	EoteDie
	Rolls []EoteDie
}

func (r EoteResult) String() string {
	parts := make([]string, 0, 6)

	if r.S > 0 {
		parts = append(parts, fmt.Sprintf("%d success", r.S))
	} else if r.S < 0 {
		parts = append(parts, fmt.Sprintf("%d failure", -r.S))
	}

	if r.A > 0 {
		parts = append(parts, fmt.Sprintf("%d advantage", r.A))
	} else if r.A < 0 {
		parts = append(parts, fmt.Sprintf("%d disadvantage", -r.A))
	}

	if r.T > 0 {
		parts = append(parts, fmt.Sprintf("(%d triumph)", r.T))
	}

	if r.D > 0 {
		parts = append(parts, fmt.Sprintf("(%d despair)", r.D))
	}

	if r.F > 0 {
		parts = append(parts, fmt.Sprintf("%d light", r.F))
	} else if r.F < 0 {
		parts = append(parts, fmt.Sprintf("%d dark", -r.F))
	}

	if len(parts) < 1 {
		parts = append(parts, "no result")
	}

	rolls := make([]string, len(r.Rolls))
	for i := range r.Rolls {
		rolls[i] = r.Rolls[i].String()
	}
	parts = append(parts, fmt.Sprintf("\n%s", strings.Join(rolls, " ")))

	return strings.Join(parts, " ")
}

func (r EoteResult) Int() int {
	return r.S
}

var eoteDice = map[string][]EoteDie{
	"b":   {{}, {}, {A: 1}, {A: 2}, {S: 1}, {S: 1, A: 1}},
	"blk": {{}, {}, {A: -1}, {A: -1}, {S: -1}, {S: -1}},
	"g":   {{}, {A: 1}, {A: 1}, {A: 2}, {S: 1}, {S: 1}, {S: 1, A: 1}, {S: 2}},
	"p":   {{}, {A: -1}, {A: -1}, {A: -1}, {A: -2}, {S: -1}, {S: -1, A: -1}, {S: -2}},
	"y":   {{}, {A: 1}, {A: 2}, {A: 2}, {S: 1}, {S: 1}, {S: 1, A: 1}, {S: 1, A: 1}, {S: 1, A: 1}, {S: 2}, {S: 2}, {S: 1, T: 1}},
	"r":   {{}, {A: -1}, {A: -1}, {A: -2}, {A: -2}, {S: -1}, {S: -1}, {S: -1, A: -1}, {S: -1, A: -1}, {S: -2}, {S: -2}, {S: -1, D: 1}},
	"w":   {{F: -1}, {F: -1}, {F: -1}, {F: -1}, {F: -1}, {F: -1}, {F: -2}, {F: 1}, {F: 1}, {F: 2}, {F: 2}, {F: 2}},
}

type EoteRoller struct{}

var eotePattern = regexp.MustCompile(`([0-9]+(?:r|b|blk|p|g|y|w)\s*)+($|\s)`)
var diePattern = regexp.MustCompile(`([0-9]+)(r|b|blk|p|g|y|w)`)

func (EoteRoller) Pattern() *regexp.Regexp { return eotePattern }

func (EoteRoller) Roll(matches []string) (RollResult, error) {
	diePattern.Longest()

	res := EoteResult{basicRollResult: basicRollResult{matches[0]}}

	for _, die := range strings.Split(matches[0], " ") {
		parts := diePattern.FindStringSubmatch(strings.Trim(die, " \t\r\n"))
		if parts == nil {
			continue
		}

		num, err := strconv.ParseInt(parts[1], 10, 0)
		if err != nil {
			continue
		}
		if num > MaxLoop {
			return res, ErrTooManyLoops
		}

		choices, ok := eoteDice[parts[2]]
		if !ok {
			continue
		}

		for i := int64(0); i < num; i++ {
			die := choices[rand.Intn(len(choices))]
			res.Add(die)
			res.Rolls = append(res.Rolls, die)
		}
	}

	return res, nil
}

func init() {
	for name := range eoteDice {
		for i := range eoteDice[name] {
			eoteDice[name][i].Type = name
		}
	}

	addRollHandler(EoteRoller{})
}
