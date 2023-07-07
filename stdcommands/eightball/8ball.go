package eightball

import (
	"fmt"
	"math/rand"

	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
)

var Command = &commands.YAGCommand{
	Cooldown:    5,
	CmdCategory: commands.CategoryFun,
	Name:        "8ball",
	Description: "Ask the magic 8ball a question",
	Arguments: []*dcmd.ArgDef{
		{Name: "Question", Type: dcmd.String},
	},
	DefaultEnabled:      true,
	SlashCommandEnabled: true,
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		// Standard set of Magic 8Ball answers.
		// See https://en.wikipedia.org/wiki/Magic_8-Ball#Possible_answers
		answers := []string{
			"It is certain",
			"It is decidedly so",
			"Without a doubt",
			"Yes, definitely",
			"You may rely on it",
			"As I see it, yes",
			"Most likely",
			"Outlook good",
			"Yes",
			"Signs point to yes",
			"Reply hazy try again",
			"Ask again later",
			"Better not tell you now",
			"Cannot predict now",
			"Concentrate and ask again",
			"Don't count on it",
			"My reply is no",
			"My sources say no",
			"Outlook not so good",
			"Very doubtful",
		}

		q := data.Args[0].Str()
		return fmt.Sprintf("> %s\n:8ball: %s", q, answers[rand.Intn(len(answers))]), nil
	},
}
