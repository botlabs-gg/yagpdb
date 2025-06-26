package dice_test

import (
	"testing"

	. "github.com/botlabs-gg/yagpdb/v2/lib/dice"
	. "gopkg.in/check.v1"
)

/* =============================================================================
 * Std dice test suite
 */

func TestStd(t *testing.T) { TestingT(t) }

type stdSuite struct {
}

/*
func (s *DiceSuite) SetUpSuite(c *C) {
}

func (s *DiceSuite) SetUpTest(c *C) {
}
*/

var _ = Suite(&stdSuite{})

/* =============================================================================
 * Std dice test cases
 */

func (s *stdSuite) TestBounds(c *C) {
	var roller StdRoller

	r, err := roller.Roll([]string{"100d2", "100", "2", "", "", "", ""})
	res := r.(StdResult)

	c.Assert(err, IsNil)

	for _, v := range res.Rolls {
		if v <= 0 || v > 2 {
			c.Errorf("Rolled out of bounds on a d2: %d", v)
		}
	}
}

func (s *stdSuite) TestCount(c *C) {
	var roller StdRoller

	r, err := roller.Roll([]string{"100d2", "100", "2", "", "", "", ""})
	res := r.(StdResult)

	c.Assert(err, IsNil)
	c.Assert(res.Rolls, HasLen, 100)
}

func (s *stdSuite) TestAdd(c *C) {
	var roller StdRoller

	r, err := roller.Roll([]string{"1d1+13", "1", "1", "", "", "", "13"})
	res := r.(StdResult)

	c.Assert(err, IsNil)
	c.Assert(res.Rolls, HasLen, 1)
	c.Assert(res.Total, Equals, 14)
}

func (s *stdSuite) TestKeep(c *C) {
	var roller StdRoller

	r, err := roller.Roll([]string{"10d6", "10", "6", "k2", "k", "2", ""})
	res := r.(StdResult)

	c.Assert(err, IsNil)
	c.Assert(res.Rolls, HasLen, 2)
	c.Assert(res.Dropped, HasLen, 8)
}

func (s *stdSuite) TestDrop(c *C) {
	var roller StdRoller

	r, err := roller.Roll([]string{"10d6", "10", "6", "d2", "d", "2", ""})
	res := r.(StdResult)

	c.Assert(err, IsNil)
	c.Assert(res.Rolls, HasLen, 8)
	c.Assert(res.Dropped, HasLen, 2)
}

func (s *stdSuite) TestBonus(c *C) {
	var roller StdRoller

	r, err := roller.Roll([]string{"1d1+1", "1", "1", "", "", "", "1"})
	res := r.(StdResult)

	c.Assert(err, IsNil)
	c.Assert(res.String(), Equals, "2 [1]")
}

func (s *stdSuite) TestNumTooBig(c *C) {
	var roller StdRoller

	//const bigNum = int(^uint(0)>>1) + 1
	const bigNum = "9223372036854775808"

	r, err := roller.Roll([]string{"1d1+1", bigNum, "1", "", "", "", "1"})

	c.Assert(err, NotNil)
	c.Assert(err.Error(), Matches, "*value out of range")
	c.Assert(r, IsNil)

	r, err = roller.Roll([]string{"1d1+1", "1", bigNum, "", "", "", "1"})

	c.Assert(err, NotNil)
	c.Assert(err.Error(), Matches, "*value out of range")
	c.Assert(r, IsNil)

	r, err = roller.Roll([]string{"1d1+1", "1", "1", "", "", "", bigNum})

	c.Assert(err, NotNil)
	c.Assert(err.Error(), Matches, "*value out of range")
	c.Assert(r, IsNil)
}
