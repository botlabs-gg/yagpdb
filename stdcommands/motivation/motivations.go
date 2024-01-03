package motivation

import (
	"math/rand"
)

func randommotivation() string {
	return Motivations[rand.Intn(len(Motivations))]
}

var Motivations = []string{
	"Stop bitching and start doing",
	"Shit is hard, do it anyway",
	"Maybe swearing will help",
	"It is totally ok to not give a fuck sometimes",
	"Leave the bullshit in the past",
	"You are strong as fuck",
	"Don't give up; you still have a lot of motherfuckers to prove wrong",
	"Know your fucking worth",
	"Be un-fucking stoppable",
	"Get Shit Done",
	"Make today your bitch",
	"Life is a bitch—just do your best",
	"Nothing in life is fair\nTough Shit",
	"You're doing great; keep that shit up!",
	"YOU are fucking awesome",
	"The key to life is simple... just don't be an asshole",
	"Wake up\nKick ass\nRepeat",
	"Don't let one asshole ruin your day",
	"Exhale the bullshit",
	"Prove them bitches wrong",
}
