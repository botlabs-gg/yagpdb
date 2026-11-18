package serverpreview

import (
	"bytes"
	"encoding/json"
	"html/template"
	"strings"
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/serverpreview/models"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/volatiletech/null/v8"
)

func TestValidateSlug(t *testing.T) {
	valid := []string{"mycoolserver", "abc", "gaming-hub-2", strings.Repeat("a", 32)}
	for _, slug := range valid {
		if err := ValidateSlug(slug); err != nil {
			t.Errorf("%q: unexpected error %v", slug, err)
		}
	}

	invalid := []string{"ab", strings.Repeat("a", 33), "-abc", "abc-", "Abc", "my server", "123456", "admin", "support", "yagpdb-fans", "notdiscord", "free-nitro"}
	for _, slug := range invalid {
		if ValidateSlug(slug) == nil {
			t.Errorf("%q: expected an error", slug)
		}
	}
}

func TestValidateCustomText(t *testing.T) {
	common.ConfHost.LoadedValue = "yagpdb.xyz"
	if err := ValidateCustomText("Welcome to our **community**! Share yagpdb.xyz/s/100 with friends", 100); err != nil {
		t.Errorf("unexpected error %v", err)
	}
	if ValidateCustomText(strings.Repeat("a", MaxCustomTextLength+1), 100) == nil {
		t.Error("expected text over the limit to be rejected")
	}
	if ValidateCustomText("click [here](https://evil.example.com)", 100) == nil {
		t.Error("expected masked links to be rejected")
	}
	if ValidateCustomText("also try yagpdb.xyz/s/200", 100) == nil {
		t.Error("expected a link to another server's preview to be rejected")
	}
	if ValidateCustomText("join us at https://discord.gg/abcdef", 100) == nil {
		t.Error("expected server invites to be rejected")
	}
}

func TestValidateButtons(t *testing.T) {
	common.ConfHost.LoadedValue = "yagpdb.xyz"
	cases := map[string][]Button{
		"too many":      {{"a", "https://a.example"}, {"b", "https://b.example"}, {"c", "https://c.example"}, {"d", "https://d.example"}},
		"no label":      {{"", "https://a.example"}},
		"long label":    {{strings.Repeat("a", MaxButtonLabelLength+1), "https://a.example"}},
		"not https":     {{"Site", "http://a.example"}},
		"not a link":    {{"Site", "a.example"}},
		"other scheme":  {{"Site", "javascript:alert(1)"}},
		"invite":        {{"Join", "https://discord.gg/abcdef"}},
		"other preview": {{"Join", "https://yagpdb.xyz/s/200"}},
		"long url":      {{"Site", "https://a.example/" + strings.Repeat("a", MaxButtonURLLength)}},
	}
	for name, buttons := range cases {
		if ValidateButtons(buttons, 100) == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestValidatePreviewSize(t *testing.T) {
	small := testSettings()
	if err := ValidatePreviewSize(small); err != nil {
		t.Errorf("unexpected error %v", err)
	}

	large := testSettings()
	large.CustomText = strings.Repeat("🔥", MaxCustomTextLength)
	longURL := "https://a.example/" + strings.Repeat("a", 400)
	large.Buttons, _ = json.Marshal([]Button{{"One", longURL}, {"Two", longURL}, {"Three", longURL}})
	if ValidatePreviewSize(large) == nil {
		t.Error("expected custom content that doesn't fit to be rejected")
	}
}

func testGuild() *discordgo.Guild {
	return &discordgo.Guild{
		ID:                       123,
		Name:                     "Cool *Server*",
		Description:              "A place to hang out",
		Icon:                     "iconhash",
		Banner:                   "bannerhash",
		ApproximateMemberCount:   1500,
		ApproximatePresenceCount: 300,
	}
}

func testSettings() *models.ServerPreview {
	buttons, _ := json.Marshal([]Button{{"Website", "https://example.com"}})
	return &models.ServerPreview{
		GuildID:     123,
		Enabled:     true,
		Slug:        null.StringFrom("coolserver"),
		AccentColor: 0xFF0000,
		HideFooter:  true,
		CustomText:  "Custom premium text",
		Buttons:     buttons,
	}
}

func encodePreview(t *testing.T, settings *models.ServerPreview, guild *discordgo.Guild, isPremium bool) string {
	t.Helper()
	preview := newPreview(settings, guild)
	preview.InviteURL = "https://discord.gg/invite"
	encoded, err := previewEmbed(settings, preview, isPremium).Encode()
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestPreviewEmbedFreeIgnoresPremiumSettings(t *testing.T) {
	got := encodePreview(t, testSettings(), testGuild(), false)

	for _, want := range []string{`Cool \\*Server\\*`, "A place to hang out", "**1,500** Members", "**300** Online", "Join Server", "Powered by", "iconhash", "bannerhash"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in %s", want, got)
		}
	}
	for _, unwanted := range []string{"Custom premium text", "Website", "16711680"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("expected premium content %q to be left out for free servers", unwanted)
		}
	}
}

func TestPreviewEmbedPremium(t *testing.T) {
	got := encodePreview(t, testSettings(), testGuild(), true)

	for _, want := range []string{"Custom premium text", "Website", "https://example.com", "bannerhash", "16711680", "Join Server"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in %s", want, got)
		}
	}
	if strings.Contains(got, "Powered by") {
		t.Error("expected the footer to be hidden")
	}
}

