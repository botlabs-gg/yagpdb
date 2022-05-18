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
	// Count picks
	for _, v := range pack.Prompts {
		numPicks := strings.Count(v.Prompt, "%s")
		if numPicks == 0 {
			v.Prompt += " %s"
			v.NumPick = 1
		} else {
			v.NumPick = numPicks
		}
	}

	Packs[pack.Name] = pack
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
)

func (p *PromptCard) PlaceHolder() string {
	s := strings.Replace(p.Prompt, "%s", "_____", -1)
	s = strings.Replace(s, "%%", `%`, -1)

	s = EscaperReplacer.Replace(s)

	return s
}

func (p *PromptCard) WithCards(cards interface{}) string {
	args := make([]interface{}, p.NumPick)
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
	// s = EscaperReplacer.Replace(s)
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

	if err == ErrGameAlreadyInChannel || err == ErrPlayerAlreadyInGame || err == ErrGameNotFound || err == ErrGameFull || err == ErrNoPacks || err == ErrNotGM || err == ErrStoppedAlready || err == ErrPlayerNotInGame || err == ErrAllPacksResponseOnly {
		return err.Error()
	}

	if c, ok := err.(*ErrUnknownPack); ok {
		return c.Error()
	}

	return ""
}
