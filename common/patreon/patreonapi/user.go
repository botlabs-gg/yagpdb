package patreonapi

type UserResponse struct {
	Data UserResponseData `json:"data"`
}

type UserResponseData struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Attributes *UserAttributes `json:"attributes"`
}

type UserAttributes struct {
	About   string `json:"about"`
	Created string `json:"created"`
	Email   string `json:"email"`

	Vanity    string `json:"vanity"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	FullName  string `json:"full_name"`

	ImageURL string `json:"image_url"`
	ThumbURL string `json:"thumb_url"`

	SocialConnections *SocialConnections `json:"social_connections"`
}

type SocialConnections struct {
	Discord *SocialConnection `json:"discord"`
}

type SocialConnection struct {
	UserID string `json:"user_id"`
}
