package newtopic

import (
	"math/rand"
)

func randomTopic() string {
	return topicList[rand.Intn(len(topicList))]
}

var topicList = []string{
	"why cats can't decide to go inside or outside",
	"what type of weather you're having",
	"the global impact of fruit rollups",
}
