package throw

import (
	"math/rand"
)

func randomThing() string {
	return throwThings[rand.Intn(len(throwThings))]
}

// If you want somthing added submit a pr
var throwThings = []string{
	"smash mouth - all star",
	"H4sIABaqgVoA/1WPvW7FIAyFd57ibHmOTlWWrlVHenFIGrCvwCTi7a9Dq0rZ0Pn5OP6kqRBYULV4jlQqVJDkIPclDTvLCV0JpSWq8BxQBUEwuzcsLSU8JOdNM7FOFefqFfOUrbLxvnGELINzSkuBJ0UkvcyKpUg2XocYviC27tyMn1YVp2f2UDJ6t+5qEy7mQpQM6d5F1SP7nYbdONhqtWnOfdBxseTqx+34CzxverIFlxzk5JtRGsMXMd64M1ClMqK31P+/j9JvRvXdXhK+O930cYd9u9HAru0X+gIEKbNKeQEAAA==",
}
