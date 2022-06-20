// Discordgo - Discord bindings for Go
// Available at https://github.com/bwmarrin/discordgo

// Copyright 2015-2016 Bruce Marriner <bruce@sqls.net>.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains functions for interacting with the Discord REST/JSON API
// at the lowest level.

package discordgo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg" // For JPEG decoding
	_ "image/png"  // For PNG decoding
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
)

// All error constants
var (
	ErrJSONUnmarshal           = errors.New("json unmarshal")
	ErrStatusOffline           = errors.New("You can't set your Status to offline")
	ErrVerificationLevelBounds = errors.New("VerificationLevel out of bounds, should be between 0 and 3")
	ErrPruneDaysBounds         = errors.New("the number of days should be more than or equal to 1")
	ErrGuildNoIcon             = errors.New("guild does not have an icon set")
	ErrGuildNoSplash           = errors.New("guild does not have a splash set")
	ErrUnauthorized            = errors.New("HTTP request was unauthorized. This could be because the provided token was not a bot token. Please add \"Bot \" to the start of your token. https://discordapp.com/developers/docs/reference#authentication-example-bot-token-authorization-header")
	ErrTokenInvalid            = errors.New("Invalid token provided, it has been marked as invalid")
)

// Request is the same as RequestWithBucketID but the bucket id is the same as the urlStr
func (s *Session) Request(method, urlStr string, data interface{}, headers map[string]string) (response []byte, err error) {
	return s.RequestWithBucketID(method, urlStr, data, headers, strings.SplitN(urlStr, "?", 2)[0])
}

// RequestWithBucketID makes a (GET/POST/...) Requests to Discord REST API with JSON data.
func (s *Session) RequestWithBucketID(method, urlStr string, data interface{}, headers map[string]string, bucketID string) (response []byte, err error) {
	var body []byte
	if data != nil {
		body, err = json.Marshal(data)
		if err != nil {
			return
		}
	}

	return s.request(method, urlStr, "application/json", body, headers, bucketID)
}

// request makes a (GET/POST/...) Requests to Discord REST API.
// Sequence is the sequence number, if it fails with a 502 it will
// retry with sequence+1 until it either succeeds or sequence >= session.MaxRestRetries
func (s *Session) request(method, urlStr, contentType string, b []byte, headers map[string]string, bucketID string) (response []byte, err error) {
	if bucketID == "" {
		bucketID = strings.SplitN(urlStr, "?", 2)[0]
	}

	return s.RequestWithBucket(method, urlStr, contentType, b, headers, s.Ratelimiter.GetBucket(bucketID))
}

type ReaderWithMockClose struct {
	*bytes.Reader
}

func (rwmc *ReaderWithMockClose) Close() error {
	return nil
}

// RequestWithLockedBucket makes a request using a bucket that's already been locked
func (s *Session) RequestWithBucket(method, urlStr, contentType string, b []byte, headers map[string]string, bucket *Bucket) (response []byte, err error) {

	for i := 0; i < s.MaxRestRetries; i++ {
		var retry bool
		var ratelimited bool
		response, retry, ratelimited, err = s.doRequest(method, urlStr, contentType, b, headers, bucket)
		if !retry {
			break
		}

		if err != nil {
			s.log(LogError, "Request error, retrying: %v", err)
		}

		if ratelimited {
			i = 0
		} else {
			time.Sleep(time.Second * time.Duration(i))
		}

	}

	return
}

type CtxKey int

const (
	CtxKeyRatelimitBucket CtxKey = iota
)

// doRequest makes a request using a bucket
func (s *Session) doRequest(method, urlStr, contentType string, b []byte, headers map[string]string, bucket *Bucket) (response []byte, retry bool, ratelimitRetry bool, err error) {

	if atomic.LoadInt32(s.tokenInvalid) != 0 {
		return nil, false, false, ErrTokenInvalid
	}

	req, resp, err := s.innerDoRequest(method, urlStr, contentType, b, headers, bucket)
	if err != nil {
		return nil, true, false, err
	}

	defer func() {
		err2 := resp.Body.Close()
		if err2 != nil {
			log.Println("error closing resp body")
		}
	}()

	response, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, true, false, err
	}

	if s.Debug {

		log.Printf("API RESPONSE  STATUS :: %s\n", resp.Status)
		for k, v := range resp.Header {
			log.Printf("API RESPONSE  HEADER :: [%s] = %+v\n", k, v)
		}
		log.Printf("API RESPONSE    BODY :: [%s]\n\n\n", response)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return
	}

	switch resp.StatusCode {
	case http.StatusBadGateway, http.StatusGatewayTimeout:
		// Retry sending request if possible
		err = errors.Errorf("%s Failed (%s)", urlStr, resp.Status)
		s.log(LogWarning, err.Error())
		return nil, true, false, err

	case 429: // TOO MANY REQUESTS - Rate limiting
		rl := TooManyRequests{}
		err = json.Unmarshal(response, &rl)
		if err != nil {
			s.log(LogError, "rate limit unmarshal error, %s, %q, url: %s", err, string(response), urlStr)
			return
		}

		rl.Bucket = bucket.Key

		s.log(LogInformational, "Rate Limiting %s, retry in %s", urlStr, rl.RetryAfterDur())
		s.handleEvent(rateLimitEventType, &RateLimit{TooManyRequests: &rl, URL: urlStr})

		time.Sleep(rl.RetryAfterDur())
		// we can make the above smarter
		// this method can cause longer delays than required
		return nil, true, true, nil

	// case http.StatusUnauthorized:
	// 	if strings.Index(s.Token, "Bot ") != 0 {
	// 		s.log(LogInformational, ErrUnauthorized.Error())
	// 		err = ErrUnauthorized
	// 	} else {
	// 		atomic.StoreInt32(s.tokenInvalid, 1)
	// 		err = ErrTokenInvalid
	// 	}
	default: // Error condition
		if resp.StatusCode >= 500 || resp.StatusCode < 400 {
			// non 400 response code
			retry = true
		}

		err = newRestError(req, resp, response)
	}

	return
}

func (s *Session) innerDoRequest(method, urlStr, contentType string, b []byte, headers map[string]string, bucket *Bucket) (*http.Request, *http.Response, error) {
	bucketLockID := s.Ratelimiter.LockBucketObject(bucket)
	defer func() {
		err := bucket.Release(nil, bucketLockID)
		if err != nil {
			s.log(LogError, "failed unlocking ratelimit bucket: %v", err)
		}
	}()

	if s.Debug {
		log.Printf("API REQUEST %8s :: %s\n", method, urlStr)
		log.Printf("API REQUEST  PAYLOAD :: [%s]\n", string(b))
	}

	req, err := http.NewRequest(method, urlStr, bytes.NewReader(b))
	if err != nil {
		return nil, nil, err
	}

	req.GetBody = func() (io.ReadCloser, error) {
		return &ReaderWithMockClose{bytes.NewReader(b)}, nil
	}

	// we may need to send a request with extra headers
	if headers != nil {
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}

	// Not used on initial login..
	// TODO: Verify if a login, otherwise complain about no-token
	if s.Token != "" {
		req.Header.Set("authorization", s.Token)
	}

	// Discord's API returns a 400 Bad Request is Content-Type is set, but the
	// request body is empty.
	if b != nil {
		req.Header.Set("Content-Type", contentType)
	}

	// TODO: Make a configurable static variable.
	req.Header.Set("User-Agent", fmt.Sprintf("DiscordBot (https://github.com/botlabs-gg/discordgo, v%s)", VERSION))

	// for things such as stats collecting in the roundtripper for example
	ctx := context.WithValue(req.Context(), CtxKeyRatelimitBucket, bucket)
	req = req.WithContext(ctx)

	if s.Debug {
		for k, v := range req.Header {
			log.Printf("API REQUEST   HEADER :: [%s] = %+v\n", k, v)
		}
	}

	resp, err := s.Client.Do(req)
	if err == nil {
		err = bucket.Release(resp.Header, bucketLockID)
	}

	return req, resp, err
}

func unmarshal(data []byte, v interface{}) error {
	err := json.Unmarshal(data, v)
	return err
}

// ------------------------------------------------------------------------------------------------
// Functions specific to Discord Users
// ------------------------------------------------------------------------------------------------

