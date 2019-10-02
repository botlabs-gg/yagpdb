package mqueue

import (
	"container/list"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jonas747/yagpdb/common/config"

	"emperror.dev/errors"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/retryableredis"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix/v3"
	"github.com/sirupsen/logrus"
)

var (
	sources  = make(map[string]PluginWithErrorHandler)
	stopChan = make(chan *sync.WaitGroup)

	currentlyProcessing     = make([]int64, 0)
	currentlyProcessingLock sync.RWMutex

	startedLock sync.Mutex
	started     bool

	numWorkers = new(int32)

	webhookSession *discordgo.Session

	logger = common.GetPluginLogger(&Plugin{})

	confMaxWorkers = config.RegisterOption("yagpdb.mqueue.max_workers", "Max mqueue sending workers", 2)
)

type PluginWithErrorHandler interface {
	DisableFeed(elem *QueuedElement, err error)
}

type PluginWithWebhookAvatar interface {
	WebhookAvatar() string
}

var (
	_ bot.LateBotInitHandler = (*Plugin)(nil)
	_ bot.BotStopperHandler  = (*Plugin)(nil)
)

type Plugin struct {
}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "mqueue",
		SysName:  "mqueue",
		Category: common.PluginCategoryCore,
	}
}

func RegisterPlugin() {

	var err error
	webhookSession, err = discordgo.New()
	if err != nil {
		logger.WithError(err).Error("failed initiializing webhook session")
	}
	webhookSession.AddHandler(handleWebhookSessionRatelimit)

	innerTransport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 10 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          10,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConnsPerHost:   10,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	webhookSession.Client.Transport = innerTransport

	_, err = common.PQ.Exec(DBSchema)
	if err != nil {
		logrus.WithError(err).Error("[mqueue] failed initiializing db schema")
	}

	p := &Plugin{}
	common.RegisterPlugin(p)
}

func RegisterSource(name string, source PluginWithErrorHandler) {
	sources[name] = source
}

func IncrIDCounter() (next int64) {
	err := common.RedisPool.Do(retryableredis.Cmd(&next, "INCR", "mqueue_id_counter"))
	if err != nil {
		logger.WithError(err).Error("Failed increasing mqueue id counter")
		return -1
	}

	return next
}

func QueueMessageString(source, sourceID string, guildID, channel int64, message string) {
	QueueMessage(source, sourceID, guildID, channel, message, nil)
}

func QueueMessageEmbed(source, sourceID string, guildID, channel int64, embed *discordgo.MessageEmbed) {
	QueueMessage(source, sourceID, guildID, channel, "", embed)
}

func QueueMessage(source, sourceID string, guildID, channel int64, msgStr string, embed *discordgo.MessageEmbed) {
	QueueMessageWebhook(source, sourceID, guildID, channel, msgStr, embed, false, "")
}

func QueueMessageWebhook(source, sourceID string, guildID, channel int64, msgStr string, embed *discordgo.MessageEmbed, webhook bool, webhookUsername string) {
	nextID := IncrIDCounter()
	if nextID == -1 {
		return
	}

	elem := &QueuedElement{
		ID:              nextID,
		Source:          source,
		SourceID:        sourceID,
		Channel:         channel,
		MessageStr:      msgStr,
		MessageEmbed:    embed,
		Guild:           guildID,
		UseWebhook:      webhook,
		WebhookUsername: webhookUsername,
	}

	serialized, err := json.Marshal(elem)
	if err != nil {
		logger.WithError(err).Error("Failed marshaling mqueue element")
		return
	}

	err = common.RedisPool.Do(retryableredis.Cmd(nil, "ZADD", "mqueue", "-1", string(serialized)))
	if err != nil {
		return
	}
}

func (p *Plugin) LateBotInit() {
	go startPolling()
	go processWorker()
	go workerScaler()
}

func (p *Plugin) StopBot(wg *sync.WaitGroup) {
	startedLock.Lock()
	if !started {
		startedLock.Unlock()
		wg.Done()
		return
	}
	startedLock.Unlock()
	stopChan <- wg
}

