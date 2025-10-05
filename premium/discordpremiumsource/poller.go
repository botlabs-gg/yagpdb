package discordpremiumsource

import (
	"math"
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

type DiscordPremiumPoller struct {
	mu                   sync.RWMutex
	activeEntitlements   []*discordgo.Entitlement
	lastSuccesfulFetchAt time.Time
	isLastFetchSuccess   bool
}

func InitPoller() *DiscordPremiumPoller {
	poller := &DiscordPremiumPoller{}
	go poller.Run()
	return poller
}

func DiscordPremiumDisabled(err error, reason string) {
	l := logger

	if err != nil {
		l = l.WithError(err)
	}

	l.Warn("Not starting discord premium integration" + reason)
}

func (p *DiscordPremiumPoller) Run() {
	logger.Info("Starting Discord Premium Poller")
	ticker := time.NewTicker(time.Minute)
	for {
		p.Poll()
		<-ticker.C
	}
}

func (p *DiscordPremiumPoller) IsLastFetchSuccess() bool {
	if len(p.activeEntitlements) < 1 {
		return false
	}
	if time.Since(p.lastSuccesfulFetchAt) < time.Minute*5 {
		return p.isLastFetchSuccess
	}
	return false
}

func (p *DiscordPremiumPoller) Poll() {
	logger.Info("Fetching Discord SKUs")
	// Get your SKU data
	skus, err := common.BotSession.SKUs(common.BotApplication.ID)
	if err != nil || len(skus) < 1 {
		p.isLastFetchSuccess = false
		logger.WithError(err).Error("Failed fetching skus")
		return
	}
	logger.Infof("Got %d SKUs", len(skus))

	beforeID := int64(math.MaxInt64)
	filterOptions := &discordgo.EntitlementFilterOptions{
		ExcludeEnded: true,
		Limit:        100,
	}

	allEntitlements := make([]*discordgo.Entitlement, 0)
	for {
		logger.Infof("Fetching Entitlements before %d", beforeID)
		entitlements, err := common.BotSession.Entitlements(common.BotApplication.ID, filterOptions)
		if err != nil {
			p.isLastFetchSuccess = false
			logger.WithError(err).Error("Failed fetching Entitlements")
			break
		}
		if len(entitlements) == 0 {
			logger.Infof("Finished Fetching All Entitlements, Total Active Entitlements: %d", len(allEntitlements))
			break
		}
		for _, entitlement := range entitlements {
			if entitlement.ID < beforeID {
				beforeID = entitlement.ID
			}
			allEntitlements = append(allEntitlements, entitlement)
		}
		filterOptions.BeforeID = beforeID
		time.Sleep(time.Second)
	}
	// Swap the stored ones, this dosen't mutate the existing returned slices so we dont have to do any copying on each request woo
	p.mu.Lock()
	p.isLastFetchSuccess = true
	p.lastSuccesfulFetchAt = time.Now()
	p.activeEntitlements = allEntitlements
	p.mu.Unlock()
}

func (p *DiscordPremiumPoller) GetEntitlements() (entitlements []*discordgo.Entitlement) {
	p.mu.RLock()
	entitlements = p.activeEntitlements
	p.mu.RUnlock()
	return
}
