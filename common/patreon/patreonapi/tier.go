package patreonapi

type TierResponse struct {
	Data TierResponseData `json:"data"`
}

type TierResponseData struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Attributes *TierAttributes `json:"attributes"`
}

type TierAttributes struct {
	AmountCents int `json:"amount_cents"`
}
