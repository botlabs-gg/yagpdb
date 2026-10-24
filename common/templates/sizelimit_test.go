package templates

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

// Each level references the previous one twice, so the serialized size doubles per level
// while the value itself costs almost nothing to build.
func doublingSlice(levels int) any {
	s := strings.Repeat("0", MaxStringLength)
	a, _ := CreateSlice(s, s)
	for range levels {
		a, _ = CreateSlice(a, a)
	}
	return a
}

func allocatedDuring(f func()) uint64 {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	f()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

func TestSerializersRejectDoublingValuesBeforeAllocating(t *testing.T) {
	huge := doublingSlice(40)
	printFn := withOutputLimit(fmt.Sprint, MaxStringLength)
	printfFn := withOutputLimitF(fmt.Sprintf, MaxStringLength)

	cases := map[string]func() error{
		"print":  func() error { _, err := printFn(huge); return err },
		"printf": func() error { _, err := printfFn("%v", huge); return err },
		"json":   func() error { _, err := tmplJson(huge); return err },
		"cembed": func() error { _, err := CreateEmbed(SDict{"description": "x", "fields": huge}); return err },
		"cbutton": func() error {
			_, err := CreateButton(SDict{"label": "x", "custom_id": huge})
			return err
		},
	}

	for name, call := range cases {
		var err error
		allocated := allocatedDuring(func() { err = call() })
		if err == nil {
			t.Errorf("%s: expected a 2^40 MB value to be rejected", name)
		}
		if allocated > 16<<20 {
			t.Errorf("%s: rejecting should not build the output, allocated %dMB", name, allocated>>20)
		}
	}
}

func TestSerializersAllowOrdinaryValues(t *testing.T) {
	small := doublingSlice(3)
	if exceedsSerializedSize(sizeModeEncode, 20*MaxStringLength, small) {
		t.Fatal("16 x 1MB is under a 20MB limit and must not be rejected")
	}

	embed := SDict{
		"title":       "Leaderboard",
		"description": strings.Repeat("line\n", 500),
		"fields":      Slice{SDict{"name": "a", "value": "b", "inline": true}},
	}
	if _, err := CreateEmbed(embed); err != nil {
		t.Fatalf("ordinary embed rejected: %v", err)
	}
	if _, err := tmplJson(embed); err != nil {
		t.Fatalf("ordinary json rejected: %v", err)
	}

	printFn := withOutputLimit(fmt.Sprint, MaxStringLength)
	if _, err := printFn(strings.Repeat("x", MaxStringLength-10), 12345); err != nil {
		t.Fatalf("print just under the limit rejected: %v", err)
	}
}

func TestSizeCheckBoundsHolds(t *testing.T) {
	type node struct {
		Name string
		Next *node
	}
	cycle := &node{Name: "a"}
	cycle.Next = cycle
	if !exceedsSerializedSize(sizeModeEncode, 1000, cycle) {
		t.Fatal("a pointer cycle must terminate and be treated as too large")
	}

	if exceedsSerializedSize(sizeModeFormat, 1000, Slice{cycle}) {
		t.Fatal("fmt prints nested pointers as addresses, so they must not be followed")
	}

	user := &discordgo.User{ID: 1, Username: strings.Repeat("u", 100)}
	if exceedsSerializedSize(sizeModeFormat, 50, user) {
		t.Fatal("types with String() are printed via that method, their fields must not be counted")
	}

	if !exceedsSerializedSize(sizeModeEncode, 1000, []byte(strings.Repeat("b", 1001))) {
		t.Fatal("byte slices must count their length")
	}
}
