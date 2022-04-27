package dice

import (
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
)

type VsRoller struct{}

var vsPattern = regexp.MustCompile(`([0-9]+)d([0-9]+)(e|r)?v([0-9]+)($|\s)`)

func (VsRoller) Pattern() *regexp.Regexp { return vsPattern }

type VsResult struct {
	basicRollResult
	Rolls     []int
	Successes int
}

func (r VsResult) Description() string { return r.desc }

func (r VsResult) String() string {
	return fmt.Sprintf("%d %v", r.Successes, r.Rolls)
}

func (r VsResult) Int() int {
	return r.Successes
}

func (VsRoller) Roll(matches []string) (RollResult, error) {
	dice, err := strconv.ParseInt(matches[1], 10, 0)
	if err != nil {
		return nil, err
	}
	if dice < 1 {
		return nil, errors.New("Count must be 1 or more")
	}
	if dice > MaxLoop {
		return nil, ErrTooManyLoops
	}

	sides, err := strconv.ParseInt(matches[2], 10, 0)
	if err != nil {
		return nil, err
	}
	if sides < 2 {
		return nil, errors.New("Sides must be 2 or more")
	}
	if sides > MaxLoop {
		return nil, ErrTooManyLoops
	}

	explode := matches[3] == "e"
	reroll := matches[3] == "r"

	target, err := strconv.ParseInt(matches[4], 10, 0)
	if err != nil {
		return nil, err
	}

	result := VsResult{
		basicRollResult: basicRollResult{matches[0]},
		Rolls:           make([]int, 0, dice),
		Successes:       0,
	}

	for i := int64(0); i < dice; i++ {
		roll := rand.Intn(int(sides)) + 1

		if roll == int(sides) && explode {
			total := roll
			for roll == int(sides) {
				roll = rand.Intn(int(sides)) + 1
				total += roll
			}
			roll = total
		}

		if roll == int(sides) && reroll {
			i--
		}

		if roll >= int(target) {
			result.Successes += 1
		}

		result.Rolls = append(result.Rolls, roll)
	}

	return result, nil
}

func init() {
	addRollHandler(VsRoller{})
}
