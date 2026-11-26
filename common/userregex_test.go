package common

import (
	"strings"
	"testing"
)

func TestFetchUserRegex(t *testing.T) {
	cache := NewUserRegexCache()
	defer cache.Stop()

	re := FetchUserRegex(cache, `^!(help|info)`, MaxTriggerRegexProgramSize)
	if re == nil || !re.MatchString("!help") {
		t.Fatal("expected a valid pattern to compile and match")
	}

	for _, p := range []string{`(unclosed`, strings.Repeat(`.{0,999}`, 10)} {
		if FetchUserRegex(cache, p, MaxTriggerRegexProgramSize) != nil {
			t.Errorf("expected %q to be rejected", p)
		}
		if cache.Get(p) == nil {
			t.Errorf("expected rejected pattern %q to be cached", p)
		}
	}
}

func TestCachedUserRegexSize(t *testing.T) {
	small := compileCachedUserRegex(`hello`, MaxTriggerRegexProgramSize)
	big := compileCachedUserRegex(`.{0,700}z`, MaxTriggerRegexProgramSize)
	invalid := compileCachedUserRegex(`(unclosed`, MaxTriggerRegexProgramSize)

	if small.Size() > 2048 {
		t.Errorf("small pattern estimated at %d bytes", small.Size())
	}
	if big.Size() < 64*1024 || big.Size() > 128*1024 {
		t.Errorf("large pattern estimated at %d bytes", big.Size())
	}
	if invalid.Size() > 512 {
		t.Errorf("invalid pattern estimated at %d bytes", invalid.Size())
	}

	worst := compileCachedUserRegex(strings.Repeat("a", MaxTriggerRegexProgramSize-2), MaxTriggerRegexProgramSize)
	if worst.re == nil || UserRegexCacheBudget/worst.Size() < 40 {
		t.Errorf("worst case pattern estimated at %d bytes, budget holds too few", worst.Size())
	}
}
