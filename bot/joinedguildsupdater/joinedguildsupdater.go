package joinedguildsupdater

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/bot/models"
	"github.com/jonas747/yagpdb/common"
	"github.com/volatiletech/sqlboiler/boil"
)

var logger = common.GetFixedPrefixLogger("joinedguildsupdater")

type updater struct {
	flushInterval time.Duration
	Incoming      chan *eventsystem.EventData

	waiting    []*QueuedAction
	processing []*QueuedAction

	flushInProgress *int32
}

func NewUpdater() *updater {
	u := &updater{
		flushInterval:   time.Second * 3,
		Incoming:        make(chan *eventsystem.EventData, 100),
		flushInProgress: new(int32),
	}

	go u.run()
	return u
}

type QueuedAction struct {
	GuildID        int64
	Guild          *discordgo.GuildCreate
	MemberCountMod int
}

func (u *updater) run() {

	t := time.NewTicker(u.flushInterval)
	for {
		select {
		case <-t.C:
			if atomic.LoadInt32(u.flushInProgress) != 0 {
				logger.Error("last flush took too long, waiting...")
				continue
			}

			if len(u.waiting) == 0 && len(u.processing) == 0 {
				continue
			}

			logger.Infof("Joined guilds, len:%d, leftovers:%d", len(u.waiting), len(u.processing))

			// merge the leftovers form last run into the main queue
		OUTER:
			for _, pv := range u.processing {
				for _, wv := range u.waiting {
					if wv.GuildID == pv.GuildID {
						wv.MemberCountMod += wv.MemberCountMod
						continue OUTER
					}
				}

				u.waiting = append(u.waiting, pv)
			}

			if len(u.waiting) > 25 {
				u.processing = u.waiting[:25]
				u.waiting = u.waiting[25:]
			} else {
				u.processing = u.waiting
				u.waiting = nil
			}

			atomic.StoreInt32(u.flushInProgress, 1)
			go u.flush()
		case evt := <-u.Incoming:
			u.handleIncEvent(evt)
		}
	}
}

func (u *updater) handleIncEvent(evt *eventsystem.EventData) {
	q := QueuedAction{}

	switch evt.Type {
	case eventsystem.EventGuildCreate:
		e := evt.GuildCreate()
		q.GuildID = e.ID
		q.Guild = e
	case eventsystem.EventGuildMemberAdd:
		q.GuildID = evt.GS.ID
		q.MemberCountMod = 1
	case eventsystem.EventGuildMemberRemove:
		q.GuildID = evt.GS.ID
		q.MemberCountMod = -1
	}

	for _, v := range u.waiting {
		if v.GuildID == q.GuildID {
			v.MemberCountMod += q.MemberCountMod

			if q.Guild != nil {
				// reset pending member change writes
				v.MemberCountMod = 0
				v.Guild = q.Guild
			}

			return
		}
	}

	u.waiting = append(u.waiting, &q)
}

func (u *updater) flush() {
	const qCountOnly = `UPDATE joined_guilds SET member_count = member_count + $2 WHERE id = $1`

	leftOver := make([]*QueuedAction, 0)
	defer func() {
		u.processing = leftOver
		atomic.StoreInt32(u.flushInProgress, 0)
	}()

	tx, err := common.PQ.Begin()
	if err != nil {
		leftOver = u.processing
		logger.WithError(err).Error("failed creating tx")
		return
	}

	// update all the stats
	for _, q := range u.processing {
		if q.Guild != nil {
			gm := &models.JoinedGuild{
				ID:          q.Guild.ID,
				MemberCount: int64(q.Guild.MemberCount + q.MemberCountMod),
				OwnerID:     q.Guild.OwnerID,
				JoinedAt:    time.Now(),
				Name:        q.Guild.Name,
				Avatar:      q.Guild.Icon,
			}

			err = gm.Upsert(context.Background(), tx, true, []string{"id"}, boil.Whitelist("member_count", "name", "avatar", "owner_id", "left_at"), boil.Infer())

		} else {
			_, err = tx.Exec(qCountOnly, q.GuildID, q.MemberCountMod)
		}

		if err != nil {
			leftOver = u.processing
			tx.Rollback()
			logger.WithError(err).Error("failed updating joined guild, rollbacking...")
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		logger.WithError(err).Error("failed comitting updating joined guild results")
		leftOver = u.processing
		tx.Rollback()
		return
	}
}
