package reddit

// Media is a media content item.
type Media struct {
	Oembed Oembed `json:"oembed"`
	Type   string `json:"type"`
}

// Oembed contains embedding information for a media item.
type Oembed struct {
	Description     string `json:"description"`
	HTML            string `json:"html"`
	Height          int    `json:"height"`
	ProviderName    string `json:"provider_name"`
	ProviderURL     string `json:"provider_url"`
	ThumbnailHeight int    `json:"thumbnail_height"`
	ThumbnailURL    string `json:"thumbnail_url"`
	ThumbnailWidth  int    `json:"thumbnail_width"`
	Title           string `json:"title"`
	Type            string `json:"type"`
	Version         string `json:"version"`
	Width           int    `json:"width"`
}
