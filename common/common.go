package common

import (
	"database/sql"
	"fmt"
	"github.com/DataDog/datadog-go/statsd"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/jonas747/discordgo"
	"github.com/mediocregopher/radix.v3"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/boil"
	stdlog "log"
	"os"
)

const (
	VERSIONMAJOR = 1
	VERSIONMINOR = 8
	VERSIONPATCH = 0
)

var (
	VERSIONNUMBER = fmt.Sprintf("%d.%d.%d", VERSIONMAJOR, VERSIONMINOR, VERSIONPATCH)
	VERSION       = VERSIONNUMBER + " Is cereal soup?"

	GORM *gorm.DB
	PQ   *sql.DB

	RedisPool *radix.Pool

	BotSession *discordgo.Session
	BotUser    *discordgo.User
	Conf       *CoreConfig

	RedisPoolSize = 25

	Statsd *statsd.Client

	Testing = os.Getenv("YAGPDB_TESTING") != ""

	CurrentRunCounter int64
)

// Initalizes all database connections, config loading and so on
func Init() error {
	stdlog.SetOutput(&STDLogProxy{})
	stdlog.SetFlags(0)

	if Testing {
		logrus.SetLevel(logrus.DebugLevel)
	}

	config, err := LoadConfig()
	if err != nil {
		return err
	}
	Conf = config

	BotSession, err = discordgo.New(config.BotToken)
	if err != nil {
		return err
	}
	BotSession.MaxRestRetries = 3

	ConnectDatadog()

	err = connectRedis(config.Redis)
	if err != nil {
		return err
	}

	err = connectDB(config.PQHost, config.PQUsername, config.PQPassword, "yagpdb")
	if err != nil {
		panic(err)
	}

	BotUser, err = BotSession.UserMe()
	if err != nil {
		panic(err)
	}
	BotSession.State.User = &discordgo.SelfUser{
		User: BotUser,
	}

	err = RedisPool.Do(radix.Cmd(&CurrentRunCounter, "INCR", "yagpdb_run_counter"))
	if err != nil {
		panic(err)
	}

	return err
}

func ConnectDatadog() {
	if Conf.DogStatsdAddress == "" {
		logrus.Warn("No datadog info provided, not connecting to datadog aggregator")
		return
	}

	client, err := statsd.New(Conf.DogStatsdAddress)
	if err != nil {
		logrus.WithError(err).Error("Failed connecting to dogstatsd, datadog integration disabled")
		return
	}

	Statsd = client

	currentTransport := BotSession.Client.HTTPClient.Transport
	BotSession.Client.HTTPClient.Transport = &LoggingTransport{Inner: currentTransport}
}

func InitTest() {
	testDB := os.Getenv("YAGPDB_TEST_DB")
	if testDB == "" {
		return
	}

	err := connectDB("localhost", "postgres", "123", testDB)
	if err != nil {
		panic(err)
	}
}

func connectRedis(addr string) (err error) {
	// RedisPool, err = pool.NewCustom("tcp", addr, 25, redis.)
	// if os.Getenv("YAGPDB_LEGACY_REDIS_POOL") != "" {
	// 	logrus.Info("Using legacy redis pool")
	// 	RedisPool, err = pool.NewCustom("tcp", addr, RedisPoolSize, RedisDialFunc)
	// } else {
	// 	logrus.Info("Using new redis pool, set YAGPDB_LEGACY_REDIS_POOL=yes if it's broken")
	// 	RedisPool, err = fixedpool.NewCustom("tcp", addr, RedisPoolSize, redis.Dial)
	// }

	RedisPool, err = radix.NewPool("tcp", addr, RedisPoolSize, radix.PoolOnEmptyWait())
	if err != nil {
		logrus.WithError(err).Fatal("Failed intitializing redis pool")
	}

	return
}

func connectDB(host, user, pass, dbName string) error {
	if host == "" {
		host = "localhost"
	}

	db, err := gorm.Open("postgres", fmt.Sprintf("host=%s user=%s dbname=%s sslmode=disable password='%s'", host, user, dbName, pass))
	GORM = db
	PQ = db.DB()
	boil.SetDB(PQ)
	if err == nil {
		PQ.SetMaxOpenConns(5)
	}
	GORM.SetLogger(&GORMLogger{})

	return err
}
