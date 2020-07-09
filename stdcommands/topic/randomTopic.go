package topic

import (
	"math/rand"
)

func randomTopic() string {
	return throwThings[rand.Intn(len(throwThings))]
}

var throwThings = []string{
	"why cats can't decide to go inside or outside.",
	"what type of weather you're having",
	"The global impact of fruit rollups",
}
