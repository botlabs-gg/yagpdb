package dice

import (
	"fmt"
	"math/rand"
	"regexp"
	"sort"
	"strconv"
)

type StdRoller struct{}

var stdPattern = regexp.MustCompile(`([0-9]+)d([0-9]+)((k|d|kh|dl|kl|dh)([0-9]+))?([+-][0-9]+)?($|\s)`)

func (StdRoller) Pattern() *regexp.Regexp { return stdPattern }

type StdResult struct {
	basicRollResult
	Rolls   []int
	Dropped []int
	Total   int
}

func (r StdResult) String() string {
	return fmt.Sprintf("%d %v (%v)", r.Total, r.Rolls, r.Dropped)
}

func (r StdResult) Int() int {
	return r.Total
}

func (StdRoller) Roll(matches []string) (RollResult, error) {
	dice, err := strconv.ParseInt(matches[1], 10, 0)
	if err != nil {
		return nil, err
	}
	if dice > MaxLoop {
		return nil, ErrTooManyLoops
	}

	sides, err := strconv.ParseInt(matches[2], 10, 0)
	if err != nil {
		return nil, err
	}

	keep := ""
	num := 0
	if matches[4] != "" {
		number, err := strconv.ParseInt(matches[5], 10, 0)
		if err != nil {
			return nil, err
		}
		num = int(number)
		keep = matches[4]
	}

	result := StdResult{
		basicRollResult: basicRollResult{matches[0]},
		Rolls:           make([]int, dice),
		Dropped:         nil,
		Total:           0,
	}

	if matches[6] != "" {
		bonus, err := strconv.ParseInt(matches[6], 10, 0)
		if err != nil {
			return nil, err
		}
		result.Total += int(bonus)
	}

	for i := 0; i < len(result.Rolls); i++ {
		roll := rand.Intn(int(sides)) + 1
		result.Rolls[i] = roll
	}

	sort.Ints(result.Rolls)
	size := len(result.Rolls)

	switch keep {
	case "k":
		fallthrough
	case "kh":
		result.Dropped = result.Rolls[:size-num]
		result.Rolls = result.Rolls[size-num:]
	case "d":
		fallthrough
	case "dl":
		result.Dropped = result.Rolls[:num]
		result.Rolls = result.Rolls[num:]
	case "kl":
		result.Dropped = result.Rolls[num:]
		result.Rolls = result.Rolls[:num]
	case "dh":
		result.Dropped = result.Rolls[size-num:]
		result.Rolls = result.Rolls[:size-num]
	}

	for i := 0; i < len(result.Rolls); i++ {
		result.Total += result.Rolls[i]
	}

	return result, nil
}

func init() {
	addRollHandler(StdRoller{})
}
