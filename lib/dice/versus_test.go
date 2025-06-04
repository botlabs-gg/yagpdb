package dice_test

import (
	. "github.com/botlabs-gg/yagpdb/v2/lib/dice"
	. "gopkg.in/check.v1"
)

/* =============================================================================
 * vs dice test suite
 */

type vsSuite struct {
}

/*
func (s *DiceSuite) SetUpSuite(c *C) {
}

func (s *DiceSuite) SetUpTest(c *C) {
}
*/

var _ = Suite(&vsSuite{})

/* =============================================================================
 * vs dice test cases
 */

func (s *vsSuite) TestBounds(c *C) {
	var roller VsRoller

	r, err := roller.Roll([]string{"100d2v2", "100", "2", "", "2"})
	res := r.(VsResult)

	c.Assert(err, IsNil)

	for _, v := range res.Rolls {
		if v <= 0 || v > 2 {
			c.Errorf("Rolled out of bounds on a d2: %d", v)
		}
	}
}

func (s *vsSuite) TestExplode(c *C) {
	var roller VsRoller

	r, err := roller.Roll([]string{"3d6ev1", "5", "4", "e", "1"})
	res := r.(VsResult)

	c.Assert(err, IsNil)
	c.Assert(res.String(), Matches, "5 \\[\\d+ \\d+ \\d+ \\d+ \\d+\\]")
}

func (s *vsSuite) TestInvalidNumber(c *C) {
	var roller VsRoller

	r, err := roller.Roll([]string{"1d1v1", "1", "1", "", "1"})

	c.Assert(err, NotNil)
	c.Assert(err.Error(), Matches, "Sides must be 2 or more")
	c.Assert(r, IsNil)

	r, err = roller.Roll([]string{"0d2v1", "0", "2", "", "1"})

	c.Assert(err, NotNil)
	c.Assert(err.Error(), Matches, "Count must be 1 or more")
	c.Assert(r, IsNil)
}

func (s *vsSuite) TestNumTooBig(c *C) {
	var roller VsRoller

	//const bigNum = int(^uint(0)>>1) + 1
	const bigNum = "9223372036854775808"

	r, err := roller.Roll([]string{"1d2v1", bigNum, "2", "", "1"})

	c.Assert(err, NotNil)
	c.Assert(err.Error(), Matches, "*value out of range")
	c.Assert(r, IsNil)

	r, err = roller.Roll([]string{"1d2v1", "1", bigNum, "", "1"})

	c.Assert(err, NotNil)
	c.Assert(err.Error(), Matches, "*value out of range")
	c.Assert(r, IsNil)

	r, err = roller.Roll([]string{"1d2v1", "1", "2", "", bigNum})

	c.Assert(err, NotNil)
	c.Assert(err.Error(), Matches, "*value out of range")
	c.Assert(r, IsNil)
}
