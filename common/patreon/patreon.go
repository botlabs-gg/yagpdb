package patreon

var ActivePoller *Poller

type Patron struct {
	Name        string
	Avatar      string
	DiscordID   int64
	Tiers       []int64
	AmountCents int64
}
