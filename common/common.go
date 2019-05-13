package common

//go:generate sqlboiler --no-hooks psql

import (
	"database/sql"
	"fmt"
	"github.com/DataDog/datadog-go/statsd"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/retryableredis"
	"github.com/jonas747/yagpdb/common/basicredispool"
	"github.com/mediocregopher/radix"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/boil"
	stdlog "log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	VERSIONMAJOR = 1
	VERSIONMINOR = 18
	VERSIONPATCH = 0
)

var (
	VERSIONNUMBER = fmt.Sprintf("%d.%d.%d", VERSIONMAJOR, VERSIONMINOR, VERSIONPATCH)
	VERSION       = VERSIONNUMBER

	GORM *gorm.DB
	PQ   *sql.DB

	RedisPool *basicredispool.Pool

	BotSession *discordgo.Session
	BotUser    *discordgo.User
	Conf       *CoreConfig

	RedisPoolSize = 25

	Statsd *statsd.Client

	Testing = os.Getenv("YAGPDB_TESTING") != ""

	CurrentRunCounter int64

	NodeID string

	// if your compile failed at this line, you're likely not compiling for 64bit, which is unsupported.
	_ interface{} = ensure64bit

	logger = GetFixedPrefixLogger("common")
)

// Initalizes all database connections, config loading and so on
func Init() error {
	rand.Seed(time.Now().UnixNano())

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

	err = setupGlobalDGoSession()
	if err != nil {
		return err
	}

	ConnectDatadog()

	err = connectRedis(config.Redis)
	if err != nil {
		return err
	}

	db := "yagpdb"
	if config.PQDB != "" {
		db = config.PQDB
	}

	err = connectDB(config.PQHost, config.PQUsername, config.PQPassword, db)
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

	InitSchema(CoreServerConfDBSchema, "core_configs")

	return err
}

func setupGlobalDGoSession() (err error) {
	BotSession, err = discordgo.New(Conf.BotToken)
	if err != nil {
		return err
	}

	maxCCReqs, _ := strconv.Atoi(os.Getenv("YAGPDB_MAX_CCR"))
	if maxCCReqs < 1 {
		maxCCReqs = 25
	}

	logger.Info("max ccr set to: ", maxCCReqs)

	BotSession.MaxRestRetries = 5
	BotSession.Ratelimiter.MaxConcurrentRequests = maxCCReqs

	innerTransport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 5 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	if os.Getenv("YAGPDB_DISABLE_KEEPALIVES") != "" {
		innerTransport.DisableKeepAlives = true
		logger.Info("Keep alive connections to REST api for discord is disabled, may cause overhead")
	}

	BotSession.Client.HTTPClient.Transport = &LoggingTransport{Inner: innerTransport}

	return nil
}

func ConnectDatadog() {
	if Conf.DogStatsdAddress == "" {
		logger.Warn("No datadog info provided, not connecting to datadog aggregator")
		return
	}

	client, err := statsd.New(Conf.DogStatsdAddress)
	if err != nil {
		logger.WithError(err).Error("Failed connecting to dogstatsd, datadog integration disabled")
		return
	}

	if NodeID != "" {
		client.Tags = append(client.Tags, "node:"+NodeID)
	}

	Statsd = client

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
	RedisPool, err = basicredispool.NewPool(RedisPoolSize, &retryableredis.DialConfig{
		Network: "tcp",
		Addr:    addr,
		OnReconnect: func(err error) {
			if err == nil {
				return
			}

			logrus.WithError(err).Warn("[core] redis reconnect triggered")
			if Statsd != nil {
				Statsd.Incr("yagpdb.redis.reconnects", nil, 1)
			}
		},
		OnRetry: func(err error) {
			logrus.WithError(err).Warn("[core] redis retrying failed action")
			if Statsd != nil {
				Statsd.Incr("yagpdb.redis.retries", nil, 1)
			}
		},
	})

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

func InitSchema(schema string, name string) {
	_, err := PQ.Exec(schema)
	if err != nil {
		logger.WithError(err).Fatal("failed initializing postgres db schema for ", name)
	}

	return
}
