package cardsagainstdiscord

import (
	"fmt"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/pkg/errors"
)

var Packs = make(map[string]*CardPack)

func AddPack(pack *CardPack) {
	PreparePack(pack)
	Packs[pack.Name] = pack
}

// PreparePack sets how many cards each prompt needs, prompts without a blank get one added at the end
func PreparePack(pack *CardPack) {
	for _, v := range pack.Prompts {
		numPicks := CountPromptBlanks(v.Prompt)
		if numPicks == 0 {
			v.Prompt += " %s"
			v.NumPick = 1
		} else {
			v.NumPick = numPicks
		}
	}
}

type CardPack struct {
	Name        string
	Description string
	Prompts     []*PromptCard
	Responses   []ResponseCard
}

type PromptCard struct {
	Prompt  string
	NumPick int
}

var (
	EscaperReplacer = strings.NewReplacer("*", "\\*", "_", "\\_")

	// replacers go left to right, so a literal %% is never read as the start of a %s blank
	placeholderReplacer = strings.NewReplacer("%%", "%", "%s", "_____")
	blankCounter        = strings.NewReplacer("%%", "")
)

// CountPromptBlanks returns how many cards the prompt needs, %% is a literal percent sign and not part of a blank
func CountPromptBlanks(prompt string) int {
	return strings.Count(blankCounter.Replace(prompt), "%s")
}

func (p *PromptCard) PlaceHolder() string {
	return EscaperReplacer.Replace(placeholderReplacer.Replace(p.Prompt))
}

func (p *PromptCard) WithCards(cards any) string {
	args := make([]any, p.NumPick)
	switch t := cards.(type) {
	case []string:
		for i, v := range t {
			args[i] = "**" + v + "**"
		}
	case []ResponseCard:
		for i, v := range t {
			args[i] = "**" + v + "**"
		}
	}

	s := fmt.Sprintf(p.Prompt, args...)
	return s
}

type ResponseCard string

type SessionProvider interface {
	SessionForGuild(guildID int64) *discordgo.Session
}

type StaticSessionProvider struct {
	Session *discordgo.Session
}

func (sp *StaticSessionProvider) SessionForGuild(guildID int64) *discordgo.Session {
	return sp.Session
}

var (
	ErrGameAlreadyInChannel = errors.New("Already a active game in this channel")
	ErrPlayerAlreadyInGame  = errors.New("Player already in a game")
	ErrGameNotFound         = errors.New("Game not found")
	ErrGameFull             = errors.New("Game is full")
	ErrNoPacks              = errors.New("No packs specified")
	ErrNotGM                = errors.New("You're not the game master")
	ErrStoppedAlready       = errors.New("Game already stopped")
	ErrPlayerNotInGame      = errors.New("Player not in your game")
	ErrAllPacksResponseOnly = errors.New("The set of packs specified are all response-only; at least one pack that has prompts is needed to start a game")
	ErrAllPacksPromptOnly   = errors.New("The set of packs specified have no responses; at least one pack that has responses is needed to start a game")
)

type ErrUnknownPack struct {
	PassedPack  string
	Suggestions []string
}

func (e *ErrUnknownPack) Error() string {
	if len(e.Suggestions) == 0 {
		return "Unknown pack `" + e.PassedPack + "`"
	}
	return fmt.Sprintf("Unknown pack `%s`; did you mean %s?", e.PassedPack, common.FormatList(e.Suggestions, "or"))
}

func HumanizeError(err error) string {
	err = errors.Cause(err)

	if err == ErrGameAlreadyInChannel || err == ErrPlayerAlreadyInGame || err == ErrGameNotFound || err == ErrGameFull || err == ErrNoPacks || err == ErrNotGM || err == ErrStoppedAlready || err == ErrPlayerNotInGame || err == ErrAllPacksResponseOnly || err == ErrAllPacksPromptOnly {
		return err.Error()
	}

	if c, ok := err.(*ErrUnknownPack); ok {
		return c.Error()
	}

	return ""
}
