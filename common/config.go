package common

import (
	"github.com/jonas747/yagpdb/common/config"
	"github.com/pkg/errors"
	"strings"
)

var (
	ConfOwner = config.RegisterOption("yagpdb.owner", "ID of the owner of the bot", nil)

	ConfClientID     = config.RegisterOption("yagpdb.clientid", "Client ID of the discord application", nil)
	ConfClientSecret = config.RegisterOption("yagpdb.clientsecret", "Client Secret of the discord application", nil)
	ConfBotToken     = config.RegisterOption("yagpdb.bottoken", "Token of the bot user", nil)
	ConfHost         = config.RegisterOption("yagpdb.host", "Host without the protocol, example: example.com, used by the webserver", nil)
	ConfEmail        = config.RegisterOption("yagpdb.email", "Email used when fetching lets encrypt certificate", "")

	ConfPQHost     = config.RegisterOption("yagpdb.pqhost", "Postgres host", "localhost:6379")
	ConfPQUsername = config.RegisterOption("yagpdb.pqusername", "Postgres user", "postgres")
	ConfPQPassword = config.RegisterOption("yagpdb.pqpassword", "Postgres passoword", "")
	ConfPQDB       = config.RegisterOption("yagpdb.pqdb", "Postgres database", "yagpdb")
	ConfRedis      = config.RegisterOption("yagpdb.redis", "Redis address", "localhost:6379")

	ConfDogStatsdAddress = config.RegisterOption("yagpdb.dogstatsdaddress", "dogstatsd address", "")
)

func LoadConfig() (err error) {
	config.AddSource(&config.EnvSource{})
	config.Load()

	requiredConf := []*config.ConfigOption{
		ConfOwner,
		ConfClientID,
		ConfClientSecret,
		ConfBotToken,
		ConfHost,
	}

	for _, v := range requiredConf {
		if v.LoadedValue == nil {
			envFormat := strings.ToUpper(strings.Replace(v.Name, ".", "_", -1))
			return errors.Errorf("Did not set required config option: %q (%s as env var)", v.Name, envFormat)
		}
	}

	return nil
}
