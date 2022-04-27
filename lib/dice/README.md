# Golang dice

This is a simple library for rolling RPG-style dice. The following formats are supported:

* Standard: `xdy[[k|d][h|l]z][+/-c]` - rolls and sums x y-sided dice, keeping or dropping the lowest or highest z dice and optionally adding or subtracting c. Example: 4d6kh3+4
* Versus: `xdy[e|r]vt` - rolls x y-sided dice, counting the number that roll t or greater.
* EotE: `xc [xc ...]` - rolls x dice of color c (b, blk, g, p, r, w, y) and returns the aggregate result.

Adding an `e` to the Versus rolls above makes dice 'explode' - Dice are rerolled and have the 
rolled value added to their total when they roll a y. Adding an `r` makes dice rolling a y add another die
to the pool instead.

## Installation

The usual `go get github.com/justinian/dice`.

## Usage

The main entrypoint to the library is the `dice.Roll` function:

```go
func Roll(desc string) (fmt.Stringer, error)
```

The `desc` argument takes any string that matches the formats above and rolls
correctly. The result is returned as a `RollResult` (which is a `fmt.Stringer`
for simple printing, but also contains different information based on the type
of roll). See the individual roll styles for their result structures.
