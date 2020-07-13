package throw

import (
	"math/rand"
)

func randomThing() string {
	return throwThings[rand.Intn(len(throwThings))]
}

// If you want somthing added submit a pr
var throwThings = []string{
	"H4sIABaqgVoA/1WPvW7FIAyFd57ibHmOTlWWrlVHenFIGrCvwCTi7a9Dq0rZ0Pn5OP6kqRBYULV4jlQqVJDkIPclDTvLCV0JpSWq8BxQBUEwuzcsLSU8JOdNM7FOFefqFfOUrbLxvnGELINzSkuBJ0UkvcyKpUg2XocYviC27tyMn1YVp2f2UDJ6t+5qEy7mQpQM6d5F1SP7nYbdONhqtWnOfdBxseTqx+34CzxverIFlxzk5JtRGsMXMd64M1ClMqK31P+/j9JvRvXdXhK+O930cYd9u9HAru0X+gIEKbNKeQEAAA==",
	"the semi-colon that was missing from your last code project",
	"a rick roll",
	"spiderpig",
	"a 52K internet connection",
	"a life with only slow internet",
	"a life with only 10gb internet",
	"a free trip to japan",
	"a free trip to USA",
	"a magic 8 ball",
	"lord voldemort",
	"the sound of silence",
	"the Windows XP startup sound",
	"a .vbs script",
	"an Excel macro written by somebody who hasn't been with the company for ten years",
	"an Access database",
	"untested patches",
	"soylent green",
	"a new linux distro",
	"a copy of windows XP",
	"a copy of Windows ME",
	"a copy of Windows 8.1",
	"a copy of Windows Vista - ew",
	"a copy of Windows 10",
	"epoch time",
	"a copy of server 2000",
	"a copy of server 2003",
	"a copy of server 2008",
	"a copy of server 2008R2"
	"a pink parrot",
	"an undocumented 'feature'",
	"a bug disguised as a feature",
	"a feature disguised as a bug",
	"the last book you read",
	"an exam certificate",
	"the mall of america",
	"a cake baked by @JGKPS",
	"a brisket made by @JordanTheItGuy",
	"a bottle of spytail courtesy of @FSociety",
	"a printer ink cartridge",
	"ID10T error",
	"PEBCAK error",
	"0x80004005",
	"a firewall",
	"a brand new shiny server rack with terribly cable management",
	"the worlds oldest server rack with perfect cable management",
	"a roll of velcro, for cable management",
	"a brand new hard drive",
	"an old spindle hard drive",
	"LMGTFY",
	"overtime pay",
	"a perfect job",
	"a deployment bunny",
	"some deployment research",
	"a powershell script with no logging",
}
