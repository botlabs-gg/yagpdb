package cah

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/cah/models"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/cardsagainstdiscord"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/karlseguin/ccache"
	"github.com/sirupsen/logrus"
)

// The pack picker is shown when /cah create is used without packs, so they can be picked from select menus
// instead of being typed. What's picked is kept here until the game is started.

const (
	pickerPrefix      = "cah_setup:"
	pickerBuiltinMenu = "builtin"
	pickerCustomMenu  = "custom"
	pickerStartButton = "start"
	pickerTimeout     = 15 * time.Minute
	pickerExpiredText = "This game setup has expired, run `/cah create` again."
)

type gamePicker struct {
	GuildID   int64
	ChannelID int64
	UserID    int64
	Username  string
	VoteMode  bool

	Builtin []string
	Custom  []string

	mu sync.Mutex
	// set once the create button was pressed, so pressing it again doesn't try to create a second game
	creating bool
}

var (
	pickers       = ccache.New(ccache.Configure().MaxSize(10000))
	pickerCounter atomic.Int64
)

func newPickerKey() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36) + strconv.FormatInt(pickerCounter.Add(1), 36)
}

// startGame creates the game, returning a message for the user if it couldn't be created
func (p *Plugin) startGame(ctx context.Context, conf *models.CahGuildConfig, customPacks []*cardsagainstdiscord.CardPack, picker *gamePicker, packs []string) (string, error) {
	packs, msg := resolveGamePacks(conf, customPacks, packs)
	if msg != "" {
		return msg, nil
	}

	_, err := p.Manager.CreateGame(cardsagainstdiscord.CreateGameOptions{
		GuildID:          picker.GuildID,
		ChannelID:        picker.ChannelID,
		UserID:           picker.UserID,
		Username:         picker.Username,
		VoteMode:         picker.VoteMode,
		Packs:            packs,
		CustomPacks:      customPacks,
		PlayerLimit:      conf.PlayerLimit,
		WinLimit:         conf.WinLimit,
		PickResponseTime: time.Duration(conf.PickSeconds) * time.Second,
		PickWinnerTime:   time.Duration(conf.JudgeSeconds) * time.Second,
	})
	if err == nil {
		logrus.Info("[cah] Created a new game in ", picker.ChannelID, ":", picker.GuildID)
		return "", nil
	}

	if cahErr := cardsagainstdiscord.HumanizeError(err); cahErr != "" {
		return cahErr, nil
	}

	return "", err
}

// pickerComponents shows the packs that can be played with the ones picked selected
func pickerComponents(key string, conf *models.CahGuildConfig, customPacks []*cardsagainstdiscord.CardPack, picker *gamePicker, notice string) []discordgo.TopLevelComponent {
	text := "## New Cards Against Humanity game\nPick the packs to play and create the game, the server's default packs are already picked."
	if notice != "" {
		text += "\n\n" + notice
	}

	container := discordgo.Container{Components: []discordgo.TopLevelComponent{discordgo.TextDisplay{Content: text}}}

	var builtin []discordgo.SelectMenuOption
	for _, name := range BuiltinPackNames() {
		if slices.Contains(conf.DisabledPacks, name) {
			continue
		}

		builtin = append(builtin, discordgo.SelectMenuOption{
			Label:       name,
			Value:       name,
			Description: common.CutStringShort(cardsagainstdiscord.Packs[name].Description, 100),
			Default:     slices.Contains(picker.Builtin, name),
		})
	}

	defaultsFirst(builtin, conf.DefaultPacks)
	if len(builtin) > 0 {
		container.Components = append(container.Components, packMenu(key+":"+pickerBuiltinMenu, "Built-in packs", builtin))
	}

	var custom []discordgo.SelectMenuOption
	for _, v := range customPacks {
		custom = append(custom, discordgo.SelectMenuOption{
			Label:       v.Name,
			Value:       v.Name,
			Description: common.CutStringShort(v.Description, 100),
			Default:     slices.Contains(picker.Custom, v.Name),
		})
	}

	defaultsFirst(custom, conf.DefaultPacks)
	if len(custom) > 0 {
		container.Components = append(container.Components, packMenu(key+":"+pickerCustomMenu, "This server's packs", custom))
	}

	mode := "Card czar"
	if picker.VoteMode {
		mode = "Vote"
	}

	container.Components = append(container.Components,
		discordgo.TextDisplay{Content: "-# Mode: " + mode},
		discordgo.ActionsRow{Components: []discordgo.InteractiveComponent{discordgo.Button{
			Label:    "Create game",
			Style:    discordgo.SuccessButton,
			CustomID: pickerPrefix + key + ":" + pickerStartButton,
		}}},
	)

	return []discordgo.TopLevelComponent{container}
}

// defaultsFirst moves the server's default packs to the top in the order they're set, the others keep their order
func defaultsFirst(options []discordgo.SelectMenuOption, defaults []string) {
	rank := func(o discordgo.SelectMenuOption) int {
		if i := slices.Index(defaults, o.Value); i >= 0 {
			return i
		}
		return len(defaults)
	}

	slices.SortStableFunc(options, func(a, b discordgo.SelectMenuOption) int {
		return rank(a) - rank(b)
	})
}

func packMenu(customID, placeholder string, options []discordgo.SelectMenuOption) discordgo.ActionsRow {
	minValues := 0
	return discordgo.ActionsRow{Components: []discordgo.InteractiveComponent{discordgo.SelectMenu{
		MenuType:    discordgo.StringSelectMenu,
		CustomID:    pickerPrefix + customID,
		Placeholder: placeholder,
		MinValues:   &minValues,
		MaxValues:   len(options),
		Options:     options,
	}}}
}

