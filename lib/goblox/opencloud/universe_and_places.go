package opencloud

import (
	"context"
	"fmt"
	"net/http"
)

// UniverseAndPlacesService will handle communciation with the actions related to the API.
//
// Roblox Open Cloud API Docs: https://create.roblox.com/docs/en-us/cloud
type UniverseAndPlacesService service

type InstanceRunContext string

const (
	InstanceRunContextLegacy InstanceRunContext = "Legacy"
	InstanceRunContextServer InstanceRunContext = "Server"
	InstanceRunContextClient InstanceRunContext = "Client"
	InstanceRunContextPlugin InstanceRunContext = "Plugin"
)

type InstanceFolder any

type InstanceLocalScript struct {
	Enabled    bool               `json:"Enabled"`
	RunContext InstanceRunContext `json:"RunContext"`
	Source     string             `json:"Source"`
}

type InstanceModuleScript struct {
	Source string `json:"Source"`
}

type InstanceScript struct {
	Enabled    bool               `json:"Enabled"`
	RunContext InstanceRunContext `json:"RunContext"`
	Source     string             `json:"Source"`
}

type InstanceDetails struct {
	Folder       *InstanceFolder       `json:"Folder"`
	LocalScript  *InstanceLocalScript  `json:"LocalScript"`
	ModuleScript *InstanceModuleScript `json:"ModuleScript"`
	Script       *InstanceScript       `json:"Script"`
}

type EngineInstance struct {
	Id     string `json:"Id"`
	Parent string `json:"Parent"`
	Name   string `json:"Name"`
}

type Instance struct {
	Path           string         `json:"path"`
	HasChildren    bool           `json:"hasChildren"`
	EngineInstance EngineInstance `json:"engineInstance"`
}

// GetInstance will fetch an instance's property data.
//
// Required scopes: universe.place.instance:read
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/Instance#Cloud_GetInstance
//
// [GET] /cloud/v2/universes/{universe_id}/places/{place_id}/instances/{instance_id}
func (s *UniverseAndPlacesService) GetInstance(ctx context.Context, universeId, placeId, instanceId string) (*Instance, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/places/%s/instances/%s", universeId, placeId, instanceId)

	req, err := s.client.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, err
	}

	instance := new(Instance)
	resp, err := s.client.Do(ctx, req, instance)
	if err != nil {
		return nil, resp, err
	}

	return instance, resp, nil
}

// UpdateInstance will update an instance's property data.
//
// Required scopes: universe.place.instance:write
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/Instance#Cloud_UpdateInstance
//
// [PATCH] /cloud/v2/universes/{universe_id}/places/{place_id}/instances/{instance_id}
func (s *UniverseAndPlacesService) UpdateInstance() error {
	// I'm not entirely sure the purpose of this endpoint, the docs do not help with how this endpoint is used.
	// The main concern is the data being pushed and what it is used for. I'm not sure how to actually go about implementing it.
	// - LuckFire

	return fmt.Errorf("UpdateInstance: Method is not implemented. Need this method? Make a PR on the GitHub!\nhttps://github.com/typical-developers/goblox")
}

type InstanceChildren struct {
	Instances     []Instance `json:"instances"`
	NextPageToken string     `json:"nextPageToken"`
}

// ListInstanceChildren will list and instance's children.
//
// Required scopes: universe.place.instance:read
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/Instance#Cloud_ListInstanceChildren
//
// [GET] /cloud/v2/universes/{universe_id}/places/{place_id}/instances/{instance_id}:listChildren
func (s *UniverseAndPlacesService) ListInstanceChildren(ctx context.Context, universeId, placeId, instanceId string, opts *Options) (*InstanceChildren, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/places/%s/instances/%s:listChildren", universeId, placeId, instanceId)

	u, err := addOpts(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, err
	}

	instanceChildren := new(InstanceChildren)
	resp, err := s.client.Do(ctx, req, instanceChildren)
	if err != nil {
		return nil, resp, err
	}

	return instanceChildren, resp, nil
}

