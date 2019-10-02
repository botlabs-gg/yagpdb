package patreon

var ActivePoller *Poller

type Patron struct {
	Name        string
	Avatar      string
	AmountCents int
	DiscordID   int64
}
