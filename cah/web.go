package cah

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/botlabs-gg/yagpdb/v2/cah/models"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/cplogs"
	"github.com/botlabs-gg/yagpdb/v2/lib/cardsagainstdiscord"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"goji.io"
	"goji.io/pat"
)

//go:embed assets/cah.html
var PageHTML string

var (
	panelLogKeyUpdatedSettings = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "cah_updated_settings", FormatString: "Updated CAH settings"})
	panelLogKeyAddedPack       = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "cah_added_pack", FormatString: "Added CAH pack %s"})
	panelLogKeyUpdatedPack     = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "cah_updated_pack", FormatString: "Updated CAH pack %s"})
	panelLogKeyRemovedPack     = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "cah_removed_pack", FormatString: "Removed CAH pack %s"})
)

type SettingsForm struct {
	DefaultPacks []string
	EnabledPacks []string

	VoteMode     bool
	PlayerLimit  int `valid:"2,11"`
	WinLimit     int `valid:"1,25"`
	PickSeconds  int `valid:"30,180"`
	JudgeSeconds int `valid:"30,180"`
}

// PackForm is checked in Validate rather than with valid tags, so what was typed is kept when it fails
type PackForm struct {
	Name        string
	Description string
	Prompts     string
	Responses   string
}

const (
	maxPackNameLength        = 40
	maxPackDescriptionLength = 200
	// the most one textarea can hold with every card at its longest
	maxPackTextLength = MaxCardsPerPack * (MaxPromptLength + 1)
)

// pack names are typed in /cah create and shown in code blocks, so they're kept to characters that work in both
var packNameRe = regexp.MustCompile(`^[\p{L}\p{N}._-]+$`)

var _ web.CustomValidator = (*PackForm)(nil)

func (f *PackForm) Validate(tmpl web.TemplateData, guildID int64) bool {
	f.Name = strings.TrimSpace(f.Name)
	f.Description = strings.TrimSpace(f.Description)

	prompts, responses := ParseCardLines(f.Prompts), ParseCardLines(f.Responses)

	var problems []string
	switch {
	case f.Name == "":
		problems = append(problems, "The pack needs a name")
	case utf8.RuneCountInString(f.Name) > maxPackNameLength:
		problems = append(problems, fmt.Sprintf("Pack names can be at most %d characters", maxPackNameLength))
	case !packNameRe.MatchString(f.Name):
		problems = append(problems, "Pack names can only have letters, numbers, dots, dashes and underscores, they're typed in /cah create")
	case cardsagainstdiscord.Packs[f.Name] != nil:
		problems = append(problems, fmt.Sprintf("There's already a built-in pack named %s", f.Name))
	}

	if utf8.RuneCountInString(f.Description) > maxPackDescriptionLength {
		problems = append(problems, fmt.Sprintf("Descriptions can be at most %d characters", maxPackDescriptionLength))
	}

	if len(f.Prompts) > maxPackTextLength || len(f.Responses) > maxPackTextLength {
		problems = append(problems, fmt.Sprintf("Packs can have at most %d cards", MaxCardsPerPack))
	}

	total := len(prompts) + len(responses)
	if total == 0 {
		problems = append(problems, "The pack needs at least one card")
	} else if total > MaxCardsPerPack {
		problems = append(problems, fmt.Sprintf("Packs can have at most %d cards, this one has %d", MaxCardsPerPack, total))
	}

	for _, v := range prompts {
		if utf8.RuneCountInString(v) > MaxPromptLength {
			problems = append(problems, fmt.Sprintf("Prompts can be at most %d characters: %s", MaxPromptLength, common.CutStringShort(v, 50)))
		} else if CountBlanks(v) > MaxPromptBlanks {
			problems = append(problems, fmt.Sprintf("Prompts can have at most %d blanks: %s", MaxPromptBlanks, common.CutStringShort(v, 50)))
		}
	}

	for _, v := range responses {
		if utf8.RuneCountInString(v) > MaxResponseLength {
			problems = append(problems, fmt.Sprintf("Responses can be at most %d characters: %s", MaxResponseLength, common.CutStringShort(v, 50)))
		}
	}

	for _, v := range problems {
		tmpl.AddAlerts(web.ErrorAlert(v))
	}

	if len(problems) > 0 {
		tmpl["PackDraft"] = f
	}

	return len(problems) == 0
}

// PackView is a custom pack with its cards joined back into one card per line for the editor
type PackView struct {
	ID          int64
	Name        string
	Description string
	Prompts     string
	Responses   string
	NumCards    int
	IsDefault   bool

	// the name as saved, the other fields can hold what was typed when saving failed
	SavedName string
	// expanded in the accordion, after saving it or opening its link
	Open bool
}

