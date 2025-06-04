package dice_test

import (
	. "github.com/botlabs-gg/yagpdb/v2/lib/dice"
	. "gopkg.in/check.v1"
)

/* =============================================================================
 * Eote dice test suite
 */

type eoteSuite struct {
}

/*
func (s *DiceSuite) SetUpSuite(c *C) {
}

func (s *DiceSuite) SetUpTest(c *C) {
}
*/

var _ = Suite(&eoteSuite{})

/* =============================================================================
 * Eote dice test cases
 */

func (s *eoteSuite) TestPositive(c *C) {
	var roller EoteRoller

	r, err := roller.Roll([]string{"100b 100g 100y", ""})
	res := r.(EoteResult)

	c.Assert(err, IsNil)
	c.Assert(res.EoteDie.String(), Matches, "\\[s+a+T+\\]")
	c.Assert(res.String(), Matches, "(?s).*\\d+ success \\d+ advantage \\(\\d+ triumph\\)(?s).*")
	c.Check(res.D, Equals, 0)
	c.Check(res.F, Equals, 0)

	if res.S <= 0 {
		c.Errorf("Total successes on positive-only roll: %d", res.S)
	}

	if res.A <= 0 {
		c.Errorf("Total advantage on positive-only roll: %d", res.A)
	}
}

func (s *eoteSuite) TestNegative(c *C) {
	var roller EoteRoller

	r, err := roller.Roll([]string{"100blk 100p 100r", ""})
	res := r.(EoteResult)

	c.Assert(err, IsNil)
	c.Assert(res.EoteDie.String(), Matches, "\\[f+d+D+\\]")
	c.Assert(res.String(), Matches, "(?s).*\\d+ failure \\d+ disadvantage \\(\\d+ despair\\)(?s).*")
	c.Check(res.T, Equals, 0)
	c.Check(res.F, Equals, 0)

	if res.S >= 0 {
		c.Errorf("Total successes on negative-only roll: %d", res.S)
	}

	if res.A >= 0 {
		c.Errorf("Total advantage on negative-only roll: %d", res.A)
	}
}

func (s *eoteSuite) TestForce(c *C) {
	var roller EoteRoller

	r, err := roller.Roll([]string{"100w", ""})
	res := r.(EoteResult)

	c.Assert(err, IsNil)
	c.Check(res.T, Equals, 0)
	c.Check(res.D, Equals, 0)
	c.Check(res.S, Equals, 0)
	c.Check(res.A, Equals, 0)
}

func (s *eoteSuite) TestBadRoll(c *C) {
	var roller EoteRoller
	c.Check(roller.Pattern().FindStringSubmatch("1l 1q"), IsNil)
}
