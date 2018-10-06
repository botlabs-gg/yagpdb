package common

import (
	"regexp"
)

var (
	mdStrikeReg = regexp.MustCompile(`~~`)
	mdCodeReg   = regexp.MustCompile("`{3}" + `.*\n`)
	mdCodeReg2  = regexp.MustCompile(`(\x60{3})(.*?)(\x60{3})`)
	mdEmphReg   = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	mdEmphReg2  = regexp.MustCompile(`\*([^*]+)\*`)
	mdEmphReg3  = regexp.MustCompile(`__([^_]+)__`)
	mdEmphReg4  = regexp.MustCompile(`_([^_]+)_`)
)

func StripMarkdown(s string) string {
	res := s

	res = mdStrikeReg.ReplaceAllString(res, "")
	res = mdCodeReg2.ReplaceAllString(res, "$2")
	res = mdCodeReg.ReplaceAllString(res, "")
	res = mdEmphReg.ReplaceAllString(res, "$1")
	res = mdEmphReg2.ReplaceAllString(res, "$1")
	res = mdEmphReg3.ReplaceAllString(res, "$1")
	res = mdEmphReg4.ReplaceAllString(res, "$1")

	return res
}