func workerScaler() {
	lastWorkerSpawnedAt := time.Now()
	t := time.NewTicker(time.Second * 10)

	deltaHistory := list.New()
	sizeHistory := list.New()

	lastSize := 0
	for {
		<-t.C

		workmu.Lock()
		current := len(workSlice)
		workmu.Unlock()

		delta := current - lastSize
		lastSize = current
		deltaHistory.PushBack(delta)
		sizeHistory.PushBack(current)

		if deltaHistory.Len() > 6*5 { // keep 5 minute average
			deltaHistory.Remove(deltaHistory.Front())
			sizeHistory.Remove(sizeHistory.Front())
		}

		// see if we should launch a worker
		if current < 100 || time.Since(lastWorkerSpawnedAt) < time.Minute*10 || deltaHistory.Len() < 6 {
			// don't bother launching workers when below 100, and atleast have a minute of averages
			continue
		}

		// calculate average to see if it increased or decreased
		deltaAverage := calcListAverage(deltaHistory)
		sizeAverage := calcListAverage(sizeHistory)

		if deltaAverage > 1 && sizeAverage > 1000 {
			logger.Info("Launched new mqueue worker, total workers: ", atomic.LoadInt32(numWorkers)+1)
			go processWorker()
			lastWorkerSpawnedAt = time.Now()
		}

		nw := atomic.LoadInt32(numWorkers)
		if int(nw) >= confMaxWorkers.GetInt() {
			return
		}
	}
}

func calcListAverage(in *list.List) int {
	total := 0
	for elem := in.Front(); elem != nil; elem = elem.Next() {
		total += elem.Value.(int)
	}

	average := total / in.Len()
	return average
}

func startPolling() {
	startedLock.Lock()
	if started {
		startedLock.Unlock()
		panic("Already started mqueue")
	}
	started = true
	startedLock.Unlock()

	first := true

	ticker := time.NewTicker(time.Second * 5)
	for {
		select {
		case wg := <-stopChan:
			shutdown(wg)
			return
		case <-ticker.C:
			pollRedis(first)
			first = false
			if common.Statsd != nil {
				workmu.Lock()
				l := len(workSlice)
				workmu.Unlock()

				common.Statsd.Gauge("yagpdb.mqueue.size", float64(l), nil, 1)
				common.Statsd.Gauge("yagpdb.mqueue.numworkers", float64(atomic.LoadInt32(numWorkers)), nil, 1)
			}
		}
	}
}

func shutdown(wg *sync.WaitGroup) {
	for i := 0; i < 10; i++ {
		currentlyProcessingLock.RLock()
		num := len(currentlyProcessing)
		currentlyProcessingLock.RUnlock()
		if num < 1 {
			break
		}
		time.Sleep(time.Second)
	}
	wg.Done()
}

func pollRedis(first bool) {
	var results [][]byte

	// Get all elements that are currently not being processed, we set thhem to processing by setting their score to our run counter, which increases every time the bot starts
	max := "0"
	if first {
		max = strconv.FormatInt(common.CurrentRunCounter, 10)
	}

	err := common.RedisPool.Do(retryableredis.Cmd(&results, "ZRANGEBYSCORE", "mqueue", "-1", "("+max))
	if err != nil {
		logger.WithError(err).Error("Failed polling redis mqueue")
		return
	}

	if len(results) < 1 {
		return
	}

	common.RedisPool.Do(radix.WithConn("mqueue", func(rc radix.Conn) error {
		workmu.Lock()
		defer workmu.Unlock()

		for _, elem := range results {

			var parsed *QueuedElement
			err := json.Unmarshal(elem, &parsed)
			if err != nil {
				logger.WithError(err).Error("Failed parsing mqueue redis elemtn")
				continue
			}

			if !bot.IsGuildOnCurrentProcess(parsed.Guild) {
				continue
			}

			// Mark it as being processed so it wont get caught in further polling, unless its a new process in which case it wasnt completed
			rc.Do(retryableredis.FlatCmd(nil, "ZADD", "mqueue", common.CurrentRunCounter, string(elem)))

			workSlice = append(workSlice, &WorkItem{
				elem: parsed,
				raw:  elem,
			})

		}

		return nil
	}))
}