func TestPreviewEmbedWorstCaseFits(t *testing.T) {
	settings := testSettings()
	settings.HideFooter = false
	label := strings.Repeat("b", MaxButtonLabelLength)
	settings.Buttons, _ = json.Marshal([]Button{{label, "https://a.example/one"}, {label, "https://a.example/two"}})

	settings.CustomText = ""
	for n := 1; n <= MaxCustomTextLength; n++ {
		candidate := *settings
		candidate.CustomText = strings.Repeat("🔥", n)
		if ValidatePreviewSize(&candidate) != nil {
			break
		}
		settings.CustomText = candidate.CustomText
	}
	t.Logf("largest accepted custom text with two long buttons: %d emoji", len([]rune(settings.CustomText)))

	for _, ch := range []string{"*", "🔥", "a"} {
		guild := testGuild()
		guild.ID = 1<<63 - 1
		guild.Name = strings.Repeat(ch, 100)
		guild.Description = strings.Repeat(ch, 500)
		guild.ApproximateMemberCount = 1<<31 - 1
		guild.ApproximatePresenceCount = 1<<31 - 1

		got := encodePreview(t, settings, guild, true)
		if len(got) > MaxPreviewBytes {
			t.Errorf("%q server: preview is %d bytes, over the %d byte cap", ch, len(got), MaxPreviewBytes)
		}
	}
}

func TestPreviewEmbedCutsLongDescriptions(t *testing.T) {
	guild := testGuild()
	guild.Description = strings.Repeat("a", 200)

	got := encodePreview(t, testSettings(), guild, false)
	if strings.Contains(got, strings.Repeat("a", MaxDescriptionLength+1)) {
		t.Errorf("expected the description to be cut to %d characters", MaxDescriptionLength)
	}
}

func TestTemplatesParse(t *testing.T) {
	for name, contents := range map[string]string{"dashboard": PageHTML, "public": PublicPageHTML} {
		if _, err := web.Templates.New("test_" + name).Parse(contents); err != nil {
			t.Errorf("%s template: %v", name, err)
		}
	}
}

func TestPublicPageRender(t *testing.T) {
	tmpl, err := template.New("").Parse(PublicPageHTML)
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err = tmpl.ExecuteTemplate(&out, "serverpreview_public", map[string]any{
		"Preview": &Preview{Name: `Cool "Server" <b>`, IconURL: "https://cdn.discordapp.com/icons/1/a.png", BannerURL: "https://cdn.discordapp.com/banners/1/b.png", InviteURL: "https://discord.gg/abc", Path: "/s/coolserver"},
		"BaseURL": template.URL("https://yagpdb.xyz"),
		"BotName": "YAGPDB.xyz",
	})
	if err != nil {
		t.Fatal(err)
	}

	got := out.String()
	for _, want := range []string{
		`<link rel="discord:component-embed" type="application/json" href="https://yagpdb.xyz/s/coolserver/embed.json">`,
		`<meta property="og:url" content="https://yagpdb.xyz/s/coolserver" />`,
		`Join Cool &#34;Server&#34; &lt;b&gt; on Discord`,
		`href="https://discord.gg/abc"`,
		`src="https://cdn.discordapp.com/banners/1/b.png"`,
		`Powered by <a href="https://yagpdb.xyz">YAGPDB.xyz</a>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "<b>") {
		t.Error("expected the server name to be escaped")
	}
}

func TestPreviewEmbedOrderAndToggles(t *testing.T) {
	got := encodePreview(t, testSettings(), testGuild(), false)
	name, counts, description := strings.Index(got, "Cool"), strings.Index(got, "Online"), strings.Index(got, "A place to hang out")
	if !(name < counts && counts < description) {
		t.Errorf("expected the name, then member counts, then the description in %s", got)
	}

	settings := testSettings()
	settings.HideMemberCount = true
	settings.HideDescription = true
	got = encodePreview(t, settings, testGuild(), false)
	for _, unwanted := range []string{"Online", "Members", "A place to hang out"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("expected %q to be hidden in %s", unwanted, got)
		}
	}
}

func TestFormatCount(t *testing.T) {
	for n, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", 1234567: "1,234,567"} {
		if got := formatCount(n); got != want {
			t.Errorf("formatCount(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestIsLinkPreviewBot(t *testing.T) {
	for ua, want := range map[string]bool{
		"Mozilla/5.0 (compatible; Discordbot/2.0; +https://discordapp.com)": true,
		"Twitterbot/1.0":          true,
		"facebookexternalhit/1.1": true,
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/120": false,
		"": false,
	} {
		if got := isLinkPreviewBot(ua); got != want {
			t.Errorf("isLinkPreviewBot(%q) = %t, want %t", ua, got, want)
		}
	}
}

func TestContainsOtherServerPreviewLink(t *testing.T) {
	common.ConfHost.LoadedValue = "yagpdb.xyz"

	for text, want := range map[string]bool{
		"join us https://yagpdb.xyz/s/100":             false,
		"join us https://yagpdb.xyz/s/200":             true,
		"join us yagpdb.xyz/s/100 or yagpdb.xyz/s/200": true,
		"join us https://YAGPDB.xyz//s//200":           true,
		"see https://yagpdb.xyz/manage/200":            false,
		"no links here":                                false,
	} {
		if got := ContainsOtherServerPreviewLink(text, 100); got != want {
			t.Errorf("ContainsOtherServerPreviewLink(%q) = %t, want %t", text, got, want)
		}
	}
}
