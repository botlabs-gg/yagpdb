package stdcommands

import (
	"math/rand"
)

func RandomThing() string {
	return ThrowThings[rand.Intn(len(ThrowThings))]
}

// If you want somthing added submit a pr
var ThrowThings = []string{
	"anime girls",
	"b1nzy",
	"bad jokes",
	"a boom box",
	"graliko~ (from GBDS)",
	"old cheese",
	"heroin",
	"sadness",
	"depression",
	"an evil villain with a plan to destroy earth",
	"a superhero on his way to stop an evil villain",
	"jonas747#0001",
	"hot firemen",
	"fat idiots",
	"a hairy potter",
	"hentai",
	"a soft knife",
	"a sharp pillow",
	"love",
	"hate",
	"a tomato",
	"a time machine that takes 1 year to travel 1 year into the future",
	"sadness disguised as hapiness",
	"debt",
	"all your imaginary friends",
	"homelessness",
	"stupid bot commands",
	"bad bots",
	"ol' musky",
	"wednesday, my dudes",
	"wednesday frog",
	"flat earthers",
	"round earthers",
	"prequel memes",
	"logan paul and his dead body",
	"selfbots",
	"very big fish",
	"ur mum",
	"the biggest fattest vape",
	"donald trump's wall",
	"smash mouth - all star",
	"H4sIABaqgVoA/1WPvW7FIAyFd57ibHmOTlWWrlVHenFIGrCvwCTi7a9Dq0rZ0Pn5OP6kqRBYULV4jlQqVJDkIPclDTvLCV0JpSWq8BxQBUEwuzcsLSU8JOdNM7FOFefqFfOUrbLxvnGELINzSkuBJ0UkvcyKpUg2XocYviC27tyMn1YVp2f2UDJ6t+5qEy7mQpQM6d5F1SP7nYbdONhqtWnOfdBxseTqx+34CzxverIFlxzk5JtRGsMXMd64M1ClMqK31P+/j9JvRvXdXhK+O930cYd9u9HAru0X+gIEKbNKeQEAAA==",
	"death",
	"tide pods",
	"a happy life with a good career and a nice family",
	"a sad life with a deadend career and a horrible family",
	"divorce papers",
	"an engagement ring",
	"yourself",
	"nothing",
	"insults",
	"compliments",
	"life advice",
	"scams",
}
