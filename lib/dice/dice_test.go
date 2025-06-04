package dice_test

import (
	"testing"

	. "github.com/botlabs-gg/yagpdb/v2/lib/dice"
)

func TestRoll(t *testing.T) {
	roll := "3d8v3"
	res, _, _ := Roll(roll)
	if _, ok := res.(VsResult); !ok {
		t.Fatalf("%s is not a VsResult", roll)
	}

	roll = "3d8+2test"
	_, _, err := Roll(roll)
	if err != nil {
		t.Logf("err '%v' properly detected in %s", err, roll)
	} else {
		t.Fatalf("err not detected in %s", roll)
	}

	roll = "4d0"
	_, _, err = Roll(roll)
	if err != nil {
		t.Logf("err '%v' properly detected in %s", err, roll)
	} else {
		t.Fatalf("err not detected in %s", roll)
	}

	roll = "4d0v5"
	_, _, err = Roll(roll)
	if err != nil {
		t.Logf("err '%v' properly detected in %s", err, roll)
	} else {
		t.Fatalf("err not detected in %s", roll)
	}

	roll = "3b4bl"
	_, reason, err := Roll(roll)
	if reason == "4bl" {
		t.Fatalf("malformed dice format read as reason, %s", roll)
	}
	if err != nil {
		t.Logf("err '%v' properly detected in %s", err, roll)
	}

	roll = "9d9rv5"
	res, _, _ = Roll(roll)
	if _, ok := res.(VsResult); !ok {
		t.Fatalf("%s is not a VsResult", roll)
	}

	// Trying to keep or drop too many dice
	roll = "2d6k5"
	_, _, err = Roll(roll)
	if err != nil {
		t.Logf("err '%v' properly detected in %s", err, roll)
	} else {
		t.Fatalf("err not detected in %s", roll)
	}

	roll = "2d6kl5"
	_, _, err = Roll(roll)
	if err != nil {
		t.Logf("err '%v' properly detected in %s", err, roll)
	} else {
		t.Fatalf("err not detected in %s", roll)
	}

	roll = "2d6dh5"
	_, _, err = Roll(roll)
	if err != nil {
		t.Logf("err '%v' properly detected in %s", err, roll)
	} else {
		t.Fatalf("err not detected in %s", roll)
	}

	roll = "2d6dl5"
	_, _, err = Roll(roll)
	if err != nil {
		t.Logf("err '%v' properly detected in %s", err, roll)
	} else {
		t.Fatalf("err not detected in %s", roll)
	}
}

func TestText(t *testing.T) {
	roll := "1d20"
	why := "death save"
	res, reason, _ := Roll(roll + " " + why)
	if res.Description() != roll {
		t.Fatalf("desc does not match roll: %s", roll)
	}
	if reason != why {
		t.Fatalf("reason does not match reason: %s", reason)
	}

	roll = "1d20v10"
	res, _, _ = Roll(roll)
	if res.Description() != roll {
		t.Fatalf("desc does not match roll: %s", roll)
	}

	roll = "1w1b2y"
	res, _, _ = Roll(roll)
	if res.Description() != roll {
		t.Fatalf("desc does not match roll: %s", roll)
	}
}

func TestResultInt(t *testing.T) {
	roll := "6d1" // aka 6
	res, _, _ := Roll(roll)
	if res.Int() != 6 {
		t.Fatalf("%s does not evaluate to 6", roll)
	}

	roll = "10d10v1" // 10 successes
	res, _, _ = Roll(roll)
	if res.Int() != 10 {
		t.Fatalf("%s fails to always roll at least 1", roll)
	}

	roll = "1w" // no success possible
	res, _, _ = Roll(roll)
	if res.Int() != 0 {
		t.Fatalf("%s fails to roll zero successes", roll)
	}
}
