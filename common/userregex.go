package common

import (
	"fmt"
	"regexp"
	"regexp/syntax"
	"time"

	"github.com/karlseguin/ccache"
)

// Matching costs roughly the compiled program size times the input length, and counted repeats such as
// .{0,999} make a short pattern compile to a very large program, so user supplied regexes are capped by
// program size rather than pattern length.
const (
	// Triggers run against every message. Long message checks like (.|\n){700,} and stretched words like
	// hi{2,1000} are common and the largest trigger in use compiles to about 8000, while stacked repeats such
	// as .{0,999}.{0,999}... reach 20000 and more.
	MaxTriggerRegexProgramSize = 10000
	// Leaves room for chunking idioms such as .{1,1000}, which compiles to about 2000.
	MaxTemplateRegexProgramSize = 10000
)

// UserRegexCacheBudget is the approximate number of bytes of compiled regexes a user regex cache holds.
// A regex at MaxTriggerRegexProgramSize is estimated at about 640KB, so this is about 50 worst case
// patterns or many thousands of typical ones.
const UserRegexCacheBudget = 32 << 20

func userRegexProgramSize(pattern string) (int, error) {
	parsed, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return 0, err
	}

	prog, err := syntax.Compile(parsed.Simplify())
	if err != nil {
		return 0, err
	}
	return len(prog.Inst), nil
}

// CheckUserRegex returns an error if pattern is invalid or compiles to more than maxProgramSize instructions.
func CheckUserRegex(pattern string, maxProgramSize int) error {
	n, err := userRegexProgramSize(pattern)
	if err != nil {
		return err
	}

	if n > maxProgramSize {
		return fmt.Errorf("regex is too complex (compiles to %d instructions, max %d), try smaller repeat counts", n, maxProgramSize)
	}
	return nil
}

// CompileUserRegex compiles a user supplied regex after checking it with CheckUserRegex.
func CompileUserRegex(pattern string, maxProgramSize int) (*regexp.Regexp, error) {
	if err := CheckUserRegex(pattern, maxProgramSize); err != nil {
		return nil, err
	}
	return regexp.Compile(pattern)
}

// cachedUserRegex implements ccache.Sized so the cache's MaxSize acts as a byte budget.
type cachedUserRegex struct {
	re   *regexp.Regexp
	size int64
}

func (c *cachedUserRegex) Size() int64 { return c.size }

// Measured at roughly 50 bytes per instruction plus a few hundred bytes of fixed overhead, rounded up.
func compileCachedUserRegex(pattern string, maxProgramSize int) *cachedUserRegex {
	invalid := &cachedUserRegex{size: 256 + int64(len(pattern))}

	n, err := userRegexProgramSize(pattern)
	if err != nil || n > maxProgramSize {
		return invalid
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return invalid
	}
	return &cachedUserRegex{re: re, size: 512 + 64*int64(n) + int64(len(pattern))}
}

func NewUserRegexCache() *ccache.Cache {
	return ccache.New(ccache.Configure().MaxSize(UserRegexCacheBudget))
}

// FetchUserRegex returns the compiled pattern from a cache made by NewUserRegexCache, or nil if the pattern is
// invalid or too complex. Invalid patterns are cached too so they aren't compiled again for every message.
func FetchUserRegex(cache *ccache.Cache, pattern string, maxProgramSize int) *regexp.Regexp {
	item, err := cache.Fetch(pattern, time.Minute*10, func() (any, error) {
		return compileCachedUserRegex(pattern, maxProgramSize), nil
	})
	if err != nil {
		return nil
	}
	return item.Value().(*cachedUserRegex).re
}