// newPicker returns the picker message with the server's default packs picked
func newPicker(conf *models.CahGuildConfig, customPacks []*cardsagainstdiscord.CardPack, picker *gamePicker) []discordgo.TopLevelComponent {
	defaults, _ := resolveGamePacks(conf, customPacks, nil)
	for _, v := range defaults {
		if cardsagainstdiscord.Packs[v] != nil && !isCustomPack(customPacks, v) {
			picker.Builtin = append(picker.Builtin, v)
		} else {
			picker.Custom = append(picker.Custom, v)
		}
	}

	key := newPickerKey()
	pickers.Set(key, picker, pickerTimeout)
	return pickerComponents(key, conf, customPacks, picker, "")
}

func isCustomPack(customPacks []*cardsagainstdiscord.CardPack, name string) bool {
	return slices.ContainsFunc(customPacks, func(p *cardsagainstdiscord.CardPack) bool { return p.Name == name })
}

// handlePickerInteraction handles the pack menus and the start button of the picker, returning false for other interactions
func (p *Plugin) handlePickerInteraction(ic *discordgo.InteractionCreate) bool {
	if ic.Type != discordgo.InteractionMessageComponent {
		return false
	}

	data := ic.MessageComponentData()
	if !strings.HasPrefix(data.CustomID, pickerPrefix) {
		return false
	}

	key, action, _ := strings.Cut(strings.TrimPrefix(data.CustomID, pickerPrefix), ":")

	item := pickers.Get(key)
	if item == nil || item.Expired() {
		if ic.Message != nil && ic.Message.Flags&discordgo.MessageFlagsEphemeral != 0 {
			respondPicker(ic, pickerText(pickerExpiredText))
		} else {
			// pickers from text commands are seen by everyone, so the message is left as it is
			replyEphemeral(ic, pickerExpiredText)
		}
		return true
	}
	picker := item.Value().(*gamePicker)

	if ic.Member == nil || ic.Member.User.ID != picker.UserID {
		// only possible on pickers from text commands, slash command pickers are only shown to whoever ran it
		replyEphemeral(ic, "Only the person who started this game setup can use it.")
		return true
	}

	picker.mu.Lock()
	defer picker.mu.Unlock()

	if action != pickerStartButton {
		switch action {
		case pickerBuiltinMenu:
			picker.Builtin = data.Values
		case pickerCustomMenu:
			picker.Custom = data.Values
		}
		pickers.Set(key, picker, pickerTimeout)

		// redrawing the picker would swallow a click on the create button, which is what usually closes the menus
		ackPicker(ic)
		return true
	}

	if picker.creating {
		replyEphemeral(ic, "The game is already being created.")
		return true
	}

	// creating the game takes several requests, which could take longer than an interaction can wait for a response
	ackPicker(ic)

	conf, err := BotCachedGetConfig(picker.GuildID)
	if err != nil {
		logrus.WithError(err).Error("[cah] failed getting config for the pack picker")
		editPicker(ic, pickerText("Something went wrong, try again later."))
		return true
	}

	ctx := context.Background()
	customPacks, err := GuildCardPacks(ctx, picker.GuildID)
	if err != nil {
		logrus.WithError(err).Error("[cah] failed getting custom packs for the pack picker")
		editPicker(ic, pickerText("Something went wrong, try again later."))
		return true
	}

	packs := append(slices.Clone(picker.Builtin), picker.Custom...)
	if len(packs) == 0 {
		editPicker(ic, pickerComponents(key, conf, customPacks, picker, "**Pick at least one pack to create the game.**"))
		return true
	}

	picker.creating = true
	msg, err := p.startGame(ctx, conf, customPacks, picker, packs)
	if err != nil {
		logrus.WithError(err).Error("[cah] failed starting a game from the pack picker")
		msg = "Something went wrong creating the game."
	}

	if msg != "" {
		picker.creating = false
		editPicker(ic, pickerComponents(key, conf, customPacks, picker, "**"+msg+"**"))
		return true
	}

	pickers.Delete(key)
	editPicker(ic, pickerText(fmt.Sprintf("Game created with %s. Press %s on the game message once everyone has joined.", common.FormatList(packs, "and"), cardsagainstdiscord.PlayPauseEmoji)))
	return true
}

func pickerText(text string) []discordgo.TopLevelComponent {
	return []discordgo.TopLevelComponent{discordgo.Container{Components: []discordgo.TopLevelComponent{discordgo.TextDisplay{Content: text}}}}
}

func ackPicker(ic *discordgo.InteractionCreate) {
	err := common.BotSession.CreateInteractionResponse(ic.ID, ic.Token, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
	if err != nil {
		logrus.WithError(err).Error("[cah] failed acknowledging the pack picker")
	}
}

// editPicker updates the picker after it was acknowledged
func editPicker(ic *discordgo.InteractionCreate, components []discordgo.TopLevelComponent) {
	_, err := common.BotSession.EditOriginalInteractionResponse(ic.ApplicationID, ic.Token, &discordgo.WebhookParams{
		Components:      components,
		AllowedMentions: &discordgo.AllowedMentions{},
	})
	if err != nil {
		logrus.WithError(err).Error("[cah] failed editing the pack picker")
	}
}

func replyEphemeral(ic *discordgo.InteractionCreate, msg string) {
	err := common.BotSession.CreateInteractionResponse(ic.ID, ic.Token, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: msg, Flags: discordgo.MessageFlagsEphemeral},
	})
	if err != nil {
		logrus.WithError(err).Error("[cah] failed replying to the pack picker")
	}
}

func respondPicker(ic *discordgo.InteractionCreate, components []discordgo.TopLevelComponent) {
	err := common.BotSession.CreateInteractionResponse(ic.ID, ic.Token, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{Components: components},
	})
	if err != nil {
		logrus.WithError(err).Error("[cah] failed updating the pack picker")
	}
}