type BuiltinPackView struct {
	Name         string
	Description  string
	Enabled      bool
	IsDefault    bool
	NumPrompts   int
	NumResponses int

	// the cards in the same format as the editor
	Prompts   string
	Responses string
}

func (p *Plugin) InitWeb() {
	web.AddHTMLTemplate("cah/assets/cah.html", PageHTML)
	web.AddSidebarItem(web.SidebarCategoryFun, &web.SidebarItem{
		Name: "Cards Against Humanity",
		URL:  "cah/",
		Icon: "fas fa-clone",
	})

	cpMux := goji.SubMux()
	web.CPMux.Handle(pat.New("/cah/*"), cpMux)
	web.CPMux.Handle(pat.New("/cah"), cpMux)

	cpMux.Use(web.RequireBotMemberMW)

	getHandler := web.ControllerHandler(HandleGetCAH, "cp_cah")
	cpMux.Handle(pat.Get(""), getHandler)
	cpMux.Handle(pat.Get("/"), getHandler)
	cpMux.Handle(pat.Get("/packs/:id"), getHandler)

	cpMux.Handle(pat.Post(""), web.ControllerPostHandler(HandlePostSettings, getHandler, SettingsForm{}))
	cpMux.Handle(pat.Post("/"), web.ControllerPostHandler(HandlePostSettings, getHandler, SettingsForm{}))
	cpMux.Handle(pat.Post("/packs/new"), web.ControllerPostHandler(HandleNewPack, getHandler, PackForm{}))
	cpMux.Handle(pat.Post("/packs/:id/update"), web.ControllerPostHandler(HandleUpdatePack, getHandler, PackForm{}))
	cpMux.Handle(pat.Post("/packs/:id/delete"), web.ControllerPostHandler(HandleDeletePack, getHandler, nil))
}

func HandleGetCAH(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	g, tmpl := web.GetBaseCPContextData(ctx)

	conf, ok := tmpl["CAHConfig"].(*models.CahGuildConfig)
	if !ok {
		var err error
		conf, err = FetchConfig(g.ID)
		if err != nil {
			return tmpl, err
		}
		tmpl["CAHConfig"] = conf
	}

	packs, err := GetCustomPacks(ctx, g.ID)
	if err != nil {
		return tmpl, err
	}

	views := make([]*PackView, 0, len(packs))
	for _, v := range packs {
		views = append(views, &PackView{
			ID:          v.ID,
			Name:        v.Name,
			Description: v.Description,
			Prompts:     strings.Join(v.Prompts, "\n"),
			Responses:   strings.Join(v.Responses, "\n"),
			NumCards:    len(v.Prompts) + len(v.Responses),
			IsDefault:   slices.Contains(conf.DefaultPacks, v.Name),
			SavedName:   v.Name,
		})
	}

	// the pack to expand, set by handlers that just created one, otherwise taken from the url
	currentID, ok := tmpl["CurrentPackID"].(int64)
	if !ok {
		currentID, _ = strconv.ParseInt(web.ParamOrEmpty(r, "id"), 10, 64)
	}

	draft, _ := tmpl["PackDraft"].(*PackForm)
	tmpl["NewPack"] = &PackForm{}
	openedPack := false
	for _, v := range views {
		if v.ID != currentID {
			continue
		}

		v.Open = true
		openedPack = true
		if draft != nil {
			// keep what was typed when saving failed
			v.Name, v.Description, v.Prompts, v.Responses = draft.Name, draft.Description, draft.Prompts, draft.Responses
		}
	}

	if !openedPack && draft != nil {
		tmpl["NewPack"] = draft
	}

	builtin := make([]*BuiltinPackView, 0, len(cardsagainstdiscord.Packs))
	for _, name := range BuiltinPackNames() {
		pack := cardsagainstdiscord.Packs[name]
		prompts, responses := builtinCards(pack)
		builtin = append(builtin, &BuiltinPackView{
			Name:         name,
			Description:  pack.Description,
			Enabled:      !slices.Contains(conf.DisabledPacks, name),
			IsDefault:    slices.Contains(conf.DefaultPacks, name),
			NumPrompts:   len(pack.Prompts),
			NumResponses: len(pack.Responses),
			Prompts:      prompts,
			Responses:    responses,
		})
	}

	tmpl["CustomPacks"] = views
	tmpl["BuiltinPacks"] = builtin
	tmpl["MaxCustomPacks"] = MaxCustomPacks
	tmpl["MaxCardsPerPack"] = MaxCardsPerPack
	tmpl["MaxPromptBlanks"] = MaxPromptBlanks
	return tmpl, nil
}

