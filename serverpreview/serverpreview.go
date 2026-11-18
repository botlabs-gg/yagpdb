package serverpreview

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/pubsub"
	"github.com/botlabs-gg/yagpdb/v2/serverpreview/models"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

type Plugin struct{}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Server Preview",
		SysName:  "server_preview",
		Category: common.PluginCategoryMisc,
	}
}

var (
	logger = common.GetPluginLogger(&Plugin{})

	cachedSettings = common.CacheSet.RegisterSlot("server_preview_settings", fetchSettings, int64(0))
	cachedSlugs    = common.CacheSet.RegisterSlot("server_preview_slugs", fetchSlugGuildID, "")
)

func RegisterPlugin() {
	common.InitSchemas("serverpreview", DBSchemas...)
	common.RegisterPlugin(&Plugin{})
}

type Button struct {
	Label string
	URL   string
}

func ParseButtons(raw []byte) []Button {
	var buttons []Button
	if err := json.Unmarshal(raw, &buttons); err != nil {
		return nil
	}
	return buttons
}

func fetchSettings(key any) (any, error) {
	guildID := key.(int64)
	settings, err := models.FindServerPreviewG(context.Background(), guildID)
	if errors.Is(err, sql.ErrNoRows) {
		return &models.ServerPreview{GuildID: guildID, Buttons: []byte("[]")}, nil
	}
	return settings, err
}

func fetchSlugGuildID(key any) (any, error) {
	settings, err := models.ServerPreviews(qm.Where("slug = ?", key.(string))).OneG(context.Background())
	if errors.Is(err, sql.ErrNoRows) {
		return int64(0), nil
	}
	if err != nil {
		return nil, err
	}
	return settings.GuildID, nil
}

func GetSettings(guildID int64) (*models.ServerPreview, error) {
	v, err := cachedSettings.Get(guildID)
	if err != nil {
		return nil, err
	}
	return v.(*models.ServerPreview), nil
}

// GuildIDForKey returns the server a preview link key points to, or 0 when it points to none.
func GuildIDForKey(key string) (int64, error) {
	guildID, _, err := resolveGuildID(key)
	return guildID, err
}

// ContainsOtherServerPreviewLink reports whether s links to the server preview of a server other than guildID.
// It fails closed, so a link that can't be looked up counts as one.
func ContainsOtherServerPreviewLink(s string, guildID int64) bool {
	for _, key := range common.ServerPreviewLinkKeys(s) {
		linkedGuildID, err := GuildIDForKey(key)
		if err != nil {
			logger.WithError(err).WithField("guild", guildID).Error("failed looking up server preview link")
			return true
		}
		if linkedGuildID != 0 && linkedGuildID != guildID {
			return true
		}
	}
	return false
}

// resolveGuildID turns a preview key, either a server ID or a premium slug, into a server ID.
func resolveGuildID(key string) (guildID int64, viaSlug bool, err error) {
	if id, err := strconv.ParseInt(key, 10, 64); err == nil {
		return id, false, nil
	}

	if ValidateSlug(key) != nil {
		return 0, true, nil
	}

	v, err := cachedSlugs.Get(key)
	if err != nil {
		return 0, true, err
	}
	return v.(int64), true, nil
}

func evictSettings(guildID int64, slugs ...null.String) {
	pubsub.EvictCacheSet(cachedSettings, guildID)
	for _, slug := range slugs {
		if slug.Valid {
			pubsub.EvictCacheSet(cachedSlugs, slug.String)
		}
	}
}

// OnRemovedPremiumGuild clears the premium customization and frees the slug.
func (p *Plugin) OnRemovedPremiumGuild(guildID int64) error {
	settings, err := models.FindServerPreviewG(context.Background(), guildID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}

	oldSlug := settings.Slug
	settings.Slug = null.String{}
	settings.AccentColor = 0
	settings.HideFooter = false
	settings.CustomText = ""
	settings.Buttons = []byte("[]")

	_, err = settings.UpdateG(context.Background(), boil.Infer())
	evictSettings(guildID, oldSlug)
	return err
}

func SetSuspended(guildID int64, suspended bool) error {
	settings, err := models.FindServerPreviewG(context.Background(), guildID)
	if errors.Is(err, sql.ErrNoRows) {
		settings = &models.ServerPreview{GuildID: guildID, Buttons: []byte("[]")}
	} else if err != nil {
		return err
	}

	settings.Suspended = suspended
	err = settings.UpsertG(context.Background(), true, []string{"guild_id"}, boil.Whitelist("suspended", "updated_at"), boil.Infer())
	evictSettings(guildID, settings.Slug)
	return err
}
