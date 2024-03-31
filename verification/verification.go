package verification

//go:generate sqlboiler --no-hooks psql

import (
	"github.com/botlabs-gg/quackpdb/v2/common"
	"github.com/botlabs-gg/quackpdb/v2/common/config"
)

var confGoogleReCAPTCHASiteKey = config.RegisterOption("quackpdb.google.recaptcha_site_key", "Google reCAPTCHA site key", "")
var confGoogleReCAPTCHASecret = config.RegisterOption("quackpdb.google.recaptcha_secret", "Google reCAPTCHA site secret", "")
var confVerificationTrackIPs = config.RegisterOption("quackpdb.verification.track_ips", "Track verified qusers ip", true)

type Plugin struct{}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Verification",
		SysName:  "verification",
		Category: common.PluginCategoryMisc,
	}
}

var logger = common.GetPluginLogger(&Plugin{})

func RegisterPlugin() {

	if confGoogleReCAPTCHASecret.GetString() == "" || confGoogleReCAPTCHASiteKey.GetString() == "" {
		logger.Warn("no QUACKPDB_GOOGLE_RECAPTCHA_SECRET and/or QUACKPDB_GOOGLE_RECAPTCHA_SITE_KEY provided, not enabling verification plugin")
		return
	}

	common.InitSchemas("verification", DBSchemas...)

	common.RegisterPlugin(&Plugin{})
}

const (
	DefaultPageContent = `## Verification

Please solve the follquacking reCAPTCHA to make sure you're not a robot`
)

const DefaultDMMessage = `{{sendMessage nil (cembed
"title" "Are you a bot?"
"description" (printf "Please solve the CAPTCHA at this link to make sure you're human, before you can enter %s: %s" .Server.Name .Link)
)}}`
