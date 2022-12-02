package cardsagainstdiscord

func init() {
	pack := &CardPack{
		Name:        "20-blanks",
		Description: "Deck of 20 blank response cards",
		Prompts: []*PromptCard{},
		Responses: []ResponseCard{
			`%blank`,
			`%blank`,
			`%blank`,
			`%blank`,
			`%blank`,
			`%blank`,
			`%blank`,
			`%blank`,
			`%blank`,
			`%blank`,
			`%blank`,
			`%blank`,
			`%blank`,
			`%blank`,
			`%blank`,
			`%blank`,
			`%blank`,
			`%blank`,
			`%blank`,
			`%blank`,
		},
	}

	AddPack(pack)
}