func HandlePostSettings(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	g, tmpl := web.GetBaseCPContextData(ctx)
	tmpl["VisibleURL"] = "/manage/" + discordgo.StrID(g.ID) + "/cah/"

	form := ctx.Value(common.ContextKeyParsedForm).(*SettingsForm)

	conf := &models.CahGuildConfig{
		GuildID:       g.ID,
		DisabledPacks: []string{},
		VoteMode:      form.VoteMode,
		PlayerLimit:   form.PlayerLimit,
		WinLimit:      form.WinLimit,
		PickSeconds:   form.PickSeconds,
		JudgeSeconds:  form.JudgeSeconds,
	}

	for _, name := range BuiltinPackNames() {
		if !slices.Contains(form.EnabledPacks, name) {
			conf.DisabledPacks = append(conf.DisabledPacks, name)
		}
	}

	available, err := availablePackNames(ctx, g.ID, conf.DisabledPacks)
	if err != nil {
		return tmpl, err
	}

	conf.DefaultPacks = []string{}
	for _, v := range form.DefaultPacks {
		if slices.Contains(available, v) && !slices.Contains(conf.DefaultPacks, v) {
			conf.DefaultPacks = append(conf.DefaultPacks, v)
		}
	}

	tmpl["CAHConfig"] = conf
	if len(conf.DefaultPacks) == 0 {
		return tmpl.AddAlerts(web.ErrorAlert("Pick at least one enabled pack to play by default")), nil
	}

	err = SaveConfig(conf)
	if err == nil {
		go cplogs.RetryAddEntry(web.NewLogEntryFromContext(ctx, panelLogKeyUpdatedSettings))
	}

	return tmpl, err
}

// availablePackNames returns the enabled built-in packs along with the custom packs if the server is premium
func availablePackNames(ctx context.Context, guildID int64, disabled []string) ([]string, error) {
	var names []string
	for _, name := range BuiltinPackNames() {
		if !slices.Contains(disabled, name) {
			names = append(names, name)
		}
	}

	if !premium.ContextPremium(ctx) {
		return names, nil
	}

	packs, err := GetCustomPacks(ctx, guildID)
	if err != nil {
		return nil, err
	}

	for _, v := range packs {
		names = append(names, v.Name)
	}

	return names, nil
}

func HandleNewPack(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	g, tmpl := web.GetBaseCPContextData(ctx)
	tmpl["VisibleURL"] = "/manage/" + discordgo.StrID(g.ID) + "/cah/"

	if !premium.ContextPremium(ctx) {
		return tmpl.AddAlerts(web.ErrorAlert("Custom packs are a premium feature")), nil
	}

	form := ctx.Value(common.ContextKeyParsedForm).(*PackForm)

	count, err := models.CahCustomPacks(models.CahCustomPackWhere.GuildID.EQ(g.ID)).CountG(ctx)
	if err != nil {
		return tmpl, err
	}
	if count >= MaxCustomPacks {
		return tmpl.AddAlerts(web.ErrorAlert(fmt.Sprintf("Servers can have at most %d custom packs", MaxCustomPacks))), nil
	}

	if taken, err := packNameTaken(ctx, g.ID, form.Name, 0); err != nil || taken {
		if taken {
			tmpl["PackDraft"] = form
			tmpl.AddAlerts(web.ErrorAlert("There's already a pack named " + form.Name))
		}
		return tmpl, err
	}

	pack := &models.CahCustomPack{
		GuildID:     g.ID,
		Name:        form.Name,
		Description: form.Description,
		Prompts:     nonNil(ParseCardLines(form.Prompts)),
		Responses:   nonNil(ParseCardLines(form.Responses)),
	}

	err = pack.InsertG(ctx, boil.Infer())
	if err != nil {
		tmpl["PackDraft"] = form
		return tmpl, err
	}

	tmpl["CurrentPackID"] = pack.ID
	tmpl["VisibleURL"] = packURL(g.ID, pack.ID)
	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(ctx, panelLogKeyAddedPack, &cplogs.Param{Type: cplogs.ParamTypeString, Value: pack.Name}))
	return tmpl, nil
}

