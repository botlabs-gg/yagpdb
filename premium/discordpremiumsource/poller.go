package discordpremiumsource

import (
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/config"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

type DiscordPremiumPoller struct {
	mu                   sync.RWMutex
	activeEntitlements   []*discordgo.Entitlement
	lastSuccesfulFetchAt time.Time
	isLastFetchSuccess   bool
}

var confDiscordPremiumSKUID = config.RegisterOption("yagpdb.discord.premium.sku_id", "SKU_ID for Discord Premium", nil)

func InitPoller() *DiscordPremiumPoller {

	discordPremiumSKUID := int64(confDiscordPremiumSKUID.GetInt())
	if discordPremiumSKUID == 0 {
		DiscordPremiumDisabled(nil, "Missing Discord Premium SKU_ID, not starting poller")
		return nil
	}

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
	ticker := time.NewTicker(time.Minute * 10)
	for {
		p.Poll()
		<-ticker.C
	}
}

func (p *DiscordPremiumPoller) IsLastFetchSuccess() bool {
	if p.activeEntitlements == nil || len(p.activeEntitlements) < 1 {
		return false
	}
	if time.Since(p.lastSuccesfulFetchAt) < time.Minute*10 {
		return p.isLastFetchSuccess
	}
	return false
}

func (p *DiscordPremiumPoller) Poll() {
	logger.Info("Fetching Discord SKUs")
	discordPremiumSKUID := int64(confDiscordPremiumSKUID.GetInt())
	// Get your SKU data
	skus, err := common.BotSession.SKUs(common.BotApplication.ID)
	if err != nil || len(skus) < 1 {
		p.isLastFetchSuccess = false
		logger.WithError(err).Error("Failed fetching skus")
		return
	}

	var is_sku_configured bool
	for _, sku := range skus {
		if sku.ID == discordPremiumSKUID {
			is_sku_configured = true
			break
		}
	}

	if !is_sku_configured {
		p.isLastFetchSuccess = false
		logger.Error("SKU ID not found in bot's application SKUs")
		return
	}

	afterID := int64(0)
	filterOptions := &discordgo.EntitlementFilterOptions{
		SkuIDs:       []int64{discordPremiumSKUID},
		ExcludeEnded: true,
		Limit:        100,
	}
	allEntitlements := make([]*discordgo.Entitlement, 0)
	for {
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
			if entitlement.ID > afterID {
				afterID = entitlement.ID
			}
			allEntitlements = append(allEntitlements, entitlement)
		}
		filterOptions.AfterID = afterID
		p.isLastFetchSuccess = true
		p.lastSuccesfulFetchAt = time.Now()
		time.Sleep(time.Second)
	}
	// Swap the stored ones, this dosen't mutate the existing returned slices so we dont have to do any copying on each request woo
	p.mu.Lock()
	p.activeEntitlements = allEntitlements
	p.mu.Unlock()
}

func (p *DiscordPremiumPoller) GetEntitlements() (entitlements []*discordgo.Entitlement) {
	p.mu.RLock()
	entitlements = p.activeEntitlements
	p.mu.RUnlock()
	return
}
