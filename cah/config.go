package cah

import (
	"context"
	"database/sql"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/cah/models"
	"github.com/botlabs-gg/yagpdb/v2/common/pubsub"
	"github.com/botlabs-gg/yagpdb/v2/lib/cardsagainstdiscord"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/karlseguin/ccache"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

const (
	MaxCustomPacks    = 10
	MaxCardsPerPack   = 500
	MaxPromptLength   = 300
	MaxResponseLength = 200
	MaxPromptBlanks   = 3
)

func DefaultConfig(guildID int64) *models.CahGuildConfig {
	return &models.CahGuildConfig{
		GuildID:      guildID,
		DefaultPacks: []string{"main"},
		PlayerLimit:  10,
		WinLimit:     10,
		PickSeconds:  int(cardsagainstdiscord.PickResponseDuration.Seconds()),
		JudgeSeconds: int(cardsagainstdiscord.PickWinnerDuration.Seconds()),
	}
}

func FetchConfig(guildID int64) (*models.CahGuildConfig, error) {
	conf, err := models.FindCahGuildConfigG(context.Background(), guildID)
	if err == sql.ErrNoRows {
		return DefaultConfig(guildID), nil
	}

	return conf, err
}

func SaveConfig(conf *models.CahGuildConfig) error {
	err := conf.UpsertG(context.Background(), true, []string{"guild_id"}, boil.Infer(), boil.Infer())
	if err != nil {
		return err
	}

	pubsub.Publish("invalidate_cah_config_cache", conf.GuildID, nil)
	configCache.Delete(cacheKey(conf.GuildID))
	return nil
}

var configCache = ccache.New(ccache.Configure().MaxSize(15000))

func BotCachedGetConfig(guildID int64) (*models.CahGuildConfig, error) {
	item, err := configCache.Fetch(cacheKey(guildID), 10*time.Minute, func() (any, error) {
		return FetchConfig(guildID)
	})
	if err != nil {
		return nil, err
	}

	return item.Value().(*models.CahGuildConfig), nil
}

func handleInvalidateConfigCache(evt *pubsub.Event) {
	configCache.Delete(cacheKey(evt.TargetGuildInt))
}

func cacheKey(guildID int64) string {
	return discordgo.StrID(guildID)
}

func GetCustomPacks(ctx context.Context, guildID int64) (models.CahCustomPackSlice, error) {
	return models.CahCustomPacks(models.CahCustomPackWhere.GuildID.EQ(guildID), qm.OrderBy("lower(name)")).AllG(ctx)
}

// GuildCardPacks returns the custom packs of the guild ready to be played, which are only available to premium guilds
func GuildCardPacks(ctx context.Context, guildID int64) ([]*cardsagainstdiscord.CardPack, error) {
	isPremium, err := premium.IsGuildPremium(guildID)
	if err != nil || !isPremium {
		return nil, err
	}

	packs, err := GetCustomPacks(ctx, guildID)
	if err != nil {
		return nil, err
	}

	result := make([]*cardsagainstdiscord.CardPack, 0, len(packs))
	for _, v := range packs {
		result = append(result, CardPackFromModel(v))
	}

	return result, nil
}

func CardPackFromModel(m *models.CahCustomPack) *cardsagainstdiscord.CardPack {
	pack := &cardsagainstdiscord.CardPack{
		Name:        m.Name,
		Description: m.Description,
	}

	for _, v := range m.Prompts {
		pack.Prompts = append(pack.Prompts, &cardsagainstdiscord.PromptCard{Prompt: PromptFromLine(v)})
	}

	for _, v := range m.Responses {
		pack.Responses = append(pack.Responses, cardsagainstdiscord.ResponseCard(v))
	}

	cardsagainstdiscord.PreparePack(pack)
	return pack
}

var blankRe = regexp.MustCompile(`_{3,}`)

// PromptFromLine converts a prompt written with ___ for each blank to the format the game uses
func PromptFromLine(line string) string {
	return blankRe.ReplaceAllString(strings.ReplaceAll(line, "%", "%%"), "%s")
}

// LineFromPrompt converts a prompt the game uses back to the ___ format of the editor
func LineFromPrompt(prompt string) string {
	return lineReplacer.Replace(prompt)
}

var lineReplacer = strings.NewReplacer("%%", "%", "%s", "___")

func CountBlanks(line string) int {
	return len(blankRe.FindAllStringIndex(line, -1))
}

// ParseCardLines splits text with one card per line, ignoring empty lines
func ParseCardLines(text string) []string {
	var cards []string
	for line := range strings.SplitSeq(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			cards = append(cards, line)
		}
	}

	return cards
}

// BuiltinPackNames returns the names of the built in packs, sorted
func BuiltinPackNames() []string {
	names := make([]string, 0, len(cardsagainstdiscord.Packs))
	for k := range cardsagainstdiscord.Packs {
		names = append(names, k)
	}

	sort.Strings(names)
	return names
}
