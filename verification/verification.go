package verification

//go:generate sqlboiler --no-hooks psql

import (
	"github.com/jonas747/yagpdb/common"
	"os"
)

var GoogleReCAPTCHASiteKey = os.Getenv("YAGPDB_GOOGLE_RECAPTCHA_SITE_KEY")
var GoogleReCAPTCHASecret = os.Getenv("YAGPDB_GOOGLE_RECAPTCHA_SECRET")

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

	if GoogleReCAPTCHASecret == "" || GoogleReCAPTCHASiteKey == "" {
		logger.Warn("no YAGPDB_GOOGLE_RECAPTCHA_SECRET and/or YAGPDB_GOOGLE_RECAPTCHA_SITE_KEY provided, not enabling verification plugin")
		return
	}

	common.InitSchema(DBSchema, "verification")

	common.RegisterPlugin(&Plugin{})
}

const (
	DefaultPageContent = `## Verification

Please solve the following reCAPTCHA to make sure you're not a robot`
)

const DefaultDMMessage = `{{sendMessage nil (cembed
"title" "Are you a bot?"
"description" (printf "Please solve the CAPTCHA at this link to make sure you're human, before you can enter %s: %s" .Server.Name .Link)
)}}`