type Place struct {
	Path        string `json:"path"`
	CreateTime  string `json:"createTime"`
	UpdateTime  string `json:"updateTime"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	ServerSize  int    `json:"serverSize"`
}

// GetPlace will get information on a specified place.
//
// Required scopes: none
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/Place#Cloud_GetPlace
//
// [GET] /cloud/v2/universes/{universe_id}/places/{place_id}
func (s *UniverseAndPlacesService) GetPlace(ctx context.Context, universeId, placeId string) (*Place, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/places/%s", universeId, placeId)

	req, err := s.client.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, err
	}

	place := new(Place)
	resp, err := s.client.Do(ctx, req, place)
	if err != nil {
		return nil, resp, err
	}

	return place, resp, nil
}

type PlaceUpdate struct {
	DisplayName *string `json:"displayName,omitempty"`
	Description *string `json:"description,omitempty"`
	ServerSize  *int    `json:"serverSize,omitempty"`
}

type PlaceUpdateOptions struct {
	UpdateMask *string `url:"updateMask,omitempty"`
}

// UpdatePlace will update information for a specified place.
//
// Required scopes: universe.place:write
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/Place#Cloud_UpdatePlace
//
// [PATCH] /cloud/v2/universes/{universe_id}/places/{place_id}
func (s *UniverseAndPlacesService) UpdatePlace(ctx context.Context, universeId, placeId string, data PlaceUpdate, opts *PlaceUpdateOptions) (*Place, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/places/%s", universeId, placeId)

	u, err := addOpts(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(http.MethodPatch, u, data)
	if err != nil {
		return nil, nil, err
	}

	place := new(Place)
	resp, err := s.client.Do(ctx, req, place)
	if err != nil {
		return nil, resp, err
	}

	return place, resp, nil
}

type UniverseVisibility string

const (
	UniverseVisibilityUnspecified UniverseVisibility = "VISIBILITY_UNSPECIFIED"
	UniverseVisibilityPublic      UniverseVisibility = "PUBLIC"
	UniverseVisibilityPrivate     UniverseVisibility = "PRIVATE"
)

type UniverseAgeRating string

const (
	UniverseAgeRatingUnspecified UniverseAgeRating = "AGE_RATING_UNSPECIFIED"
	UniverseAgeRatingAll         UniverseAgeRating = "AGE_RATING_ALL"
	UniverseAgeRating9Plus       UniverseAgeRating = "AGE_RATING_9_PLUS"
	UniverseAgeRating13Plus      UniverseAgeRating = "AGE_RATING_13_PLUS"
	UniverseAgeRating17Plus      UniverseAgeRating = "AGE_RATING_17_PLUS"
)

type UniverseSocialLink struct {
	Title string `json:"title"`
	URI   string `json:"uri"`
}

type Universe struct {
	Path                    string             `json:"path"`
	CreateTime              string             `json:"createTime"`
	UpdateTime              string             `json:"updateTime"`
	DisplayName             string             `json:"displayName"`
	Description             string             `json:"description"`
	User                    *string            `json:"user"`
	Group                   *string            `json:"group"`
	Visibility              UniverseVisibility `json:"visibility"`
	FacebookSocialLink      UniverseSocialLink `json:"facebookSocialLink"`
	TwitterSocialLink       UniverseSocialLink `json:"twitterSocialLink"`
	YoutubeSocialLink       UniverseSocialLink `json:"youtubeSocialLink"`
	TwitchSocialLink        UniverseSocialLink `json:"twitchSocialLink"`
	DiscordSocialLink       UniverseSocialLink `json:"discordSocialLink"`
	RobloxGroupSocialLink   UniverseSocialLink `json:"robloxgroupSocialLink"`
	GuildedSocialLink       UniverseSocialLink `json:"guildedSocialLink"`
	VoiceChatEnabled        bool               `json:"voiceChatEnabled"`
	AgeRating               UniverseAgeRating  `json:"ageRating"`
	PrivateServerPriceRobux int                `json:"privateServerPriceRobux"`
	DesktopEnabled          bool               `json:"desktopEnabled"`
	MobileEnabled           bool               `json:"mobileEnabled"`
	TabletEnabled           bool               `json:"tabletEnabled"`
	ConsoleEnabled          bool               `json:"consoleEnabled"`
	VREnabled               bool               `json:"vrEnabled"`
}

// GetUniverse will fetch information on a specified universe.
//
// Required scopes: none
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/Universe#Cloud_GetUniverse
//
// [GET] /cloud/v2/universes/{universe_id}
func (s *UniverseAndPlacesService) GetUniverse(ctx context.Context, universeId string) (*Universe, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s", universeId)

	req, err := s.client.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, err
	}

	universe := new(Universe)
	resp, err := s.client.Do(ctx, req, universe)
	if err != nil {
		return nil, resp, err
	}

	return universe, resp, nil
}

type UniverseUpdate struct {
	DisplayName             *string             `json:"displayName,omitempty"`
	Description             *string             `json:"description,omitempty"`
	Visibility              *UniverseVisibility `json:"visibility,omitempty"`
	FacebookSocialLink      *UniverseSocialLink `json:"facebookSocialLink,omitempty"`
	TwitterSocialLink       *UniverseSocialLink `json:"twitterSocialLink,omitempty"`
	YoutubeSocialLink       *UniverseSocialLink `json:"youtubeSocialLink,omitempty"`
	TwitchSocialLink        *UniverseSocialLink `json:"twitchSocialLink,omitempty"`
	DiscordSocialLink       *UniverseSocialLink `json:"discordSocialLink,omitempty"`
	RobloxGroupSocialLink   *UniverseSocialLink `json:"robloxgroupSocialLink,omitempty"`
	VoiceChatEnabled        *bool               `json:"voiceChatEnabled,omitempty"`
	PrivateServerPriceRobux *int                `json:"privateServerPriceRobux,omitempty"`
	DesktopEnabled          *bool               `json:"desktopEnabled,omitempty"`
	MobileEnabled           *bool               `json:"mobileEnabled,omitempty"`
	TabletEnabled           *bool               `json:"tabletEnabled,omitempty"`
	ConsoleEnabled          *bool               `json:"consoleEnabled,omitempty"`
	VREnabled               *bool               `json:"vrEnabled,omitempty"`
}

// UpdateUniverse will update information for a specified universe.
//
// Required scopes: universe:write
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/Universe#Cloud_UpdateUniverse
//
// [PATCH] /cloud/v2/universes/{universe_id}
func (s *UniverseAndPlacesService) UpdateUniverse(ctx context.Context, universeId string, data UniverseUpdate) (*Universe, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s", universeId)

	req, err := s.client.NewRequest(http.MethodPatch, u, data)
	if err != nil {
		return nil, nil, err
	}

	universe := new(Universe)
	resp, err := s.client.Do(ctx, req, universe)
	if err != nil {
		return nil, resp, err
	}

	return universe, resp, nil
}

type UniverseMessage struct {
	Topic   string `json:"topic"`
	Message string `json:"message"`
}

// PublishUniverseMessage will publish a message to all of the universe's live servers.
//
// Required scopes: universe-messaging-service:publish
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/Universe#Cloud_PublishUniverseMessage
//
// [POST] /cloud/v2/universes/{universe_id}:publishMessage
func (s *UniverseAndPlacesService) PublishUniverseMessage(ctx context.Context, universeId string, data UniverseMessage) (*Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s:publishMessage", universeId)

	req, err := s.client.NewRequest(http.MethodPost, u, data)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(ctx, req, nil)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

// RestartUniverseServers will restart all active servers for a specific universe ONLY if a new version of the experience has been published.
//
// Required scopes: universe:write
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/Universe#Cloud_RestartUniverseServers
//
// [POST] /cloud/v2/universes/{universe_id}:restartServers
func (s *UniverseAndPlacesService) RestartUniverseServers(ctx context.Context, universeId string) (*Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s:restartServers", universeId)

	req, err := s.client.NewRequest(http.MethodPost, u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(ctx, req, nil)
	if err != nil {
		return resp, err
	}

	return resp, nil
}
