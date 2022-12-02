package cardsagainstdiscord

func init() {
	pack := &CardPack{
		Name:        "college",
		Description: "College Pack",
		Prompts: []*PromptCard{
			&PromptCard{Prompt: `All classes today are cancelled due to %s.`},
			&PromptCard{Prompt: `Did you know? Our college was recently named the #1 school for %s!`},
			&PromptCard{Prompt: `In this paper, I will explore %s from a feminist perspective.`},
			&PromptCard{Prompt: `My memory of last night is pretty hazy. I remember %s and that's pretty much it.`},
			&PromptCard{Prompt: `Pledges! Time to prove you're Delta Phi material. Chug this beer, take off your shirts, and get ready for %s.`},
			&PromptCard{Prompt: `The Department of Psychology is looking for paid research volunteers. Are you 18-25 and suffering from %s.`},
		},
		Responses: []ResponseCard{
			`A bachelor's degeree in communications`,
			`A girl who is so interesting that she has blue hair`,
			`A Yale man`,
			`An emergency all-floor meeting on inclusion`,
			`Calling mom because it's just really hard and I miss her and I don't know anyone here`,
			`Falling in love with poetry`,
			`Five morons signing a lease together`,
			`Fucking the beat boxer from the a cappella group`,
			`Going to college and becoming a new person, who has sex`,
			`Googling how to eat pussy`,
			`How many Asians there are`,
			`My high school boyfriend`,
			`Performative wokeness`,
			`Pretending to have done the reading`,
			`Rocking a 1.5 GPA`,
			`Sucking a flaccid penis for 20 minutes`,
			`The sound of my roommate masturbating`,
			`Throw up`,
			`Uggs, leggings, and a North Face`,
			`Underage drinking`,
			`Valuable leadership experience`,
			`Wandering the streets in search of a party`,
			`Whichever one of you took a shit in the shower`,
			`Young Republicans`,
		},
	}

	AddPack(pack)
}
