package discordgo

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/lib/gojay"
)

// A User stores all data for an individual Discord user.
type User struct {
	// The ID of the user.
	ID int64 `json:"id,string"`

	// The user's username.
	Username string `json:"username"`

	// The user's display name on discord
	Globalname string `json:"global_name"`

	// The hash of the user's avatar. Use Session.UserAvatar
	// to retrieve the avatar itself.
	Avatar string `json:"avatar"`

	// The hash of the user's banner. Use Session.UserBanner
	// to retrieve the banner itself.
	Banner string `json:"banner"`

	// The color of the user's banner in case they don't have a banner.
	BannerColor string `json:"banner_color"`

	// The user's chosen language option.
	Locale string `json:"locale"`

	// The discriminator of the user (4 numbers after name).
	Discriminator string `json:"discriminator"`

	// Whether the user is a bot.
	Bot bool `json:"bot"`

	// The flags on the user.
	Flags UserFlags `json:"flags"`

	// The public flags of the user.
	PublicFlags UserFlags `json:"public_flags"`

	// The user's primary guild.
	PrimaryGuild *UserPrimaryGuild `json:"primary_guild,omitempty"`
}

// UserFlags is the public flags of a user.
type UserFlags int

// Block containing known UserFlags values.
const (
	// UserFlagStaff indicates the user is a Discord employee.
	UserFlagStaff UserFlags = 1 << 0
	// UserFlagPartner indicates the user is a Partnered Server Owner.
	UserFlagPartner UserFlags = 1 << 1
	// UserFlagHypeSquad indicates the user is a HypeSquad Events member.
	UserFlagHypeSquad UserFlags = 1 << 2
	// UserFlagBugHunterLevel1 indicates the user is a Bug Hunter (Level 1).
	UserFlagBugHunterLevel1 UserFlags = 1 << 3
	// UserFlagHypesquadOnlineHouse1 indicates the user is a House Bravery member.
	UserFlagHypesquadOnlineHouse1 UserFlags = 1 << 6
	// UserFlagHypesquadOnlineHouse2 indicates the user is a House Brilliance member.
	UserFlagHypesquadOnlineHouse2 UserFlags = 1 << 7
	// UserFlagHypesquadOnlineHouse3 indicates the user is a House Balance member.
	UserFlagHypesquadOnlineHouse3 UserFlags = 1 << 8
	// UserFlagPremiumEarlySupporter indicates the user is an Early Nitro Supporter.
	UserFlagPremiumEarlySupporter UserFlags = 1 << 9
	// UserFlagTeamPseudoUser indicates the account represents a Team.
	UserFlagTeamPseudoUser UserFlags = 1 << 10
	// UserFlagBugHunterLevel2 indicates the user is a Bug Hunter (Level 2).
	UserFlagBugHunterLevel2 UserFlags = 1 << 14
	// UserFlagVerifiedBot indicates the user is a Verified Bot.
	UserFlagVerifiedBot UserFlags = 1 << 16
	// UserFlagVerifiedDeveloper indicates the user is an Early Verified Bot Developer.
	UserFlagVerifiedDeveloper UserFlags = 1 << 17
	// UserFlagCertifiedModerator indicates the user is Moderator Programs Alumni.
	UserFlagCertifiedModerator UserFlags = 1 << 18
	// UserFlagBotHTTPInteractions indicates the bot uses only HTTP interactions and is shown online.
	UserFlagBotHTTPInteractions UserFlags = 1 << 19
	// UserFlagActiveDeveloper indicates the user is an Active Developer.
	UserFlagActiveDeveloper UserFlags = 1 << 22
)

// UserPrimaryGuild stores information about a user's primary guild.
type UserPrimaryGuild struct {
	IdentityGuildID int64  `json:"identity_guild_id,string"`
	IdentityEnabled bool   `json:"identity_enabled"`
	Tag             string `json:"tag,omitempty"`
	Badge           string `json:"badge,omitempty"`
}

// BadgeURL returns a URL to the user's primary guild badge.
func (pg *UserPrimaryGuild) BadgeURL() string {
	if pg.Badge == "" {
		return ""
	}
	return EndpointGuildTagBadge(pg.IdentityGuildID, pg.Badge)
}

// String returns a unique identifier of the form username#discriminator
func (u *User) String() string {
	if u.Discriminator == "0" {
		return u.Username
	}
	return fmt.Sprintf("%s#%s", u.Username, u.Discriminator)
}

// Mention return a string which mentions the user
func (u *User) Mention() string {
	return fmt.Sprintf("<@%d>", u.ID)
}

// implement gojay.UnmarshalerJSONObject
func (u *User) UnmarshalJSONObject(dec *gojay.Decoder, key string) error {
	switch key {
	case "id":
		return DecodeSnowflake(&u.ID, dec)
	case "username":
		return dec.String(&u.Username)
	case "global_name":
		return dec.String(&u.Globalname)
	case "avatar":
		return dec.String(&u.Avatar)
	case "banner":
		return dec.String(&u.Banner)
	case "banner_color":
		return dec.String(&u.BannerColor)
	case "locale":
		return dec.String(&u.Locale)
	case "discriminator":
		return dec.String(&u.Discriminator)
	case "bot":
		return dec.Bool(&u.Bot)
	case "flags":
		var tmp int
		if err := dec.Int(&tmp); err != nil {
			return err
		}
		u.Flags = UserFlags(tmp)
		return nil
	case "public_flags":
		var tmp int
		if err := dec.Int(&tmp); err != nil {
			return err
		}
		u.PublicFlags = UserFlags(tmp)
		return nil
	}

	return nil
}

func (u *User) NKeys() int {
	return 0
}

// AvatarURL returns a URL to the user's avatar.
//
//	size:    The size of the user's avatar as a power of two
//	         if size is an empty string, no size parameter will
//	         be added to the URL.
func (u *User) AvatarURL(size string) string {
	var URL string
	if u.Avatar == "" {
		// See https://discord.com/developers/docs/reference#image-formatting-cdn-endpoints:
		//
		// "For users on the new username system, `index` will be `(user_id >> 22) % 6`.
		// For users on the legacy username system, `index` will be `discriminator % 5`."
		var index int
		if u.Discriminator == "0" {
			index = int((u.ID >> 22) % 6)
		} else {
			discrim, _ := strconv.Atoi(u.Discriminator)
			index = discrim % 5
		}
		URL = EndpointDefaultUserAvatar(index)
	} else if strings.HasPrefix(u.Avatar, "a_") {
		URL = EndpointUserAvatarAnimated(u.ID, u.Avatar)
	} else {
		URL = EndpointUserAvatar(u.ID, u.Avatar)
	}

	if size != "" {
		return URL + "?size=" + size
	}
	return URL
}

// BannerURL returns a URL to the user's banner.
//
//	size:    The size of the user's banner as a power of two
//	         if size is an empty string, no size parameter will
//	         be added to the URL.
func (u *User) BannerURL(size string) string {
	var URL string
	if u.Banner == "" {
		return ""
	} else if strings.HasPrefix(u.Banner, "a_") {
		URL = EndpointUserBannerAnimated(u.ID, u.Banner)
	} else {
		URL = EndpointUserBanner(u.ID, u.Banner)
	}

	if size != "" {
		return URL + "?size=" + size
	}
	return URL
}

// A SelfUser stores user data about the token owner.
// Includes a few extra fields than a normal user struct.
type SelfUser struct {
	*User
	Token string `json:"token"`
}