func HandleUpdatePack(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	g, tmpl := web.GetBaseCPContextData(ctx)
	tmpl["VisibleURL"] = "/manage/" + discordgo.StrID(g.ID) + "/cah/"

	if !premium.ContextPremium(ctx) {
		return tmpl.AddAlerts(web.ErrorAlert("Custom packs are a premium feature")), nil
	}

	form := ctx.Value(common.ContextKeyParsedForm).(*PackForm)

	pack, err := findPack(r, g.ID)
	if err != nil {
		return tmpl.AddAlerts(web.ErrorAlert("Unknown pack")), err
	}
	tmpl["VisibleURL"] = packURL(g.ID, pack.ID)

	if taken, err := packNameTaken(ctx, g.ID, form.Name, pack.ID); err != nil || taken {
		if taken {
			tmpl["PackDraft"] = form
			tmpl.AddAlerts(web.ErrorAlert("There's already a pack named " + form.Name))
		}
		return tmpl, err
	}

	oldName := pack.Name
	pack.Name = form.Name
	pack.Description = form.Description
	pack.Prompts = nonNil(ParseCardLines(form.Prompts))
	pack.Responses = nonNil(ParseCardLines(form.Responses))

	_, err = pack.UpdateG(ctx, boil.Infer())
	if err != nil {
		tmpl["PackDraft"] = form
		return tmpl, err
	}

	if oldName != pack.Name {
		if err := renameDefaultPack(g.ID, oldName, pack.Name); err != nil {
			return tmpl, err
		}
	}

	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(ctx, panelLogKeyUpdatedPack, &cplogs.Param{Type: cplogs.ParamTypeString, Value: pack.Name}))
	return tmpl, nil
}

// deleting is allowed without premium so servers can clean up packs they can no longer use
func HandleDeletePack(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	g, tmpl := web.GetBaseCPContextData(ctx)
	tmpl["VisibleURL"] = "/manage/" + discordgo.StrID(g.ID) + "/cah/"

	pack, err := findPack(r, g.ID)
	if err != nil {
		return tmpl.AddAlerts(web.ErrorAlert("Unknown pack")), err
	}

	_, err = pack.DeleteG(ctx)
	if err != nil {
		return tmpl, err
	}

	tmpl["CurrentPackID"] = int64(0)
	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(ctx, panelLogKeyRemovedPack, &cplogs.Param{Type: cplogs.ParamTypeString, Value: pack.Name}))
	return tmpl, renameDefaultPack(g.ID, pack.Name, "")
}

// renameDefaultPack keeps the default packs in line with a renamed pack, an empty new name removes it from them
func renameDefaultPack(guildID int64, oldName, newName string) error {
	conf, err := FetchConfig(guildID)
	if err != nil || !slices.Contains(conf.DefaultPacks, oldName) {
		return err
	}

	defaults := make([]string, 0, len(conf.DefaultPacks))
	for _, v := range conf.DefaultPacks {
		switch {
		case v != oldName:
			defaults = append(defaults, v)
		case newName != "":
			defaults = append(defaults, newName)
		}
	}

	conf.DefaultPacks = defaults
	return SaveConfig(conf)
}

// builtinCards returns the prompts and responses of the pack in the format of the editor, one card per line
func builtinCards(pack *cardsagainstdiscord.CardPack) (prompts, responses string) {
	promptLines := make([]string, 0, len(pack.Prompts))
	for _, v := range pack.Prompts {
		promptLines = append(promptLines, LineFromPrompt(v.Prompt))
	}

	responseLines := make([]string, 0, len(pack.Responses))
	for _, v := range pack.Responses {
		if v == "%blank" {
			v = cardsagainstdiscord.BlankCard
		}
		responseLines = append(responseLines, string(v))
	}

	return strings.Join(promptLines, "\n"), strings.Join(responseLines, "\n")
}

func packURL(guildID, packID int64) string {
	return "/manage/" + discordgo.StrID(guildID) + "/cah/packs/" + discordgo.StrID(packID)
}

func findPack(r *http.Request, guildID int64) (*models.CahCustomPack, error) {
	id, _ := strconv.ParseInt(pat.Param(r, "id"), 10, 64)
	return models.CahCustomPacks(models.CahCustomPackWhere.GuildID.EQ(guildID), models.CahCustomPackWhere.ID.EQ(id)).OneG(r.Context())
}

func packNameTaken(ctx context.Context, guildID int64, name string, exceptID int64) (bool, error) {
	return models.CahCustomPacks(
		models.CahCustomPackWhere.GuildID.EQ(guildID),
		models.CahCustomPackWhere.ID.NEQ(exceptID),
		qm.Where("lower(name) = lower(?)", name),
	).ExistsG(ctx)
}

// the card columns are not null
func nonNil(cards []string) []string {
	if cards == nil {
		return []string{}
	}
	return cards
}