// User returns the user details of the given userID
// userID    : A user ID
func (s *Session) User(userID int64) (st *User, err error) {

	body, err := s.RequestWithBucketID("GET", EndpointUser(StrID(userID)), nil, nil, EndpointUsers)
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// UserMe returns the user details of the current user
func (s *Session) UserMe() (st *User, err error) {

	body, err := s.RequestWithBucketID("GET", EndpointUser("@me"), nil, nil, EndpointUsers)
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// UserAvatar is deprecated. Please use UserAvatarDecode
// userID    : A user ID or "@me" which is a shortcut of current user ID
func (s *Session) UserAvatar(userID int64) (img image.Image, err error) {
	u, err := s.User(userID)
	if err != nil {
		return
	}
	img, err = s.UserAvatarDecode(u)
	return
}

// UserAvatarDecode returns an image.Image of a user's Avatar
// user : The user which avatar should be retrieved
func (s *Session) UserAvatarDecode(u *User) (img image.Image, err error) {
	body, err := s.RequestWithBucketID("GET", EndpointUserAvatar(u.ID, u.Avatar), nil, nil, EndpointUserAvatar(0, ""))
	if err != nil {
		return
	}

	img, _, err = image.Decode(bytes.NewReader(body))
	return
}

// UserUpdate updates a users settings.
func (s *Session) UserUpdate(email, password, username, avatar, newPassword string) (st *User, err error) {

	// NOTE: Avatar must be either the hash/id of existing Avatar or
	// data:image/png;base64,BASE64_STRING_OF_NEW_AVATAR_PNG
	// to set a new avatar.
	// If left blank, avatar will be set to null/blank

	data := struct {
		Email       string `json:"email,omitempty"`
		Password    string `json:"password,omitempty"`
		Username    string `json:"username,omitempty"`
		Avatar      string `json:"avatar,omitempty"`
		NewPassword string `json:"new_password,omitempty"`
	}{email, password, username, avatar, newPassword}

	body, err := s.RequestWithBucketID("PATCH", EndpointUser("@me"), data, nil, EndpointUsers)
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// UserSettings returns the settings for a given user
func (s *Session) UserSettings() (st *Settings, err error) {

	body, err := s.RequestWithBucketID("GET", EndpointUserSettings("@me"), nil, nil, EndpointUserSettings(""))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// UserUpdateStatus update the user status
// status   : The new status (Actual valid status are 'online','idle','dnd','invisible')
func (s *Session) UserUpdateStatus(status Status) (st *Settings, err error) {
	if status == StatusOffline {
		err = ErrStatusOffline
		return
	}

	data := struct {
		Status Status `json:"status"`
	}{status}

	body, err := s.RequestWithBucketID("PATCH", EndpointUserSettings("@me"), data, nil, EndpointUserSettings(""))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// UserConnections returns the user's connections
func (s *Session) UserConnections() (conn []*UserConnection, err error) {
	response, err := s.RequestWithBucketID("GET", EndpointUserConnections("@me"), nil, nil, EndpointUserConnections("@me"))
	if err != nil {
		return nil, err
	}

	err = unmarshal(response, &conn)
	if err != nil {
		return
	}

	return
}

// UserChannels returns an array of Channel structures for all private
// channels.
func (s *Session) UserChannels() (st []*Channel, err error) {

	body, err := s.RequestWithBucketID("GET", EndpointUserChannels("@me"), nil, nil, EndpointUserChannels(""))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// UserChannelCreate creates a new User (Private) Channel with another User
// recipientID : A user ID for the user to which this channel is opened with.
func (s *Session) UserChannelCreate(recipientID int64) (st *Channel, err error) {

	data := struct {
		RecipientID int64 `json:"recipient_id,string"`
	}{recipientID}

	body, err := s.RequestWithBucketID("POST", EndpointUserChannels("@me"), data, nil, EndpointUserChannels(""))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// UserGuilds returns an array of UserGuild structures for all guilds.
// limit     : The number guilds that can be returned. (max 100)
// beforeID  : If provided all guilds returned will be before given ID.
// afterID   : If provided all guilds returned will be after given ID.
func (s *Session) UserGuilds(limit int, beforeID, afterID int64) (st []*UserGuild, err error) {

	v := url.Values{}

	if limit > 0 {
		v.Set("limit", strconv.Itoa(limit))
	}
	if afterID != 0 {
		v.Set("after", StrID(afterID))
	}
	if beforeID != 0 {
		v.Set("before", StrID(beforeID))
	}

	uri := EndpointUserGuilds("@me")

	if len(v) > 0 {
		uri = fmt.Sprintf("%s?%s", uri, v.Encode())
	}

	body, err := s.RequestWithBucketID("GET", uri, nil, nil, EndpointUserGuilds(""))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// UserGuildSettingsEdit Edits the users notification settings for a guild
// guildID   : The ID of the guild to edit the settings on
// settings  : The settings to update
func (s *Session) UserGuildSettingsEdit(guildID int64, settings *UserGuildSettingsEdit) (st *UserGuildSettings, err error) {

	body, err := s.RequestWithBucketID("PATCH", EndpointUserGuildSettings("@me", guildID), settings, nil, EndpointUserGuildSettings("", guildID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// UserChannelPermissions returns the permission of a user in a channel.
// userID    : The ID of the user to calculate permissions for.
// channelID : The ID of the channel to calculate permission for.
//
// NOTE: This function is now deprecated and will be removed in the future.
// Please see the same function inside state.go
func (s *Session) UserChannelPermissions(userID, channelID int64) (apermissions int64, err error) {
	// Try to just get permissions from state.
	apermissions, err = s.State.UserChannelPermissions(userID, channelID)
	if err == nil {
		return
	}

	// Otherwise try get as much data from state as possible, falling back to the network.
	channel, err := s.State.Channel(channelID)
	if err != nil || channel == nil {
		channel, err = s.Channel(channelID)
		if err != nil {
			return
		}
	}

	guild, err := s.State.Guild(channel.GuildID)
	if err != nil || guild == nil {
		guild, err = s.Guild(channel.GuildID)
		if err != nil {
			return
		}
	}

	if userID == guild.OwnerID {
		apermissions = PermissionAll
		return
	}

	member, err := s.State.Member(guild.ID, userID)
	if err != nil || member == nil {
		member, err = s.GuildMember(guild.ID, userID)
		if err != nil {
			return
		}
	}

	return MemberPermissions(guild, channel, member), nil
}

// Calculates the permissions for a member.
// https://support.discordapp.com/hc/en-us/articles/206141927-How-is-the-permission-hierarchy-structured-
func MemberPermissions(guild *Guild, channel *Channel, member *Member) (apermissions int64) {
	userID := member.User.ID

	if userID == guild.OwnerID {
		apermissions = PermissionAll
		return
	}

	for _, role := range guild.Roles {
		if role.ID == guild.ID {
			apermissions |= role.Permissions
			break
		}
	}

	for _, role := range guild.Roles {
		for _, roleID := range member.Roles {
			if role.ID == roleID {
				apermissions |= role.Permissions
				break
			}
		}
	}

	if apermissions&PermissionAdministrator == PermissionAdministrator {
		apermissions |= PermissionAll
		// Administrator overwrites everything, so no point in checking further
		return
	}

	if channel != nil {
		// Apply @everyone overrides from the channel.
		for _, overwrite := range channel.PermissionOverwrites {
			if guild.ID == overwrite.ID {
				apermissions &= ^overwrite.Deny
				apermissions |= overwrite.Allow
				break
			}
		}

		denies := int64(0)
		allows := int64(0)

		// Member overwrites can override role overrides, so do two passes
		for _, overwrite := range channel.PermissionOverwrites {
			for _, roleID := range member.Roles {
				if overwrite.Type == PermissionOverwriteTypeRole && roleID == overwrite.ID {
					denies |= overwrite.Deny
					allows |= overwrite.Allow
					break
				}
			}
		}

		apermissions &= ^denies
		apermissions |= allows

		for _, overwrite := range channel.PermissionOverwrites {
			if overwrite.Type == PermissionOverwriteTypeMember && overwrite.ID == userID {
				apermissions &= ^overwrite.Deny
				apermissions |= overwrite.Allow
				break
			}
		}
	}

	return apermissions
}

// ------------------------------------------------------------------------------------------------
// Functions specific to Discord Guilds
// ------------------------------------------------------------------------------------------------

// Guild returns a Guild structure of a specific Guild.
// guildID   : The ID of a Guild
func (s *Session) Guild(guildID int64) (st *Guild, err error) {
	body, err := s.RequestWithBucketID("GET", EndpointGuild(guildID), nil, nil, EndpointGuild(guildID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// Guild returns a Guild structure of a specific Guild.
// guildID   : The ID of a Guild
func (s *Session) GuildWithCounts(guildID int64) (st *Guild, err error) {
	body, err := s.RequestWithBucketID("GET", EndpointGuild(guildID)+"?with_counts=true", nil, nil, EndpointGuild(guildID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// GuildCreate creates a new Guild
// name      : A name for the Guild (2-100 characters)
func (s *Session) GuildCreate(name string) (st *Guild, err error) {

	data := struct {
		Name string `json:"name"`
	}{name}

	body, err := s.RequestWithBucketID("POST", EndpointGuildCreate, data, nil, EndpointGuildCreate)
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// GuildEdit edits a new Guild
// guildID   : The ID of a Guild
// g 		 : A GuildParams struct with the values Name, Region and VerificationLevel defined.
func (s *Session) GuildEdit(guildID int64, g GuildParams) (st *Guild, err error) {

	// Bounds checking for VerificationLevel, interval: [0, 3]
	if g.VerificationLevel != nil {
		val := *g.VerificationLevel
		if val < 0 || val > 3 {
			err = ErrVerificationLevelBounds
			return
		}
	}

	//Bounds checking for regions
	if g.Region != "" {
		isValid := false
		regions, _ := s.VoiceRegions()
		for _, r := range regions {
			if g.Region == r.ID {
				isValid = true
			}
		}
		if !isValid {
			var valid []string
			for _, r := range regions {
				valid = append(valid, r.ID)
			}
			err = fmt.Errorf("Region not a valid region (%q)", valid)
			return
		}
	}

	body, err := s.RequestWithBucketID("PATCH", EndpointGuild(guildID), g, nil, EndpointGuild(guildID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// GuildDelete deletes a Guild.
// guildID   : The ID of a Guild
func (s *Session) GuildDelete(guildID int64) (st *Guild, err error) {

	body, err := s.RequestWithBucketID("DELETE", EndpointGuild(guildID), nil, nil, EndpointGuild(guildID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// GuildLeave leaves a Guild.
// guildID   : The ID of a Guild
func (s *Session) GuildLeave(guildID int64) (err error) {

	_, err = s.RequestWithBucketID("DELETE", EndpointUserGuild("@me", guildID), nil, nil, EndpointUserGuild("", guildID))
	return
}

// GuildBans returns an array of User structures for all bans of a
// given guild.
// guildID   : The ID of a Guild.
func (s *Session) GuildBans(guildID int64) (st []*GuildBan, err error) {

	body, err := s.RequestWithBucketID("GET", EndpointGuildBans(guildID), nil, nil, EndpointGuildBans(guildID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return
}

// GuildBan returns a ban object for the given user or a 404 not found if the ban cannot be found. Requires the BAN_MEMBERS permission.
// guildID   : The ID of a Guild.
func (s *Session) GuildBan(guildID, userID int64) (st *GuildBan, err error) {

	body, err := s.RequestWithBucketID("GET", EndpointGuildBan(guildID, userID), nil, nil, EndpointGuildBan(guildID, 0)+"/")
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return
}

// GuildBanCreate bans the given user from the given guild.
// guildID   : The ID of a Guild.
// userID    : The ID of a User
// days      : The number of days of previous comments to delete.
func (s *Session) GuildBanCreate(guildID, userID int64, days int) (err error) {
	return s.GuildBanCreateWithReason(guildID, userID, "", days)
}

// GuildBanCreateWithReason bans the given user from the given guild also providing a reaso.
// guildID   : The ID of a Guild.
// userID    : The ID of a User
// reason    : The reason for this ban
// days      : The number of days of previous comments to delete.
func (s *Session) GuildBanCreateWithReason(guildID, userID int64, reason string, days int) (err error) {

	uri := EndpointGuildBan(guildID, userID)

	data := make(map[string]interface{})
	if days > 0 {
		data["delete_message_days"] = days
	}

	headers := make(map[string]string)
	if reason != "" {
		headers["X-Audit-Log-Reason"] = url.PathEscape(reason)
	}

	_, err = s.RequestWithBucketID("PUT", uri, data, headers, EndpointGuildBan(guildID, 0))
	return
}

// GuildBanDelete removes the given user from the guild bans
// guildID   : The ID of a Guild.
// userID    : The ID of a User
func (s *Session) GuildBanDelete(guildID, userID int64) (err error) {

	_, err = s.RequestWithBucketID("DELETE", EndpointGuildBan(guildID, userID), nil, nil, EndpointGuildBan(guildID, 0))
	return
}

// GuildMembers returns a list of members for a guild.
//  guildID  : The ID of a Guild.
//  after    : The id of the member to return members after
//  limit    : max number of members to return (max 1000)
func (s *Session) GuildMembers(guildID int64, after int64, limit int) (st []*Member, err error) {

	uri := EndpointGuildMembers(guildID)

	v := url.Values{}

	if after != 0 {
		v.Set("after", StrID(after))
	}

	if limit > 0 {
		v.Set("limit", strconv.Itoa(limit))
	}

	if len(v) > 0 {
		uri = fmt.Sprintf("%s?%s", uri, v.Encode())
	}

	body, err := s.RequestWithBucketID("GET", uri, nil, nil, EndpointGuildMembers(guildID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// GuildMember returns a member of a guild.
//  guildID   : The ID of a Guild.
//  userID    : The ID of a User
func (s *Session) GuildMember(guildID, userID int64) (st *Member, err error) {

	body, err := s.RequestWithBucketID("GET", EndpointGuildMember(guildID, userID), nil, nil, EndpointGuildMember(guildID, 0))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// GuildMemberAdd force joins a user to the guild.
//  accessToken   : Valid access_token for the user.
//  guildID       : The ID of a Guild.
//  userID        : The ID of a User.
//  nick          : Value to set users nickname to
//  roles         : A list of role ID's to set on the member.
//  mute          : If the user is muted.
//  deaf          : If the user is deafened.
func (s *Session) GuildMemberAdd(accessToken string, guildID, userID int64, nick string, roles []int64, mute, deaf bool) (err error) {

	data := struct {
		AccessToken string  `json:"access_token"`
		Nick        string  `json:"nick,omitempty"`
		Roles       IDSlice `json:"roles,omitempty"`
		Mute        bool    `json:"mute,omitempty"`
		Deaf        bool    `json:"deaf,omitempty"`
	}{accessToken, nick, roles, mute, deaf}

	_, err = s.RequestWithBucketID("PUT", EndpointGuildMember(guildID, userID), data, nil, EndpointGuildMember(guildID, 0))
	if err != nil {
		return err
	}

	return err
}

// GuildMemberDelete removes the given user from the given guild.
// guildID   : The ID of a Guild.
// userID    : The ID of a User
func (s *Session) GuildMemberDelete(guildID, userID int64) (err error) {

	return s.GuildMemberDeleteWithReason(guildID, userID, "")
}

// GuildMemberDeleteWithReason removes the given user from the given guild.
// guildID   : The ID of a Guild.
// userID    : The ID of a User
// reason    : The reason for the kick
func (s *Session) GuildMemberDeleteWithReason(guildID, userID int64, reason string) (err error) {

	uri := EndpointGuildMember(guildID, userID)
	if reason != "" {
		uri += "?reason=" + url.QueryEscape(reason)
	}

	_, err = s.RequestWithBucketID("DELETE", uri, nil, nil, EndpointGuildMember(guildID, 0))
	return
}

// GuildMemberEdit edits the roles of a member.
// guildID  : The ID of a Guild.
// userID   : The ID of a User.
// roles    : A list of role ID's to set on the member.
func (s *Session) GuildMemberEdit(guildID, userID int64, roles []string) (err error) {

	data := struct {
		Roles []string `json:"roles"`
	}{roles}

	_, err = s.RequestWithBucketID("PATCH", EndpointGuildMember(guildID, userID), data, nil, EndpointGuildMember(guildID, 0))
	if err != nil {
		return
	}

	return
}

// GuildMemberMove moves a guild member from one voice channel to another/none
//  guildID   : The ID of a Guild.
//  userID    : The ID of a User.
//  channelID : The ID of a channel to move user to. Use 0 to disconnect the member.
// NOTE : I am not entirely set on the name of this function and it may change
// prior to the final 1.0.0 release of Discordgo
func (s *Session) GuildMemberMove(guildID, userID, channelID int64) (err error) {

	data := struct {
		ChannelID NullableID `json:"channel_id,string"`
	}{NullableID(channelID)}

	_, err = s.RequestWithBucketID("PATCH", EndpointGuildMember(guildID, userID), data, nil, EndpointGuildMember(guildID, 0))
	if err != nil {
		return
	}

	return
}

// GuildMemberNickname updates the nickname of a guild member
// guildID   : The ID of a guild
// userID    : The ID of a user or "@me" which is a shortcut of the current user ID
// nickname  : The new nickname
func (s *Session) GuildMemberNickname(guildID, userID int64, nickname string) (err error) {

	data := struct {
		Nick string `json:"nick"`
	}{nickname}

	_, err = s.RequestWithBucketID("PATCH", EndpointGuildMember(guildID, userID), data, nil, EndpointGuildMember(guildID, 0))
	return
}

// GuildMemberTimeoutWithReason times out a guild member with a mandatory reason
//  guildID   : The ID of a Guild.
//  userID    : The ID of a User.
//  until     : The timestamp for how long a member should be timed out.
//              Set to nil to remove timeout.
// reason    : The reason for the timeout
func (s *Session) GuildMemberTimeoutWithReason(guildID int64, userID int64, until *time.Time, reason string) (err error) {
	data := struct {
		TimeoutExpiresAt *time.Time `json:"communication_disabled_until"`
	}{until}

	headers := make(map[string]string)
	if reason != "" {
		headers["X-Audit-Log-Reason"] = url.PathEscape(reason)
	}
	_, err = s.RequestWithBucketID("PATCH", EndpointGuildMember(guildID, userID), data, headers, EndpointGuildMember(guildID, 0))
	return
}

// GuildMemberTimeout times out a guild member
//  guildID   : The ID of a Guild.
//  userID    : The ID of a User.
//  until     : The timestamp for how long a member should be timed out.
//              Set to nil to remove timeout.
func (s *Session) GuildMemberTimeout(guildID int64, userID int64, until *time.Time, reason string) (err error) {
	return s.GuildMemberTimeoutWithReason(guildID, userID, until, reason)
}

// GuildMemberNicknameMe updates the nickname the current user
// guildID   : The ID of a guild
// nickname  : The new nickname
func (s *Session) GuildMemberNicknameMe(guildID int64, nickname string) (err error) {

	data := struct {
		Nick string `json:"nick"`
	}{nickname}

	_, err = s.RequestWithBucketID("PATCH", EndpointGuildMemberMe(guildID)+"/nick", data, nil, EndpointGuildMember(guildID, 0))
	return
}

// GuildMemberRoleAdd adds the specified role to a given member
//  guildID   : The ID of a Guild.
//  userID    : The ID of a User.
//  roleID 	  : The ID of a Role to be assigned to the user.
func (s *Session) GuildMemberRoleAdd(guildID, userID, roleID int64) (err error) {

	_, err = s.RequestWithBucketID("PUT", EndpointGuildMemberRole(guildID, userID, roleID), nil, nil, EndpointGuildMemberRole(guildID, 0, 0))

	return
}

// GuildMemberRoleRemove removes the specified role to a given member
//  guildID   : The ID of a Guild.
//  userID    : The ID of a User.
//  roleID 	  : The ID of a Role to be removed from the user.
func (s *Session) GuildMemberRoleRemove(guildID, userID, roleID int64) (err error) {

	_, err = s.RequestWithBucketID("DELETE", EndpointGuildMemberRole(guildID, userID, roleID), nil, nil, EndpointGuildMemberRole(guildID, 0, 0))

	return
}

// GuildChannels returns an array of Channel structures for all channels of a
// given guild.
// guildID   : The ID of a Guild.
func (s *Session) GuildChannels(guildID int64) (st []*Channel, err error) {

	body, err := s.request("GET", EndpointGuildChannels(guildID), "", nil, nil, EndpointGuildChannels(guildID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return
}

// GuildChannelCreate creates a new channel in the given guild
// guildID   : The ID of a Guild.
// name      : Name of the channel (2-100 chars length)
// ctype     : Type of the channel
func (s *Session) GuildChannelCreate(guildID int64, name string, ctype ChannelType) (st *Channel, err error) {

	data := struct {
		Name string      `json:"name"`
		Type ChannelType `json:"type"`
	}{name, ctype}

	body, err := s.RequestWithBucketID("POST", EndpointGuildChannels(guildID), data, nil, EndpointGuildChannels(guildID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// GuildChannelCreateWithOverwrites creates a new channel in the given guild
// guildID     : The ID of a Guild.
// name        : Name of the channel (2-100 chars length)
// ctype       : Type of the channel
// overwrites  : slice of permission overwrites
func (s *Session) GuildChannelCreateWithOverwrites(guildID int64, name string, ctype ChannelType, parentID int64, overwrites []*PermissionOverwrite) (st *Channel, err error) {

	data := struct {
		Name                 string                 `json:"name"`
		Type                 ChannelType            `json:"type"`
		ParentID             int64                  `json:"parent_id,string"`
		PermissionOverwrites []*PermissionOverwrite `json:"permission_overwrites"`
	}{name, ctype, parentID, overwrites}

	body, err := s.RequestWithBucketID("POST", EndpointGuildChannels(guildID), data, nil, EndpointGuildChannels(guildID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// GuildChannelsReorder updates the order of channels in a guild
// guildID   : The ID of a Guild.
// channels  : Updated channels.
func (s *Session) GuildChannelsReorder(guildID int64, channels []*Channel) (err error) {

	data := make([]struct {
		ID       int64 `json:"id,string"`
		Position int   `json:"position"`
	}, len(channels))

	for i, c := range channels {
		data[i].ID = c.ID
		data[i].Position = c.Position
	}

	_, err = s.RequestWithBucketID("PATCH", EndpointGuildChannels(guildID), data, nil, EndpointGuildChannels(guildID))
	return
}

// GuildInvites returns an array of Invite structures for the given guild
// guildID   : The ID of a Guild.
func (s *Session) GuildInvites(guildID int64) (st []*Invite, err error) {
	body, err := s.RequestWithBucketID("GET", EndpointGuildInvites(guildID), nil, nil, EndpointGuildInvites(guildID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// GuildRoles returns all roles for a given guild.
// guildID   : The ID of a Guild.
func (s *Session) GuildRoles(guildID int64) (st []*Role, err error) {

	body, err := s.RequestWithBucketID("GET", EndpointGuildRoles(guildID), nil, nil, EndpointGuildRoles(guildID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return // TODO return pointer
}

// GuildRoleCreate returns a new Guild Role.
// guildID: The ID of a Guild.
func (s *Session) GuildRoleCreate(guildID int64) (st *Role, err error) {

	body, err := s.RequestWithBucketID("POST", EndpointGuildRoles(guildID), nil, nil, EndpointGuildRoles(guildID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return
}

// GuildRoleCreateComplex returns a new Guild Role.
// guildID: The ID of a Guild.
func (s *Session) GuildRoleCreateComplex(guildID int64, roleCreate RoleCreate) (st *Role, err error) {

	body, err := s.RequestWithBucketID("POST", EndpointGuildRoles(guildID), roleCreate, nil, EndpointGuildRoles(guildID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return
}

// GuildRoleEdit updates an existing Guild Role with new values
// guildID   : The ID of a Guild.
// roleID    : The ID of a Role.
// name      : The name of the Role.
// color     : The color of the role (decimal, not hex).
// hoist     : Whether to display the role's users separately.
// perm      : The permissions for the role.
// mention   : Whether this role is mentionable
func (s *Session) GuildRoleEdit(guildID, roleID int64, name string, color int, hoist bool, perm int64, mention bool) (st *Role, err error) {

	// Prevent sending a color int that is too big.
	if color > 0xFFFFFF {
		err = fmt.Errorf("color value cannot be larger than 0xFFFFFF")
		return nil, err
	}

	data := struct {
		Name        string `json:"name"`               // The role's name (overwrites existing)
		Color       int    `json:"color"`              // The color the role should have (as a decimal, not hex)
		Hoist       bool   `json:"hoist"`              // Whether to display the role's users separately
		Permissions int64  `json:"permissions,string"` // The overall permissions number of the role (overwrites existing)
		Mentionable bool   `json:"mentionable"`        // Whether this role is mentionable
	}{name, color, hoist, perm, mention}

	body, err := s.RequestWithBucketID("PATCH", EndpointGuildRole(guildID, roleID), data, nil, EndpointGuildRole(guildID, 0))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return
}

// GuildRoleReorder reoders guild roles
// guildID   : The ID of a Guild.
// roles     : A list of ordered roles.
func (s *Session) GuildRoleReorder(guildID int64, roles []*Role) (st []*Role, err error) {

	body, err := s.RequestWithBucketID("PATCH", EndpointGuildRoles(guildID), roles, nil, EndpointGuildRoles(guildID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return
}

// GuildRoleDelete deletes an existing role.
// guildID   : The ID of a Guild.
// roleID    : The ID of a Role.
func (s *Session) GuildRoleDelete(guildID, roleID int64) (err error) {

	_, err = s.RequestWithBucketID("DELETE", EndpointGuildRole(guildID, roleID), nil, nil, EndpointGuildRole(guildID, 0))

	return
}

// GuildPruneCount Returns the number of members that would be removed in a prune operation.
// Requires 'KICK_MEMBER' permission.
// guildID	: The ID of a Guild.
// days		: The number of days to count prune for (1 or more).
func (s *Session) GuildPruneCount(guildID int64, days uint32) (count uint32, err error) {
	count = 0

	if days <= 0 {
		err = ErrPruneDaysBounds
		return
	}

	p := struct {
		Pruned uint32 `json:"pruned"`
	}{}

	uri := EndpointGuildPrune(guildID) + fmt.Sprintf("?days=%d", days)
	body, err := s.RequestWithBucketID("GET", uri, nil, nil, EndpointGuildPrune(guildID))
	if err != nil {
		return
	}

	err = unmarshal(body, &p)
	if err != nil {
		return
	}

	count = p.Pruned

	return
}

// GuildPrune Begin as prune operation. Requires the 'KICK_MEMBERS' permission.
// Returns an object with one 'pruned' key indicating the number of members that were removed in the prune operation.
// guildID	: The ID of a Guild.
// days		: The number of days to count prune for (1 or more).
func (s *Session) GuildPrune(guildID int64, days uint32) (count uint32, err error) {

	count = 0

	if days <= 0 {
		err = ErrPruneDaysBounds
		return
	}

	data := struct {
		days uint32
	}{days}

	p := struct {
		Pruned uint32 `json:"pruned"`
	}{}

	body, err := s.RequestWithBucketID("POST", EndpointGuildPrune(guildID), data, nil, EndpointGuildPrune(guildID))
	if err != nil {
		return
	}

	err = unmarshal(body, &p)
	if err != nil {
		return
	}

	count = p.Pruned

	return
}

// GuildIntegrations returns an array of Integrations for a guild.
// guildID   : The ID of a Guild.
func (s *Session) GuildIntegrations(guildID int64) (st []*Integration, err error) {

	body, err := s.RequestWithBucketID("GET", EndpointGuildIntegrations(guildID), nil, nil, EndpointGuildIntegrations(guildID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return
}

// GuildIntegrationCreate creates a Guild Integration.
// guildID          : The ID of a Guild.
// integrationType  : The Integration type.
// integrationID    : The ID of an integration.
func (s *Session) GuildIntegrationCreate(guildID int64, integrationType string, integrationID int64) (err error) {

	data := struct {
		Type string `json:"type"`
		ID   int64  `json:"id,string"`
	}{integrationType, integrationID}

	_, err = s.RequestWithBucketID("POST", EndpointGuildIntegrations(guildID), data, nil, EndpointGuildIntegrations(guildID))
	return
}

// GuildIntegrationEdit edits a Guild Integration.
// guildID              : The ID of a Guild.
// integrationType      : The Integration type.
// integrationID        : The ID of an integration.
// expireBehavior	      : The behavior when an integration subscription lapses (see the integration object documentation).
// expireGracePeriod    : Period (in seconds) where the integration will ignore lapsed subscriptions.
// enableEmoticons	    : Whether emoticons should be synced for this integration (twitch only currently).
func (s *Session) GuildIntegrationEdit(guildID, integrationID int64, expireBehavior, expireGracePeriod int, enableEmoticons bool) (err error) {

	data := struct {
		ExpireBehavior    int  `json:"expire_behavior"`
		ExpireGracePeriod int  `json:"expire_grace_period"`
		EnableEmoticons   bool `json:"enable_emoticons"`
	}{expireBehavior, expireGracePeriod, enableEmoticons}

	_, err = s.RequestWithBucketID("PATCH", EndpointGuildIntegration(guildID, integrationID), data, nil, EndpointGuildIntegration(guildID, 0))
	return
}

// GuildIntegrationDelete removes the given integration from the Guild.
// guildID          : The ID of a Guild.
// integrationID    : The ID of an integration.
func (s *Session) GuildIntegrationDelete(guildID, integrationID int64) (err error) {

	_, err = s.RequestWithBucketID("DELETE", EndpointGuildIntegration(guildID, integrationID), nil, nil, EndpointGuildIntegration(guildID, 0))
	return
}

// GuildIntegrationSync syncs an integration.
// guildID          : The ID of a Guild.
// integrationID    : The ID of an integration.
func (s *Session) GuildIntegrationSync(guildID, integrationID int64) (err error) {

	_, err = s.RequestWithBucketID("POST", EndpointGuildIntegrationSync(guildID, integrationID), nil, nil, EndpointGuildIntegration(guildID, 0))
	return
}

// GuildIcon returns an image.Image of a guild icon.
// guildID   : The ID of a Guild.
func (s *Session) GuildIcon(guildID int64) (img image.Image, err error) {
	g, err := s.Guild(guildID)
	if err != nil {
		return
	}

	if g.Icon == "" {
		err = ErrGuildNoIcon
		return
	}

	body, err := s.RequestWithBucketID("GET", EndpointGuildIcon(guildID, g.Icon), nil, nil, EndpointGuildIcon(guildID, ""))
	if err != nil {
		return
	}

	img, _, err = image.Decode(bytes.NewReader(body))
	return
}

// GuildSplash returns an image.Image of a guild splash image.
// guildID   : The ID of a Guild.
func (s *Session) GuildSplash(guildID int64) (img image.Image, err error) {
	g, err := s.Guild(guildID)
	if err != nil {
		return
	}

	if g.Splash == "" {
		err = ErrGuildNoSplash
		return
	}

	body, err := s.RequestWithBucketID("GET", EndpointGuildSplash(guildID, g.Splash), nil, nil, EndpointGuildSplash(guildID, ""))
	if err != nil {
		return
	}

	img, _, err = image.Decode(bytes.NewReader(body))
	return
}

// GuildEmbed returns the embed for a Guild.
// guildID   : The ID of a Guild.
func (s *Session) GuildEmbed(guildID int64) (st *GuildEmbed, err error) {

	body, err := s.RequestWithBucketID("GET", EndpointGuildEmbed(guildID), nil, nil, EndpointGuildEmbed(guildID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// GuildEmbedEdit returns the embed for a Guild.
// guildID   : The ID of a Guild.
func (s *Session) GuildEmbedEdit(guildID int64, enabled bool, channelID int64) (err error) {

	data := GuildEmbed{enabled, channelID}

	_, err = s.RequestWithBucketID("PATCH", EndpointGuildEmbed(guildID), data, nil, EndpointGuildEmbed(guildID))
	return
}

// GuildAuditLog returns the audit log for a Guild.
// guildID     : The ID of a Guild.
// userID      : If provided the log will be filtered for the given ID.
// beforeID    : If provided all log entries returned will be before the given ID.
// actionType  : If provided the log will be filtered for the given Action Type.
// limit       : The number messages that can be returned. (default 50, min 1, max 100)
func (s *Session) GuildAuditLog(guildID, userID, beforeID int64, actionType, limit int) (st *GuildAuditLog, err error) {

	uri := EndpointGuildAuditLogs(guildID)

	v := url.Values{}
	if userID != 0 {
		v.Set("user_id", StrID(userID))
	}
	if beforeID != 0 {
		v.Set("before", StrID(beforeID))
	}
	if actionType > 0 {
		v.Set("action_type", strconv.Itoa(actionType))
	}
	if limit > 0 {
		v.Set("limit", strconv.Itoa(limit))
	}
	if len(v) > 0 {
		uri = fmt.Sprintf("%s?%s", uri, v.Encode())
	}

	body, err := s.RequestWithBucketID("GET", uri, nil, nil, EndpointGuildAuditLogs(guildID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// GuildEmojiCreate creates a new emoji
// guildID : The ID of a Guild.
// name    : The Name of the Emoji.
// image   : The base64 encoded emoji image, has to be smaller than 256KB.
// roles   : The roles for which this emoji will be whitelisted, can be nil.
func (s *Session) GuildEmojiCreate(guildID int64, name, image string, roles []int64) (emoji *Emoji, err error) {

	data := struct {
		Name  string  `json:"name"`
		Image string  `json:"image"`
		Roles IDSlice `json:"roles,omitempty"`
	}{name, image, roles}

	body, err := s.RequestWithBucketID("POST", EndpointGuildEmojis(guildID), data, nil, EndpointGuildEmojis(guildID))
	if err != nil {
		return
	}

	err = unmarshal(body, &emoji)
	return
}

// GuildEmojiEdit modifies an emoji
// guildID : The ID of a Guild.
// emojiID : The ID of an Emoji.
// name    : The Name of the Emoji.
// roles   : The roles for which this emoji will be whitelisted, can be nil.
func (s *Session) GuildEmojiEdit(guildID, emojiID int64, name string, roles []int64) (emoji *Emoji, err error) {

	data := struct {
		Name  string  `json:"name"`
		Roles IDSlice `json:"roles,omitempty"`
	}{name, roles}

	body, err := s.RequestWithBucketID("PATCH", EndpointGuildEmoji(guildID, emojiID), data, nil, EndpointGuildEmojis(guildID))
	if err != nil {
		return
	}

	err = unmarshal(body, &emoji)
	return
}

// GuildEmojiDelete deletes an Emoji.
// guildID : The ID of a Guild.
// emojiID : The ID of an Emoji.
func (s *Session) GuildEmojiDelete(guildID, emojiID int64) (err error) {

	_, err = s.RequestWithBucketID("DELETE", EndpointGuildEmoji(guildID, emojiID), nil, nil, EndpointGuildEmojis(guildID))
	return
}

// ------------------------------------------------------------------------------------------------
// Functions specific to Discord Channels
// ------------------------------------------------------------------------------------------------

// Channel returns a Channel structure of a specific Channel.
// channelID  : The ID of the Channel you want returned.
func (s *Session) Channel(channelID int64) (st *Channel, err error) {
	body, err := s.RequestWithBucketID("GET", EndpointChannel(channelID), nil, nil, EndpointChannel(channelID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// ChannelEdit edits the given channel
// channelID  : The ID of a Channel
// name       : The new name to assign the channel.
func (s *Session) ChannelEdit(channelID int64, name string) (*Channel, error) {
	return s.ChannelEditComplex(channelID, &ChannelEdit{
		Name: name,
	})
}

// ChannelEditComplex edits an existing channel, replacing the parameters entirely with ChannelEdit struct
// channelID  : The ID of a Channel
// data          : The channel struct to send
func (s *Session) ChannelEditComplex(channelID int64, data *ChannelEdit) (st *Channel, err error) {
	body, err := s.RequestWithBucketID("PATCH", EndpointChannel(channelID), data, nil, EndpointChannel(channelID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// ChannelDelete deletes the given channel
// channelID  : The ID of a Channel
func (s *Session) ChannelDelete(channelID int64) (st *Channel, err error) {

	body, err := s.RequestWithBucketID("DELETE", EndpointChannel(channelID), nil, nil, EndpointChannel(channelID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// ChannelTyping broadcasts to all members that authenticated user is typing in
// the given channel.
// channelID  : The ID of a Channel
func (s *Session) ChannelTyping(channelID int64) (err error) {

	_, err = s.RequestWithBucketID("POST", EndpointChannelTyping(channelID), nil, nil, EndpointChannelTyping(channelID))
	return
}

// ChannelMessages returns an array of Message structures for messages within
// a given channel.
// channelID : The ID of a Channel.
// limit     : The number messages that can be returned. (max 100)
// beforeID  : If provided all messages returned will be before given ID.
// afterID   : If provided all messages returned will be after given ID.
// aroundID  : If provided all messages returned will be around given ID.
func (s *Session) ChannelMessages(channelID int64, limit int, beforeID, afterID, aroundID int64) (st []*Message, err error) {

	uri := EndpointChannelMessages(channelID)

	v := url.Values{}
	if limit > 0 {
		v.Set("limit", strconv.Itoa(limit))
	}
	if afterID != 0 {
		v.Set("after", StrID(afterID))
	}
	if beforeID != 0 {
		v.Set("before", StrID(beforeID))
	}
	if aroundID != 0 {
		v.Set("around", StrID(aroundID))
	}

	if len(v) > 0 {
		uri = fmt.Sprintf("%s?%s", uri, v.Encode())
	}

	body, err := s.RequestWithBucketID("GET", uri, nil, nil, EndpointChannelMessages(channelID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// ChannelMessage gets a single message by ID from a given channel.
// channeld  : The ID of a Channel
// messageID : the ID of a Message
func (s *Session) ChannelMessage(channelID, messageID int64) (st *Message, err error) {

	response, err := s.RequestWithBucketID("GET", EndpointChannelMessage(channelID, messageID), nil, nil, EndpointChannelMessage(channelID, 0))
	if err != nil {
		return
	}

	err = unmarshal(response, &st)
	return
}

// ChannelMessageAck acknowledges and marks the given message as read
// channeld  : The ID of a Channel
// messageID : the ID of a Message
// lastToken : token returned by last ack
func (s *Session) ChannelMessageAck(channelID, messageID int64, lastToken string) (st *Ack, err error) {

	body, err := s.RequestWithBucketID("POST", EndpointChannelMessageAck(channelID, messageID), &Ack{Token: lastToken}, nil, EndpointChannelMessageAck(channelID, 0))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// ChannelMessageSend sends a message to the given channel.
// channelID : The ID of a Channel.
// content   : The message to send.
func (s *Session) ChannelMessageSend(channelID int64, content string) (*Message, error) {
	return s.ChannelMessageSendComplex(channelID, &MessageSend{
		Content:         content,
		AllowedMentions: AllowedMentions{},
	})
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

// ChannelMessageSendComplex sends a message to the given channel.
// channelID : The ID of a Channel.
// data      : The message struct to send.
func (s *Session) ChannelMessageSendComplex(channelID int64, msg *MessageSend) (st *Message, err error) {
	msg.Embeds = ValidateComplexMessageEmbeds(msg.Embeds)
	endpoint := EndpointChannelMessages(channelID)

	// TODO: Remove this when compatibility is not required.
	files := msg.Files
	if msg.File != nil {
		if files == nil {
			files = []*File{msg.File}
		} else {
			err = fmt.Errorf("cannot specify both File and Files")
			return
		}
	}

	var response []byte
	if len(files) > 0 {
		body := &bytes.Buffer{}
		bodywriter := multipart.NewWriter(body)

		var payload []byte
		payload, err = json.Marshal(msg)
		if err != nil {
			return
		}

		var p io.Writer

		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", `form-data; name="payload_json"`)
		h.Set("Content-Type", "application/json")

		p, err = bodywriter.CreatePart(h)
		if err != nil {
			return
		}

		if _, err = p.Write(payload); err != nil {
			return
		}

		for i, file := range files {
			h := make(textproto.MIMEHeader)
			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file%d"; filename="%s"`, i, quoteEscaper.Replace(file.Name)))
			contentType := file.ContentType
			if contentType == "" {
				contentType = "application/octet-stream"
			}
			h.Set("Content-Type", contentType)

			p, err = bodywriter.CreatePart(h)
			if err != nil {
				return
			}

			if _, err = io.Copy(p, file.Reader); err != nil {
				return
			}
		}

		err = bodywriter.Close()
		if err != nil {
			return
		}

		response, err = s.request("POST", endpoint, bodywriter.FormDataContentType(), body.Bytes(), nil, endpoint)
	} else {
		response, err = s.RequestWithBucketID("POST", endpoint, msg, nil, endpoint)
	}
	if err != nil {
		return
	}

	err = unmarshal(response, &st)
	return
}

// ChannelMessageSendTTS sends a message to the given channel with Text to Speech.
// channelID : The ID of a Channel.
// content   : The message to send.
func (s *Session) ChannelMessageSendTTS(channelID int64, content string) (*Message, error) {
	return s.ChannelMessageSendComplex(channelID, &MessageSend{
		Content: content,
		TTS:     true,
	})
}

// ChannelMessageSendEmbeds sends a message to the given channel with list of embedded data.
// channelID : The ID of a Channel.
// embed     : The list embed data to send.
func (s *Session) ChannelMessageSendEmbedList(channelID int64, embeds []*MessageEmbed) (*Message, error) {
	return s.ChannelMessageSendComplex(channelID, &MessageSend{
		Embeds: embeds,
	})
}

// ChannelMessageSendEmbed sends a message to the given channel with embedded data.
// channelID : The ID of a Channel.
// embed     : The embed data to send.
func (s *Session) ChannelMessageSendEmbed(channelID int64, embed *MessageEmbed) (*Message, error) {
	return s.ChannelMessageSendComplex(channelID, &MessageSend{
		Embeds: []*MessageEmbed{embed},
	})
}

// ChannelMessageSendEmbeds sends a message to the given channel with multiple embedded data.
// channelID : The ID of a Channel.
// embeds    : The embeds data to send.
func (s *Session) ChannelMessageSendEmbeds(channelID int64, embeds []*MessageEmbed) (*Message, error) {
	return s.ChannelMessageSendComplex(channelID, &MessageSend{
		Embeds: embeds,
	})
}

// ChannelMessageSendReply sends a message to the given channel with reference data.
// channelID : The ID of a Channel.
// content   : The message to send.
// reference : The message reference to send.
func (s *Session) ChannelMessageSendReply(channelID int64, content string, reference *MessageReference) (*Message, error) {
	if reference == nil {
		return nil, fmt.Errorf("reply attempted with nil message reference")
	}
	return s.ChannelMessageSendComplex(channelID, &MessageSend{
		Content:   content,
		Reference: reference,
	})
}

// ChannelMessageEdit edits an existing message, replacing it entirely with
// the given content.
// channelID  : The ID of a Channel
// messageID  : The ID of a Message
// content    : The contents of the message
func (s *Session) ChannelMessageEdit(channelID, messageID int64, content string) (*Message, error) {
	return s.ChannelMessageEditComplex(NewMessageEdit(channelID, messageID).SetContent(content))
}

// ChannelMessageEditComplex edits an existing message, replacing it entirely with
// the given MessageEdit struct
func (s *Session) ChannelMessageEditComplex(msg *MessageEdit) (st *Message, err error) {
	msg.Embeds = ValidateComplexMessageEmbeds(msg.Embeds)
	response, err := s.RequestWithBucketID("PATCH", EndpointChannelMessage(msg.Channel, msg.ID), msg, nil, EndpointChannelMessage(msg.Channel, 0))
	if err != nil {
		return
	}

	err = unmarshal(response, &st)
	return
}

// ChannelMessageEditEmbed edits an existing message with embedded data.
// channelID : The ID of a Channel
// messageID : The ID of a Message
// embed     : The embed data to send
func (s *Session) ChannelMessageEditEmbed(channelID, messageID int64, embed *MessageEmbed) (*Message, error) {
	return s.ChannelMessageEditComplex(NewMessageEdit(channelID, messageID).SetEmbeds([]*MessageEmbed{embed}))
}

// ChannelMessageEditEmbeds edits an existing message with a list of embedded data.
// channelID : The ID of a Channel
// messageID : The ID of a Message
// embeds     : The list of embed data to send
func (s *Session) ChannelMessageEditEmbedList(channelID, messageID int64, embeds []*MessageEmbed) (*Message, error) {
	return s.ChannelMessageEditComplex(NewMessageEdit(channelID, messageID).SetEmbeds(embeds))
}

// ChannelMessageDelete deletes a message from the Channel.
func (s *Session) ChannelMessageDelete(channelID, messageID int64) (err error) {

	_, err = s.RequestWithBucketID("DELETE", EndpointChannelMessage(channelID, messageID), nil, nil, EndpointChannelMessage(channelID, 0))
	return
}

// ChannelMessagesBulkDelete bulk deletes the messages from the channel for the provided messageIDs.
// If only one messageID is in the slice call channelMessageDelete function.
// If the slice is empty do nothing.
// channelID : The ID of the channel for the messages to delete.
// messages  : The IDs of the messages to be deleted. A slice of message IDs. A maximum of 100 messages.
func (s *Session) ChannelMessagesBulkDelete(channelID int64, messages []int64) (err error) {

	if len(messages) == 0 {
		return
	}

	if len(messages) == 1 {
		err = s.ChannelMessageDelete(channelID, messages[0])
		return
	}

	if len(messages) > 100 {
		messages = messages[:100]
	}

	data := struct {
		Messages IDSlice `json:"messages"`
	}{messages}

	_, err = s.RequestWithBucketID("POST", EndpointChannelMessagesBulkDelete(channelID), data, nil, EndpointChannelMessagesBulkDelete(channelID))
	return
}

// ChannelMessagePin pins a message within a given channel.
// channelID: The ID of a channel.
// messageID: The ID of a message.
func (s *Session) ChannelMessagePin(channelID, messageID int64) (err error) {

	_, err = s.RequestWithBucketID("PUT", EndpointChannelMessagePin(channelID, messageID), nil, nil, EndpointChannelMessagePin(channelID, 0))
	return
}

// ChannelMessageUnpin unpins a message within a given channel.
// channelID: The ID of a channel.
// messageID: The ID of a message.
func (s *Session) ChannelMessageUnpin(channelID, messageID int64) (err error) {

	_, err = s.RequestWithBucketID("DELETE", EndpointChannelMessagePin(channelID, messageID), nil, nil, EndpointChannelMessagePin(channelID, 0))
	return
}

// ChannelMessagesPinned returns an array of Message structures for pinned messages
// within a given channel
// channelID : The ID of a Channel.
func (s *Session) ChannelMessagesPinned(channelID int64) (st []*Message, err error) {

	body, err := s.RequestWithBucketID("GET", EndpointChannelMessagesPins(channelID), nil, nil, EndpointChannelMessagesPins(channelID))

	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// ChannelFileSend sends a file to the given channel.
// channelID : The ID of a Channel.
// name: The name of the file.
// io.Reader : A reader for the file contents.
func (s *Session) ChannelFileSend(channelID int64, name string, r io.Reader) (*Message, error) {
	return s.ChannelMessageSendComplex(channelID, &MessageSend{File: &File{Name: name, Reader: r}})
}

// ChannelFileSendWithMessage sends a file to the given channel with an message.
// DEPRECATED. Use ChannelMessageSendComplex instead.
// channelID : The ID of a Channel.
// content: Optional Message content.
// name: The name of the file.
// io.Reader : A reader for the file contents.
func (s *Session) ChannelFileSendWithMessage(channelID int64, content string, name string, r io.Reader) (*Message, error) {
	return s.ChannelMessageSendComplex(channelID, &MessageSend{File: &File{Name: name, Reader: r}, Content: content})
}

// ChannelInvites returns an array of Invite structures for the given channel
// channelID   : The ID of a Channel
func (s *Session) ChannelInvites(channelID int64) (st []*Invite, err error) {

	body, err := s.RequestWithBucketID("GET", EndpointChannelInvites(channelID), nil, nil, EndpointChannelInvites(channelID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// ChannelInviteCreate creates a new invite for the given channel.
// channelID   : The ID of a Channel
// i           : An Invite struct with the values MaxAge, MaxUses and Temporary defined.
func (s *Session) ChannelInviteCreate(channelID int64, i Invite) (st *Invite, err error) {

	data := struct {
		MaxAge    int  `json:"max_age"`
		MaxUses   int  `json:"max_uses"`
		Temporary bool `json:"temporary"`
		Unique    bool `json:"unique"`
	}{i.MaxAge, i.MaxUses, i.Temporary, i.Unique}

	body, err := s.RequestWithBucketID("POST", EndpointChannelInvites(channelID), data, nil, EndpointChannelInvites(channelID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// ChannelPermissionSet creates a Permission Override for the given channel.
// NOTE: This func name may changed.  Using Set instead of Create because
// you can both create a new override or update an override with this function.
func (s *Session) ChannelPermissionSet(channelID, targetID int64, targetType PermissionOverwriteType, allow, deny int64) (err error) {

	data := struct {
		ID    int64                   `json:"id,string"`
		Type  PermissionOverwriteType `json:"type,string"`
		Allow int64                   `json:"allow"`
		Deny  int64                   `json:"deny"`
	}{targetID, targetType, allow, deny}

	_, err = s.RequestWithBucketID("PUT", EndpointChannelPermission(channelID, targetID), data, nil, EndpointChannelPermission(channelID, 0))
	return
}

// ChannelPermissionDelete deletes a specific permission override for the given channel.
// NOTE: Name of this func may change.
func (s *Session) ChannelPermissionDelete(channelID, targetID int64) (err error) {

	_, err = s.RequestWithBucketID("DELETE", EndpointChannelPermission(channelID, targetID), nil, nil, EndpointChannelPermission(channelID, 0))
	return
}

// ------------------------------------------------------------------------------------------------
// Functions specific to Discord Invites
// ------------------------------------------------------------------------------------------------

// Invite returns an Invite structure of the given invite
// inviteID : The invite code
func (s *Session) Invite(inviteID string) (st *Invite, err error) {

	body, err := s.RequestWithBucketID("GET", EndpointInvite(inviteID), nil, nil, EndpointInvite(""))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// InviteWithCounts returns an Invite structure of the given invite including approximate member counts
// inviteID : The invite code
func (s *Session) InviteWithCounts(inviteID string) (st *Invite, err error) {

	body, err := s.RequestWithBucketID("GET", EndpointInvite(inviteID)+"?with_counts=true", nil, nil, EndpointInvite(""))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// InviteDelete deletes an existing invite
// inviteID   : the code of an invite
func (s *Session) InviteDelete(inviteID string) (st *Invite, err error) {

	body, err := s.RequestWithBucketID("DELETE", EndpointInvite(inviteID), nil, nil, EndpointInvite(""))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// InviteAccept accepts an Invite to a Guild or Channel
// inviteID : The invite code
func (s *Session) InviteAccept(inviteID string) (st *Invite, err error) {

	body, err := s.RequestWithBucketID("POST", EndpointInvite(inviteID), nil, nil, EndpointInvite(""))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// ------------------------------------------------------------------------------------------------
// Functions specific to Discord Voice
// ------------------------------------------------------------------------------------------------

// VoiceRegions returns the voice server regions
func (s *Session) VoiceRegions() (st []*VoiceRegion, err error) {

	body, err := s.RequestWithBucketID("GET", EndpointVoiceRegions, nil, nil, EndpointVoiceRegions)
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// VoiceICE returns the voice server ICE information
func (s *Session) VoiceICE() (st *VoiceICE, err error) {

	body, err := s.RequestWithBucketID("GET", EndpointVoiceIce, nil, nil, EndpointVoiceIce)
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// ------------------------------------------------------------------------------------------------
// Functions specific to Discord Websockets
// ------------------------------------------------------------------------------------------------

// Gateway returns the websocket Gateway address
func (s *Session) Gateway() (gateway string, err error) {

	response, err := s.RequestWithBucketID("GET", EndpointGateway, nil, nil, EndpointGateway)
	if err != nil {
		return
	}

	temp := struct {
		URL string `json:"url"`
	}{}

	err = unmarshal(response, &temp)
	if err != nil {
		return
	}

	gateway = temp.URL

	// Ensure the gateway always has a trailing slash.
	// MacOS will fail to connect if we add query params without a trailing slash on the base domain.
	if !strings.HasSuffix(gateway, "/") {
		gateway += "/"
	}

	return
}

// GatewayBot returns the websocket Gateway address and the recommended number of shards
func (s *Session) GatewayBot() (st *GatewayBotResponse, err error) {

	response, err := s.RequestWithBucketID("GET", EndpointGatewayBot, nil, nil, EndpointGatewayBot)
	if err != nil {
		return
	}

	err = unmarshal(response, &st)
	if err != nil {
		return
	}

	// Ensure the gateway always has a trailing slash.
	// MacOS will fail to connect if we add query params without a trailing slash on the base domain.
	if !strings.HasSuffix(st.URL, "/") {
		st.URL += "/"
	}

	return
}

// Functions specific to Webhooks

// WebhookCreate returns a new Webhook.
// channelID: The ID of a Channel.
// name     : The name of the webhook.
// avatar   : The avatar of the webhook.
func (s *Session) WebhookCreate(channelID int64, name, avatar string) (st *Webhook, err error) {

	data := struct {
		Name   string `json:"name"`
		Avatar string `json:"avatar,omitempty"`
	}{name, avatar}

	body, err := s.RequestWithBucketID("POST", EndpointChannelWebhooks(channelID), data, nil, EndpointChannelWebhooks(channelID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return
}

// ChannelWebhooks returns all webhooks for a given channel.
// channelID: The ID of a channel.
func (s *Session) ChannelWebhooks(channelID int64) (st []*Webhook, err error) {

	body, err := s.RequestWithBucketID("GET", EndpointChannelWebhooks(channelID), nil, nil, EndpointChannelWebhooks(channelID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return
}

// GuildWebhooks returns all webhooks for a given guild.
// guildID: The ID of a Guild.
func (s *Session) GuildWebhooks(guildID int64) (st []*Webhook, err error) {

	body, err := s.RequestWithBucketID("GET", EndpointGuildWebhooks(guildID), nil, nil, EndpointGuildWebhooks(guildID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return
}

// Webhook returns a webhook for a given ID
// webhookID: The ID of a webhook.
func (s *Session) Webhook(webhookID int64) (st *Webhook, err error) {

	body, err := s.RequestWithBucketID("GET", EndpointWebhook(webhookID), nil, nil, EndpointWebhooks)
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return
}

// WebhookWithToken returns a webhook for a given ID
// webhookID: The ID of a webhook.
// token    : The auth token for the webhook.
func (s *Session) WebhookWithToken(webhookID int64, token string) (st *Webhook, err error) {

	body, err := s.RequestWithBucketID("GET", EndpointWebhookToken(webhookID, token), nil, nil, EndpointWebhookToken(0, ""))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return
}

// WebhookEdit updates an existing Webhook.
// webhookID: The ID of a webhook.
// name     : The name of the webhook.
// avatar   : The avatar of the webhook.
func (s *Session) WebhookEdit(webhookID int64, name, avatar string, channelID int64) (st *Role, err error) {

	data := struct {
		Name      string `json:"name,omitempty"`
		Avatar    string `json:"avatar,omitempty"`
		ChannelID int64  `json:"channel_id,string,omitempty"`
	}{name, avatar, channelID}

	body, err := s.RequestWithBucketID("PATCH", EndpointWebhook(webhookID), data, nil, EndpointWebhooks)
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return
}

// WebhookEditWithToken updates an existing Webhook with an auth token.
// webhookID: The ID of a webhook.
// token    : The auth token for the webhook.
// name     : The name of the webhook.
// avatar   : The avatar of the webhook.
func (s *Session) WebhookEditWithToken(webhookID int64, token, name, avatar string) (st *Role, err error) {

	data := struct {
		Name   string `json:"name,omitempty"`
		Avatar string `json:"avatar,omitempty"`
	}{name, avatar}

	body, err := s.RequestWithBucketID("PATCH", EndpointWebhookToken(webhookID, token), data, nil, EndpointWebhookToken(0, ""))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return
}

// WebhookDelete deletes a webhook for a given ID
// webhookID: The ID of a webhook.
func (s *Session) WebhookDelete(webhookID int64) (err error) {
	_, err = s.RequestWithBucketID("DELETE", EndpointWebhook(webhookID), nil, nil, EndpointWebhooks)

	return
}

// WebhookDeleteWithToken deletes a webhook for a given ID with an auth token.
// webhookID: The ID of a webhook.
// token    : The auth token for the webhook.
func (s *Session) WebhookDeleteWithToken(webhookID int64, token string) (st *Webhook, err error) {

	body, err := s.RequestWithBucketID("DELETE", EndpointWebhookToken(webhookID, token), nil, nil, EndpointWebhookToken(0, ""))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return
}

// WebhookExecute executes a webhook.
// webhookID: The ID of a webhook.
// token    : The auth token for the webhook
func (s *Session) WebhookExecute(webhookID int64, token string, wait bool, data *WebhookParams) (err error) {
	uri := EndpointWebhookToken(webhookID, token)

	if wait {
		uri += "?wait=true"
	}

	_, err = s.RequestWithBucketID("POST", uri, data, nil, EndpointWebhookToken(webhookID, token))

	return
}

// WebhookExecuteComplex executes a webhook.
// webhookID: The ID of a webhook.
// token    : The auth token for the webhook
func (s *Session) WebhookExecuteComplex(webhookID int64, token string, wait bool, data *WebhookParams) (m *Message, err error) {
	uri := EndpointWebhookToken(webhookID, token)

	if wait {
		uri += "?wait=true"
	}

	endpoint := uri

	// TODO: Remove this when compatibility is not required.
	var files []*File
	if data.File != nil {
		files = []*File{data.File}
	}

	var response []byte
	if len(files) > 0 {
		body := &bytes.Buffer{}
		bodywriter := multipart.NewWriter(body)

		var payload []byte
		payload, err = json.Marshal(data)
		if err != nil {
			return
		}

		var p io.Writer

		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", `form-data; name="payload_json"`)
		h.Set("Content-Type", "application/json")

		p, err = bodywriter.CreatePart(h)
		if err != nil {
			return
		}

		if _, err = p.Write(payload); err != nil {
			return
		}

		for i, file := range files {
			h := make(textproto.MIMEHeader)
			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file%d"; filename="%s"`, i, quoteEscaper.Replace(file.Name)))
			contentType := file.ContentType
			if contentType == "" {
				contentType = "application/octet-stream"
			}
			h.Set("Content-Type", contentType)

			p, err = bodywriter.CreatePart(h)
			if err != nil {
				return
			}

			if _, err = io.Copy(p, file.Reader); err != nil {
				return
			}
		}

		err = bodywriter.Close()
		if err != nil {
			return
		}

		response, err = s.request("POST", endpoint, bodywriter.FormDataContentType(), body.Bytes(), nil, EndpointWebhookToken(webhookID, token))
	} else {
		response, err = s.RequestWithBucketID("POST", endpoint, data, nil, EndpointWebhookToken(webhookID, token))
	}

	if err != nil {
		return
	}

	if wait {
		err = unmarshal(response, &m)
	}

	return

	// _, err = s.RequestWithBucketID("POST", uri, data, EndpointWebhookToken(0, ""))
	// return
}

// MessageReactionAdd creates an emoji reaction to a message.
// channelID : The channel ID.
// messageID : The message ID.
// emoji     : Either the unicode emoji for the reaction, or a guild emoji identifier.
func (s *Session) MessageReactionAdd(channelID, messageID int64, emoji string) error {

	_, err := s.RequestWithBucketID("PUT", EndpointMessageReaction(channelID, messageID, EmojiName{emoji}, "@me"), nil, nil, EndpointMessageReaction(channelID, 0, EmojiName{""}, ""))

	return err
}

// MessageReactionRemove deletes an emoji reaction to a message.
// channelID : The channel ID.
// messageID : The message ID.
// emoji     : Either the unicode emoji for the reaction, or a guild emoji identifier.
// userID	   : The ID of the user to delete the reaction for.
func (s *Session) MessageReactionRemove(channelID, messageID int64, emoji string, userID int64) error {

	_, err := s.RequestWithBucketID("DELETE", EndpointMessageReaction(channelID, messageID, EmojiName{emoji}, StrID(userID)), nil, nil, EndpointMessageReaction(channelID, 0, EmojiName{""}, ""))

	return err
}

// MessageReactionRemoveMe deletes an emoji reaction to a message the current user made.
// channelID : The channel ID.
// messageID : The message ID.
// emoji     : Either the unicode emoji for the reaction, or a guild emoji identifier.
func (s *Session) MessageReactionRemoveMe(channelID, messageID int64, emoji string) error {

	_, err := s.RequestWithBucketID("DELETE", EndpointMessageReaction(channelID, messageID, EmojiName{emoji}, "@me"), nil, nil, EndpointMessageReaction(channelID, 0, EmojiName{""}, ""))

	return err
}

// MessageReactionRemoveEmoji deletes all emoji reactions in a message.
// channelID : The channel ID.
// messageID : The message ID.
// emoji     : Either the unicode emoji for the reaction, or a guild emoji identifier.
func (s *Session) MessageReactionRemoveEmoji(channelID, messageID int64, emoji string) error {

	_, err := s.RequestWithBucketID("DELETE", EndpointMessageReactions(channelID, messageID, EmojiName{emoji}), nil, nil, EndpointMessageReactions(channelID, 0, EmojiName{""}))

	return err
}

// MessageReactionsRemoveAll deletes all reactions from a message
// channelID : The channel ID
// messageID : The message ID.
func (s *Session) MessageReactionsRemoveAll(channelID, messageID int64) error {

	_, err := s.RequestWithBucketID("DELETE", EndpointMessageReactionsAll(channelID, messageID), nil, nil, EndpointMessageReactionsAll(channelID, messageID))

	return err
}

// MessageReactions gets all the users reactions for a specific emoji.
// channelID : The channel ID.
// messageID : The message ID.
// emoji     : Either the unicode emoji for the reaction, or a guild emoji identifier.
// limit     : max number of users to return (max 100)
func (s *Session) MessageReactions(channelID, messageID int64, emoji string, limit int, before, after int64) (st []*User, err error) {
	uri := EndpointMessageReactions(channelID, messageID, EmojiName{emoji})

	v := url.Values{}

	if limit > 0 {
		if limit > 100 {
			limit = 100
		}

		v.Set("limit", strconv.Itoa(limit))
	}

	if before != 0 {
		v.Set("before", strconv.FormatInt(before, 10))
	} else if after != 0 {
		v.Set("after", strconv.FormatInt(after, 10))
	}

	if len(v) > 0 {
		uri = fmt.Sprintf("%s?%s", uri, v.Encode())
	}

	body, err := s.RequestWithBucketID("GET", uri, nil, nil, EndpointMessageReaction(channelID, 0, EmojiName{""}, ""))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// ------------------------------------------------------------------------------------------------
// Functions specific to user notes
// ------------------------------------------------------------------------------------------------

// UserNoteSet sets the note for a specific user.
func (s *Session) UserNoteSet(userID int64, message string) (err error) {
	data := struct {
		Note string `json:"note"`
	}{message}

	_, err = s.RequestWithBucketID("PUT", EndpointUserNotes(userID), data, nil, EndpointUserNotes(0))
	return
}

// ------------------------------------------------------------------------------------------------
// Functions specific to Discord Relationships (Friends list)
// ------------------------------------------------------------------------------------------------

// RelationshipsGet returns an array of all the relationships of the user.
func (s *Session) RelationshipsGet() (r []*Relationship, err error) {
	body, err := s.RequestWithBucketID("GET", EndpointRelationships(), nil, nil, EndpointRelationships())
	if err != nil {
		return
	}

	err = unmarshal(body, &r)
	return
}

// relationshipCreate creates a new relationship. (I.e. send or accept a friend request, block a user.)
// relationshipType : 1 = friend, 2 = blocked, 3 = incoming friend req, 4 = sent friend req
func (s *Session) relationshipCreate(userID int64, relationshipType int) (err error) {
	data := struct {
		Type int `json:"type"`
	}{relationshipType}

	_, err = s.RequestWithBucketID("PUT", EndpointRelationship(userID), data, nil, EndpointRelationships())
	return
}

// RelationshipFriendRequestSend sends a friend request to a user.
// userID: ID of the user.
func (s *Session) RelationshipFriendRequestSend(userID int64) (err error) {
	err = s.relationshipCreate(userID, 4)
	return
}

// RelationshipFriendRequestAccept accepts a friend request from a user.
// userID: ID of the user.
func (s *Session) RelationshipFriendRequestAccept(userID int64) (err error) {
	err = s.relationshipCreate(userID, 1)
	return
}

// RelationshipUserBlock blocks a user.
// userID: ID of the user.
func (s *Session) RelationshipUserBlock(userID int64) (err error) {
	err = s.relationshipCreate(userID, 2)
	return
}

// RelationshipDelete removes the relationship with a user.
// userID: ID of the user.
func (s *Session) RelationshipDelete(userID int64) (err error) {
	_, err = s.RequestWithBucketID("DELETE", EndpointRelationship(userID), nil, nil, EndpointRelationships())
	return
}

// RelationshipsMutualGet returns an array of all the users both @me and the given user is friends with.
// userID: ID of the user.
func (s *Session) RelationshipsMutualGet(userID int64) (mf []*User, err error) {
	body, err := s.RequestWithBucketID("GET", EndpointRelationshipsMutual(userID), nil, nil, EndpointRelationshipsMutual(userID))
	if err != nil {
		return
	}

	err = unmarshal(body, &mf)
	return
}

// GetGlobalApplicationCommands fetches all of the global commands for your application. Returns an array of ApplicationCommand objects.
// GET /applications/{application.id}/commands
func (s *Session) GetGlobalApplicationCommands(applicationID int64) (st []*ApplicationCommand, err error) {
	body, err := s.RequestWithBucketID("GET", EndpointApplicationCommands(applicationID), nil, nil, EndpointApplicationCommands(0))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return
}

// CreateGlobalApplicationCommand creates a new global command. New global commands will be available in all guilds after 1 hour.
// POST /applications/{application.id}/commands
func (s *Session) CreateGlobalApplicationCommand(applicationID int64, command *CreateApplicationCommandRequest) (st *ApplicationCommand, err error) {
	body, err := s.RequestWithBucketID("POST", EndpointApplicationCommands(applicationID), command, nil, EndpointApplicationCommands(0))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return
}

// BulkOverwriteGlobalApplicationCommands Takes a list of application commands, overwriting existing commands that are registered globally for this application. Updates will be available in all guilds after 1 hour.
// PUT /applications/{application.id}/commands
func (s *Session) BulkOverwriteGlobalApplicationCommands(applicationID int64, data []*CreateApplicationCommandRequest) (st []*ApplicationCommand, err error) {
	body, err := s.RequestWithBucketID("PUT", EndpointApplicationCommands(applicationID), data, nil, EndpointApplicationCommands(0))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return
}

// GetGlobalApplicationCommand fetches a global command for your application. Returns an ApplicationCommand object.
// GET /applications/{application.id}/commands/{command.id}
func (s *Session) GetGlobalApplicationCommand(applicationID int64, cmdID int64) (st *ApplicationCommand, err error) {
	body, err := s.RequestWithBucketID("GET", EndpointApplicationCommand(applicationID, cmdID), nil, nil, EndpointApplicationCommand(0, 0))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return
}

type EditApplicationCommandRequest struct {
	Name              *string                      `json:"name,omitempty"`               //	1-32 character name matching ^[\w-]{1,32}$
	Description       *string                      `json:"description,omitempty"`        //	1-100 character description
	Options           *[]*ApplicationCommandOption `json:"options,omitempty"`            // the parameters for the command
	DefaultPermission *bool                        `json:"default_permission,omitempty"` // (default true)	whether the command is enabled by default when the app is added to a guild
}

// EditGlobalApplicationCommand edits a global command. Updates will be available in all guilds after 1 hour.
// PATCH /applications/{application.id}/commands/{command.id}
func (s *Session) EditGlobalApplicationCommand(applicationID int64, cmdID int64, data *EditApplicationCommandRequest) (st *ApplicationCommand, err error) {
	body, err := s.RequestWithBucketID("PATCH", EndpointApplicationCommand(applicationID, cmdID), data, nil, EndpointApplicationCommand(0, 0))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return
}

// DeleteGlobalApplicationCommand deletes a global command.
// DELETE /applications/{application.id}/commands/{command.id}
func (s *Session) DeleteGlobalApplicationCommand(applicationID int64, cmdID int64) (err error) {
	_, err = s.RequestWithBucketID("DELETE", EndpointApplicationCommand(applicationID, cmdID), nil, nil, EndpointApplicationCommand(0, 0))
	return
}

// GetGuildApplicationCommands fetches all of the guild commands for your application for a specific guild.
// GET /applications/{application.id}/guilds/{guild.id}/commands
func (s *Session) GetGuildApplicationCommands(applicationID int64, guildID int64) (st []*ApplicationCommand, err error) {
	body, err := s.RequestWithBucketID("GET", EndpointApplicationGuildCommands(applicationID, guildID), nil, nil, EndpointApplicationGuildCommands(0, guildID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return
}

// CreateGuildApplicationCommands Create a new guild command. New guild commands will be available in the guild immediately. Returns 201 and an ApplicationCommand object. If the command did not already exist, it will count toward daily application command create limits.
// POST /applications/{application.id}/guilds/{guild.id}/commands
func (s *Session) CreateGuildApplicationCommands(applicationID int64, guildID int64, data *CreateApplicationCommandRequest) (st *ApplicationCommand, err error) {
	body, err := s.RequestWithBucketID("POST", EndpointApplicationGuildCommands(applicationID, guildID), data, nil, EndpointApplicationGuildCommands(0, guildID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return
}

// GetGuildApplicationCommand Fetch a guild command for your application.
// GET /applications/{application.id}/guilds/{guild.id}/commands/{command.id}
func (s *Session) GetGuildApplicationCommand(applicationID int64, guildID int64, cmdID int64) (st *ApplicationCommand, err error) {
	body, err := s.RequestWithBucketID("GET", EndpointApplicationGuildCommand(applicationID, guildID, cmdID), nil, nil, EndpointApplicationGuildCommand(0, guildID, 0))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return
}

// EditGuildApplicationCommand Edit a guild command. Updates for guild commands will be available immediately.
// PATCH /applications/{application.id}/guilds/{guild.id}/commands/{command.id}
func (s *Session) EditGuildApplicationCommand(applicationID int64, guildID int64, cmdID int64, data *EditApplicationCommandRequest) (st *ApplicationCommand, err error) {
	body, err := s.RequestWithBucketID("PATCH", EndpointApplicationGuildCommand(applicationID, guildID, cmdID), data, nil, EndpointApplicationGuildCommand(0, guildID, 0))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return
}

// DeleteGuildApplicationCommand Delete a guild command.
// DELETE /applications/{application.id}/guilds/{guild.id}/commands/{command.id}
func (s *Session) DeleteGuildApplicationCommand(applicationID int64, guildID int64, cmdID int64) (err error) {
	_, err = s.RequestWithBucketID("DELETE", EndpointApplicationGuildCommand(applicationID, guildID, cmdID), nil, nil, EndpointApplicationGuildCommand(0, guildID, 0))
	return
}

// BulkOverwriteGuildApplicationCommands Takes a list of application commands, overwriting existing commands for the guild.
// PUT /applications/{application.id}/guilds/{guild.id}/commands
func (s *Session) BulkOverwriteGuildApplicationCommands(applicationID int64, guildID int64, data []*CreateApplicationCommandRequest) (st []*ApplicationCommand, err error) {
	body, err := s.RequestWithBucketID("PUT", EndpointApplicationGuildCommands(applicationID, guildID), data, nil, EndpointApplicationGuildCommands(0, guildID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return
}

// GetGuildApplicationCommandPermissions Fetches command permissions for all commands for your application in a guild.
// GET /applications/{application.id}/guilds/{guild.id}/commands/permissions
func (s *Session) GetGuildApplicationCommandsPermissions(applicationID int64, guildID int64) (st []*GuildApplicationCommandPermissions, err error) {
	body, err := s.RequestWithBucketID("GET", EndpointApplicationGuildCommandsPermissions(applicationID, guildID), nil, nil, EndpointApplicationGuildCommandsPermissions(0, guildID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return
}

// GetGuildApplicationCommandPermissions Fetches command permissions for a specific command for your application in a guild.
// GET /applications/{application.id}/guilds/{guild.id}/commands/{command.id}/permissions
func (s *Session) GetGuildApplicationCommandPermissions(applicationID int64, guildID int64, cmdID int64) (st *GuildApplicationCommandPermissions, err error) {
	body, err := s.RequestWithBucketID("GET", EndpointApplicationGuildCommandPermissions(applicationID, guildID, cmdID), nil, nil, EndpointApplicationGuildCommandPermissions(0, guildID, 0))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return
}

// EditGuildApplicationCommandPermissions Edits command permissions for a specific command for your application in a guild.
// PUT /applications/{application.id}/guilds/{guild.id}/commands/{command.id}/permissions
// TODO: what does this return? docs dosn't say
func (s *Session) EditGuildApplicationCommandPermissions(applicationID int64, guildID int64, cmdID int64, permissions []*ApplicationCommandPermissions) (err error) {
	data := struct {
		Permissions []*ApplicationCommandPermissions `json:"permissions"`
	}{
		permissions,
	}

	_, err = s.RequestWithBucketID("PUT", EndpointApplicationGuildCommandPermissions(applicationID, guildID, cmdID), data, nil, EndpointApplicationGuildCommandPermissions(0, guildID, 0))
	return
}

// BatchEditGuildApplicationCommandsPermissions Fetches command permissions for a specific command for your application in a guild.
// PUT /applications/{application.id}/guilds/{guild.id}/commands/permissions
func (s *Session) BatchEditGuildApplicationCommandsPermissions(applicationID int64, guildID int64, data []*GuildApplicationCommandPermissions) (st []*GuildApplicationCommandPermissions, err error) {

	body, err := s.RequestWithBucketID("PUT", EndpointApplicationGuildCommandsPermissions(applicationID, guildID), data, nil, EndpointApplicationGuildCommandsPermissions(0, guildID))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)

	return
}

// CreateInteractionResponse Create a response to an Interaction from the gateway. Takes an Interaction response.
// POST /interactions/{interaction.id}/{interaction.token}/callback
func (s *Session) CreateInteractionResponse(interactionID int64, token string, data *InteractionResponse) (err error) {
	_, err = s.RequestWithBucketID("POST", EndpointInteractionCallback(interactionID, token), data, nil, EndpointInteractionCallback(0, ""))
	return
}

// GetOriginalInteractionResponse Returns the initial Interaction response. Functions the same as Get Webhook Message.
// GET /webhooks/{application.id}/{interaction.token}/messages/@original
func (s *Session) GetOriginalInteractionResponse(applicationID int64, token string) (st *Message, err error) {
	body, err := s.RequestWithBucketID("GET", EndpointInteractionOriginalMessage(applicationID, token), nil, nil, EndpointInteractionOriginalMessage(0, ""))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// Edits the initial Interaction response. Functions the same as Edit Webhook Message.
// PATCH /webhooks/{application.id}/{interaction.token}/messages/@original
func (s *Session) EditOriginalInteractionResponse(applicationID int64, token string, data *WebhookParams) (st *Message, err error) {
	body, err := s.RequestWithBucketID("PATCH", EndpointInteractionOriginalMessage(applicationID, token), data, nil, EndpointInteractionOriginalMessage(0, ""))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// DeleteInteractionResponse Deletes the initial Interaction response.
// DELETE /webhooks/{application.id}/{interaction.token}/messages/@original
func (s *Session) DeleteInteractionResponse(applicationID int64, token string) (err error) {
	_, err = s.RequestWithBucketID("DELETE", EndpointInteractionOriginalMessage(applicationID, token), nil, nil, EndpointInteractionOriginalMessage(0, ""))
	return
}

// CreateFollowupMessage Creates a followup message for an Interaction. Functions the same as Execute Webhook, but wait is always true, and flags can be set to 64 in the body to send an ephemeral message.
// POST /webhooks/{application.id}/{interaction.token}
func (s *Session) CreateFollowupMessage(applicationID int64, token string, data *WebhookParams) (st *Message, err error) {
	body, err := s.WebhookExecuteComplex(applicationID, token, true, data)
	return body, err

	// body, err := s.RequestWithBucketID("POST", EndpointWebhookToken(applicationID, token), data, EndpointWebhookToken(0, ""))
	// if err != nil {
	// 	return
	// }

	// err = unmarshal(body, &st)
	// return
}

// EditFollowupMessage Edits a followup message for an Interaction. Functions the same as Edit Webhook Message.
// PATCH /webhooks/{application.id}/{interaction.token}/messages/{message.id}
func (s *Session) EditFollowupMessage(applicationID int64, token string, messageID int64, data *WebhookParams) (st *Message, err error) {
	body, err := s.RequestWithBucketID("PATCH", EndpointInteractionFollowupMessage(applicationID, token, messageID), data, nil, EndpointInteractionFollowupMessage(0, "", 0))
	if err != nil {
		return
	}

	err = unmarshal(body, &st)
	return
}

// DeleteFollowupMessage Deletes a followup message for an Interaction.
// DELETE /webhooks/{application.id}/{interaction.token}/messages/{message.id}
func (s *Session) DeleteFollowupMessage(applicationID int64, token string, messageID int64) (err error) {
	_, err = s.RequestWithBucketID("DELETE", EndpointInteractionFollowupMessage(applicationID, token, messageID), nil, nil, EndpointInteractionFollowupMessage(0, "", 0))
	return
}
