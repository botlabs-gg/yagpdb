package discordgo

// https://docs.discord.com/developers/resources/poll
type PollLayoutType int

const PollLayoutTypeDefault PollLayoutType = 1

type Poll struct {
	Question         PollMedia      `json:"question"`
	Answers          []PollAnswer   `json:"answers"`
	Expiry           *Timestamp     `json:"expiry"`
	AllowMultiselect bool           `json:"allow_multiselect"`
	LayoutType       PollLayoutType `json:"layout_type"`
	Results          *PollResults   `json:"results,omitempty"`
}

type PollMedia struct {
	Text  string          `json:"text,omitempty"`
	Emoji *ComponentEmoji `json:"emoji,omitempty"`
}

type PollAnswer struct {
	AnswerID  int       `json:"answer_id,omitempty"`
	PollMedia PollMedia `json:"poll_media"`
}

type PollResults struct {
	IsFinalized  bool              `json:"is_finalized"`
	AnswerCounts []PollAnswerCount `json:"answer_counts"`
}

type PollAnswerCount struct {
	ID      int  `json:"id"`
	Count   int  `json:"count"`
	MeVoted bool `json:"me_voted"`
}

// TextContents returns the question followed by the text of every answer.
func (p *Poll) TextContents() []string {
	contents := appendNonEmpty(nil, p.Question.Text)
	for _, a := range p.Answers {
		contents = appendNonEmpty(contents, a.PollMedia.Text)
	}
	return contents
}
