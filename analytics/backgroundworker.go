package analytics

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/backgroundworkers"
	"github.com/mediocregopher/radix/v3"
)

var _ backgroundworkers.BackgroundWorkerPlugin = (*Plugin)(nil)

func (p *Plugin) RunBackgroundWorker() {
	ticker := time.NewTicker(time.Minute * 60)
	for {
		select {
		case <-ticker.C:
			logger.Info("Performing saving of temp analytics")
			started := time.Now()
			err := p.saveTempStats()
			if err != nil {
				logger.WithError(err).Error("failed saving temp analytics")
			}
			logger.Infof("Took %s to save analytics", time.Since(started))
		case wg := <-p.stopWorkers:
			wg.Done()
			return
		}

		p.RunBackgroundWorker()
	}
}

func (p *Plugin) StopBackgroundWorker(wg *sync.WaitGroup) {
	p.stopWorkers <- wg
}

type ActivityBucket struct {
	GuildID int64
	Count   int
	Plugin  string
	Name    string
}

func (p *Plugin) saveTempStats() error {
	compiled := make(map[string][]*ActivityBucket)

	err := common.RedisPool.Do(radix.WithConn("", func(c radix.Conn) error {
		s := radix.NewScanner(c, radix.ScanOpts{
			Command: "SCAN",
			Pattern: "anaylytics_active_units.*",
		})

		var key string
		for s.Next(&key) {

			// copy it to a safe location first
			err := c.Do(radix.Cmd(nil, "RENAME", key, "temp_"+key))
			if err != nil {
				return errors.WithStackIf(err)
			}

			// read raw counts
			var rawCounts map[string]string
			err = c.Do(radix.Cmd(&rawCounts, "HGETALL", "temp_"+key))
			if err != nil {
				return errors.WithStackIf(err)
			}

			// get plugin and metric name from key
			split := strings.Split(key, ".")
			if len(split) < 3 {
				return errors.New("Incorrect length on analytic name")
			}
			plugin := split[1]
			metricName := split[2]

			bucket := compiled[plugin+"."+metricName]
		OUTER:
			for g, countStr := range rawCounts {
				parsedG, _ := strconv.ParseInt(g, 10, 64)
				parsedCount, _ := strconv.Atoi(countStr)

				for _, compiledCount := range bucket {
					if compiledCount.GuildID == parsedG {
						compiledCount.Count += parsedCount
						continue OUTER
					}
				}

				// did not find the entry for this guild, create a new one
				bucket = append(bucket, &ActivityBucket{
					GuildID: parsedG,
					Plugin:  plugin,
					Name:    metricName,
					Count:   parsedCount,
				})
			}

			compiled[plugin+"."+metricName] = bucket

			// clear it
			err = c.Do(radix.Cmd(nil, "DEL", "temp_"+key))
			if err != nil {
				return errors.WithStackIf(err)
			}
		}

		return nil
	}))

	if err != nil {
		return errors.WithStackIf(err)
	}

	const q = `INSERT INTO analytics (guild_id, created_at, plugin, name, count)
	VALUES ($1, now(), $2, $3, $4)`

	for _, bucket := range compiled {
		for _, entry := range bucket {
			_, err := common.PQ.Exec(q, entry.GuildID, entry.Plugin, entry.Name, entry.Count)
			if err != nil {
				return errors.WithStackIf(err)
			}
		}
	}
	return nil
}
