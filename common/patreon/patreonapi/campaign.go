package patreonapi

type CampaignsResponse struct {
	Data []*CampaignData `json:"data"`
}

type CampaignData struct {
	ID         string              `json:"id"`
	Type       string              `json:"type"`
	Attributes *CampaignAttributes `json:"attributes"`
}

type CampaignAttributes struct {
}
