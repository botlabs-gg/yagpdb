package verification

//go:generate sqlboiler --no-hooks psql

import (
	"github.com/jonas747/yagpdb/common"
	"github.com/sirupsen/logrus"
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

func RegisterPlugin() {

	if GoogleReCAPTCHASecret == "" || GoogleReCAPTCHASiteKey == "" {
		logrus.Warn("[verification] no YAGPDB_GOOGLE_RECAPTCHA_SECRET and/or YAGPDB_GOOGLE_RECAPTCHA_SITE_KEY provided, not enabling verification plugin")
		return
	}

	common.InitSchema(DBSchema, "verification")

	common.RegisterPlugin(&Plugin{})
}

const (
	DefaultPageContent = `## Verification

Please solve the following reCAPTCHA to make sure you're not a robot`
)