type WorkItem struct {
	elem *QueuedElement
	raw  []byte
}

var (
	workSlice  []*WorkItem
	activeWork []*WorkItem
	workmu     sync.Mutex
)

func findWork() int {
	// find a work item that does not share a channel with any other item being processed (so ratelimits only take up max 1 worker)
OUTER:
	for i, v := range workSlice {
		for _, active := range activeWork {
			if active.elem.Channel == v.elem.Channel {
				continue OUTER
			}
		}

		// Skip channels we have already skipped over
		for j, w := range workSlice {
			if j >= i {
				break
			}

			if w.elem.Channel == v.elem.Channel {
				continue OUTER
			}
		}

		b := common.BotSession.Ratelimiter.GetBucket(discordgo.EndpointChannelMessages(v.elem.Channel))
		b.Lock()
		waitTime := common.BotSession.Ratelimiter.GetWaitTime(b, 1)
		b.Unlock()
		if waitTime > time.Millisecond*250 {
			continue
		}

		return i
	}

	return -1
}

func processWorker() {
	atomic.AddInt32(numWorkers, 1)
	defer atomic.AddInt32(numWorkers, -1)

	var currentItem *WorkItem
	for {
		workmu.Lock()

		// find a work item that does not share a channel with any other item being processed (so ratelimits only take up max 1 worker)
		workItemIndex := findWork()

		// did not find any
		if workItemIndex == -1 {
			workmu.Unlock()
			time.Sleep(time.Second * 1)
			continue
		}

		currentItem = workSlice[workItemIndex]
		activeWork = append(activeWork, currentItem)
		workSlice = append(workSlice[:workItemIndex], workSlice[workItemIndex+1:]...)
		workmu.Unlock()

		process(currentItem.elem, currentItem.raw)

		workmu.Lock()

		// if were done processing a item previously then remove it from the active processing slice
		for i, v := range activeWork {
			if v == currentItem {
				activeWork = append(activeWork[:i], activeWork[i+1:]...)
				break
			}
		}
		currentItem = nil

		l := len(workSlice)
		workmu.Unlock()

		// we sleep between each element as to avoid global ratelimits
		// this amount will atuomatically scale with how many elements are in the queue, the max sleep is 1 second
		var msSleep int
		if l > 5 {
			msSleep = 5000 / l
		} else {
			msSleep = 1000
		}
		time.Sleep(time.Millisecond * time.Duration(msSleep))
	}
}

func process(elem *QueuedElement, raw []byte) {
	id := elem.ID

	queueLogger := logger.WithField("mq_id", id)

	defer func() {
		common.RedisPool.Do(retryableredis.Cmd(nil, "ZREM", "mqueue", string(raw)))
	}()

	for {
		var err error
		if elem.UseWebhook {
			err = trySendWebhook(queueLogger, elem)
		} else {
			err = trySendNormal(queueLogger, elem)
		}
		if err == nil {
			break
		}

		if e, ok := errors.Cause(err).(*discordgo.RESTError); ok {
			if (e.Response != nil && e.Response.StatusCode >= 400 && e.Response.StatusCode < 500) || (e.Message != nil && e.Message.Code != 0) {
				if source, ok := sources[elem.Source]; ok {
					maybeDisableFeed(source, elem, e)
				}

				break
			}
		} else {
			if onGuild, err := common.BotIsOnGuild(elem.Guild); !onGuild {
				if source, ok := sources[elem.Source]; ok {
					logger.WithError(err).Warnf("disabling feed item %s from %s to nonexistant guild", elem.SourceID, elem.Source)
					source.DisableFeed(elem, err)
				}

				break
			} else if err != nil {
				logger.WithError(err).Error("failed checking if bot is on guild")
			}
		}

		if c, _ := common.DiscordError(err); c != 0 {
			break
		}

		queueLogger.Warn("Non-discord related error when sending message, retrying. ", err)
		time.Sleep(time.Second)
	}
}

var disableOnError = []int{
	discordgo.ErrCodeUnknownChannel,
	discordgo.ErrCodeMissingAccess,
	discordgo.ErrCodeMissingPermissions,
	30007, // max number of webhooks
}

