package common

//go:generate sqlboiler --no-hooks psql

import (
	"database/sql"
	"fmt"
	stdlog "log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/jonas747/discordgo"
	"github.com/mediocregopher/radix/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/boil"
)

var (
	VERSION = "unknown"

	GORM *gorm.DB
	PQ   *sql.DB

	RedisPool *radix.Pool

	BotSession *discordgo.Session
	BotUser    *discordgo.User

	RedisPoolSize = 0

	Testing = os.Getenv("YAGPDB_TESTING") != ""

	CurrentRunCounter int64

	NodeID string

	// if your compile failed at this line, you're likely not compiling for 64bit, which is unsupported.
	_ interface{} = ensure64bit

	logger = GetFixedPrefixLogger("common")
)

// CoreInit initializes the essential parts
func CoreInit() error {

	rand.Seed(time.Now().UnixNano())

	stdlog.SetOutput(&STDLogProxy{})
	stdlog.SetFlags(0)

	if Testing {
		logrus.SetLevel(logrus.DebugLevel)
	}

	err := connectRedis(false)
	if err != nil {
		return err
	}

	err = LoadConfig()
	if err != nil {
		return err
	}

	return nil
}

// Init initializes the rest of the bot
func Init() error {

	err := setupGlobalDGoSession()
	if err != nil {
		return err
	}

	db := "yagpdb"
	if ConfPQDB.GetString() != "" {
		db = ConfPQDB.GetString()
	}

	err = connectDB(ConfPQHost.GetString(), ConfPQUsername.GetString(), ConfPQPassword.GetString(), db, confMaxSQLConns.GetInt())
	if err != nil {
		panic(err)
	}

	logger.Info("Retrieving bot info....")
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

	logger.Info("Initializing core schema")
	InitSchemas("core_configs", CoreServerConfDBSchema)

	return err
}

func GetBotToken() string {
	token := ConfBotToken.GetString()
	if !strings.HasPrefix(token, "Bot ") {
		token = "Bot " + token
	}
	return token
}

func setupGlobalDGoSession() (err error) {

	BotSession, err = discordgo.New(GetBotToken())
	if err != nil {
		return err
	}

	maxCCReqs := ConfMaxCCR.GetInt()
	if maxCCReqs < 1 {
		maxCCReqs = 25
	}

	logger.Info("max ccr set to: ", maxCCReqs)

	BotSession.MaxRestRetries = 10
	BotSession.Ratelimiter.MaxConcurrentRequests = maxCCReqs

	innerTransport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 10 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConnsPerHost:   maxCCReqs,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	if ConfDisableKeepalives.GetBool() {
		innerTransport.DisableKeepAlives = true
		logger.Info("Keep alive connections to REST api for discord is disabled, may cause overhead")
	}

	BotSession.Client.Transport = &LoggingTransport{Inner: innerTransport}

	return nil
}

func InitTest() {
	testDB := os.Getenv("YAGPDB_TEST_DB")
	if testDB == "" {
		return
	}

	err := connectDB("localhost", "postgres", "123", testDB, 3)
	if err != nil {
		panic(err)
	}
}

var (
	metricsRedisReconnects = promauto.NewCounter(prometheus.CounterOpts{
		Name: "yagpdb_redis_reconnects_total",
		Help: "Number of reconnects to the redis server",
	})
	metricsRedisRetries = promauto.NewCounter(prometheus.CounterOpts{
		Name: "yagpdb_redis_retries_total",
		Help: "Number of retries on redis commands",
	})
)

func connectRedis(unitTests bool) (err error) {
	maxConns := RedisPoolSize
	if maxConns == 0 {
		maxConns, _ = strconv.Atoi(os.Getenv("YAGPDB_REDIS_POOL_SIZE"))
		if maxConns == 0 {
			maxConns = 10
		}
	}

	logger.Infof("Set redis pool size to %d", maxConns)

	// we kinda bypass the config system because the config system also relies on redis
	// this way the only required env var is the redis address, and per-host specific things
	addr := os.Getenv("YAGPDB_REDIS")
	if addr == "" {
		addr = "localhost:6379"
	}

	opts := []radix.PoolOpt{
		radix.PoolOnEmptyWait(),
		radix.PoolOnFullClose(),
		radix.PoolPipelineWindow(0, 0),
	}

	// if were running unit tests, use the 2nd db to avoid accidentally running tests against a main db
	if unitTests {
		radix.PoolConnFunc(func(network, addr string) (radix.Conn, error) {
			return radix.Dial(network, addr, radix.DialSelectDB(2))
		})
	}

	RedisPool, err = radix.NewPool("tcp", addr, maxConns, opts...)
	return
}

// InitTestRedis sets common.RedisPool to a redis pool for unit testing
func InitTestRedis() error {
	if RedisPool != nil {
		return nil
	}

	err := connectRedis(true)
	return err
}

func connectDB(host, user, pass, dbName string, maxConns int) error {
	if host == "" {
		host = "localhost"
	}

	passwordPart := ""
	if pass != "" {
		passwordPart = " password='" + pass + "'"
	}

	db, err := gorm.Open("postgres", fmt.Sprintf("host=%s user=%s dbname=%s sslmode=disable%s", host, user, dbName, passwordPart))
	GORM = db
	PQ = db.DB()
	boil.SetDB(PQ)
	if err == nil {
		PQ.SetMaxOpenConns(maxConns)
		PQ.SetMaxIdleConns(maxConns)
		logger.Infof("Set max PG connections to %d", maxConns)
	}
	GORM.SetLogger(&GORMLogger{})

	return err
}

var (
	shutdownFunc   func()
	shutdownCalled bool
	shutdownMU     sync.Mutex
)

func Shutdown() {
	shutdownMU.Lock()
	f := shutdownFunc
	if f == nil || shutdownCalled {
		shutdownMU.Unlock()
		return
	}

	shutdownCalled = true
	shutdownMU.Unlock()

	if f != nil {
		f()
	}
}

func SetShutdownFunc(f func()) {
	shutdownMU.Lock()
	shutdownFunc = f
	shutdownMU.Unlock()
}
