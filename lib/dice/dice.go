// Package dice provides a library for rolling tabletop role-playing style dice
// via a text description similar to that used in such games.
//
// Examples
//
//	dice.Roll("3d6") // Roll and sum 3 six-sided dice
//	dice.Roll("1d20+2") // Roll a twenty-sided die and add 2
//
// # Supported formats
//
// Standard: `xdy[[k|d][h|l]z][+/-c]` - rolls and sums x y-sided dice,
// keeping or dropping the lowest or highest z dice and optionally adding
// or subtracting c. Example: 4d6kh3+4
//
// Fudge: `xdf[+/-c]` - rolls and sums x fudge dice (Dice that return
// numbers between -1 and 1), and optionally adding or subtracting c.
// Example: 4df+4
//
// Versus: `xdy[e|r]vt` - rolls x y-sided dice, counting the number that
// roll t or greater.
//
// EotE: `xc [xc ...]` - rolls x dice of color c (b, blk, g, p, r, w, y)
// and returns the aggregate result.
package dice

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	MaxLoop int64 = 1000
)

var (
	ErrTooManyLoops = errors.New("Too many loops, you either specified too many dices or sides")
)

// RollResult is an interface to describe a dice roll and its outcome.
//
// RollResult contains [fmt.Stringer], and calling `String()` on a RollResult
// will get a textual description of the roll outcome.
//
// Description returns a textual description of the roll. This is often the
// same as the input to [dice.Roll].
//
// Int will get the result of the roll as an integer.
type RollResult interface {
	fmt.Stringer
	Description() string
	Int() int
}

type roller interface {
	Pattern() *regexp.Regexp
	Roll([]string) (RollResult, error)
}

type basicRollResult struct {
	desc string
}

func (r basicRollResult) Description() string { return strings.Trim(r.desc, " \t\r\n") }

var rollHandlers []roller

func addRollHandler(handler roller) {
	rollHandlers = append(rollHandlers, handler)
}

/*
	open - regexp.MustCompile(`([0-9]+)d([0-9]+)(e)?o$`)
	sil - regexp.MustCompile(`([0-9]+)d([0-9]+)(e)?s$`)
*/

// Roll parses the given description and calls the appropriate roller implementation,
// if found. Returns the result plus any extra trailing text if successful.
func Roll(desc string) (RollResult, string, error) {
	for _, rollHandler := range rollHandlers {
		rollHandler.Pattern().Longest()

		if r := rollHandler.Pattern().FindStringSubmatch(desc); r != nil {
			result, err := rollHandler.Roll(r)
			if err != nil {
				return nil, "", err
			}

			indexes := rollHandler.Pattern().FindStringSubmatchIndex(desc)
			reason := strings.Trim(desc[indexes[0]+len(r[0]):], " \t\r\n")
			return result, reason, nil
		}
	}

	return nil, "", errors.New("Bad roll format: " + desc)
}
