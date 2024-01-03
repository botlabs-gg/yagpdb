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
	"Embrace the suck; success is on the other damn side",
	"Stop dreaming, start fucking doing",
	"Tough times don't last; tough people do, motherfucker",
	"In the middle of difficulty lies a fucking opportunity",
	"Do it with passion or don't fucking do it at all",
	"Success is the best goddamn revenge",
	"Hustle until your haters ask if you're hiring, bitch",
	"Don't just exist; live with a fucking purpose",
	"Fear is a liar; action is the goddamn truth",
	"You're not a mess; you're a fucking masterpiece in progress",
	"Your vibe attracts your fucking tribe",
	"The only limit is the one you set for your damn self",
	"Your future self will thank you for the shit you deal with today",
	"Chase your fucking dreams; in high heels, of course",
	"Rise like the goddamn sun and fucking burn",
	"Success is the sum of small efforts repeated day in and fucking day out",
	"Be so damn good they can't ignore you",
	"Your only limit is your fucking mind",
	"Make it fucking happen; shock the hell out of everyone",
	"Keep your head high, keep your chin up, and keep smiling, because life's a beautiful damn thing",
	"Don't be afraid to fucking fail; be afraid not to try",
}