func maybeDisableFeed(source PluginWithErrorHandler, elem *QueuedElement, err *discordgo.RESTError) {
	// source.HandleMQueueError(elem, errors.Cause(err))
	if err.Message == nil || !common.ContainsIntSlice(disableOnError, err.Message.Code) {
		// don't disable
		l := logger.WithError(err).WithField("source", elem.Source).WithField("sourceid", elem.SourceID)
		if elem.MessageEmbed != nil {
			serializedEmbed, _ := json.Marshal(elem.MessageEmbed)
			l = l.WithField("embed", serializedEmbed)
		}

		l.Error("error sending mqueue message")
		return
	}

	logger.WithError(err).Warnf("disabling feed item %s from %s", elem.SourceID, elem.Source)
	source.DisableFeed(elem, err)
}

func trySendNormal(l *logrus.Entry, elem *QueuedElement) (err error) {
	if elem.MessageStr != "" {
		_, err = common.BotSession.ChannelMessageSend(elem.Channel, elem.MessageStr)
	} else if elem.MessageEmbed != nil {
		_, err = common.BotSession.ChannelMessageSendEmbed(elem.Channel, elem.MessageEmbed)
	} else {
		l.Error("Both MessageEmbed and MessageStr empty")
	}

	return
}

type CacheKeyWebhook int64

var ErrGuildNotFound = errors.New("Guild not found")

func trySendWebhook(l *logrus.Entry, elem *QueuedElement) (err error) {
	if elem.MessageStr == "" && elem.MessageEmbed == nil {
		l.Error("Both MessageEmbed and MessageStr empty")
		return
	}

	// find the avatar, this is slightly expensive, do i need to rethink this?
	avatar := ""
	if source, ok := sources[elem.Source]; ok {
		if avatarProvider, ok := source.(PluginWithWebhookAvatar); ok {
			avatar = avatarProvider.WebhookAvatar()
		}
	}

	gs := bot.State.Guild(true, elem.Guild)
	if gs == nil {
		// another check just in case
		if onGuild, err := common.BotIsOnGuild(elem.Guild); err == nil && !onGuild {
			return ErrGuildNotFound
		} else if err != nil {
			return err
		}
	}

	var wh interface{}
	// in some cases guild state may not be available (starting up and the like)
	if gs != nil {
		wh, err = gs.UserCacheFetch(CacheKeyWebhook(elem.Channel), func() (interface{}, error) {
			return FindCreateWebhook(elem.Guild, elem.Channel, elem.Source, avatar)
		})
	} else {
		// fallback if no gs is available
		wh, err = FindCreateWebhook(elem.Guild, elem.Channel, elem.Source, avatar)
		logger.WithField("guild", elem.Guild).Warn("Guild state not available, ignoring cache")
	}

	if err != nil {
		return err
	}
	webhook := wh.(*Webhook)

	webhookParams := &discordgo.WebhookParams{
		Username: elem.WebhookUsername,
		Content:  elem.MessageStr,
	}

	if elem.MessageEmbed != nil {
		webhookParams.Embeds = []*discordgo.MessageEmbed{elem.MessageEmbed}
	}

	err = webhookSession.WebhookExecute(webhook.ID, webhook.Token, true, webhookParams)
	if code, _ := common.DiscordError(err); code == discordgo.ErrCodeUnknownWebhook {
		// if the webhook was deleted, then delete the bad boi from the databse and retry
		const query = `DELETE FROM mqueue_webhooks WHERE id=$1`
		_, err := common.PQ.Exec(query, webhook.ID)
		if err != nil {
			return errors.WrapIf(err, "sql.delete")
		}

		if gs != nil {
			gs.UserCacheDel(CacheKeyWebhook(elem.Channel))
		}

		return errors.New("deleted webhook")
	}

	return
}

func handleWebhookSessionRatelimit(s *discordgo.Session, r *discordgo.RateLimit) {
	if !r.TooManyRequests.Global {
		return
	}

	if common.Statsd != nil {
		common.Statsd.Incr("yagpdb.webhook_session_ratelimit", nil, 1)
	}
}
