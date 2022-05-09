//go:generate generateEmojiCodeMap -pkg discordgo
// Discordgo - Discord bindings for Go
// Available at https://github.com/bwmarrin/discordgo

// Copyright 2015-2016 Bruce Marriner <bruce@sqls.net>.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains custom types, currently only a timestamp wrapper.

package discordgo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/lib/gojay"
)

// Timestamp stores a timestamp, as sent by the Discord API.
type Timestamp string

// Parse parses a timestamp string into a time.Time object.
// The only time this can fail is if Discord changes their timestamp format.
func (t Timestamp) Parse() (time.Time, error) {
	tim, err := time.Parse(time.RFC3339, string(t))
	return tim.UTC(), err
}

// RESTError stores error information about a request with a bad response code.
// Message is not always present, there are cases where api calls can fail
// without returning a json message.
type RESTError struct {
	Request      *http.Request
	Response     *http.Response
	ResponseBody []byte

	Message *APIErrorMessage // Message may be nil.
}

func newRestError(req *http.Request, resp *http.Response, body []byte) *RESTError {
	restErr := &RESTError{
		Request:      req,
		Response:     resp,
		ResponseBody: body,
	}

	// Attempt to decode the error and assume no message was provided if it fails
	var msg *APIErrorMessage
	err := json.Unmarshal(body, &msg)
	if err == nil {
		restErr.Message = msg
	}

	return restErr
}

func (r RESTError) Error() string {
	return fmt.Sprintf("HTTP %s, %s", r.Response.Status, r.ResponseBody)
}

// A NullableID is a nullable snowflake ID that represents null as the value 0.
// It marshals into "null" if its value is 0, and otherwise marshals into the
// string representation of its value. Unmarshaling behaves similarly, and
// accepts null, string, and integer values.
type NullableID int64

func (i NullableID) MarshalJSON() ([]byte, error) {
	if i == 0 {
		return []byte("null"), nil
	}
	out := []byte{'"'}
	out = strconv.AppendInt(out, int64(i), 10)
	return append(out, '"'), nil
}

func (i *NullableID) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		*i = 0
		return nil
	}

	if len(data) > 2 && data[0] == '"' && data[len(data)-1] == '"' {
		data = data[1 : len(data)-1]
	}
	v, err := strconv.ParseInt(string(data), 10, 64)
	*i = NullableID(v)
	return err
}

// IDSlice Is a slice of snowflake id's that properly marshals and unmarshals the way discord expects them to
// They unmarshal from string arrays and marshals back to string arrays
type IDSlice []int64

func (ids *IDSlice) UnmarshalJSON(data []byte) error {
	if len(data) < 3 {
		return nil
	}

	// Split and strip away "[" "]"
	split := strings.Split(string(data[1:len(data)-1]), ",")
	*ids = make([]int64, 0, len(split))
	for _, s := range split {
		s = strings.TrimSpace(s)
		if len(s) < 3 {
			// Empty or invalid
			continue
		}

		// Strip away quotes and parse
		parsed, err := strconv.ParseInt(s[1:len(s)-1], 10, 64)
		if err != nil {
			return err
		}

		*ids = append(*ids, parsed)
	}

	return nil
}

func (ids IDSlice) MarshalJSON() ([]byte, error) {
	// Capacity:
	// 2 brackets
	// each id is:
	//    18 characters currently, but 1 extra added for the future,
	//    1 comma
	//    2 quotes
	if len(ids) < 1 {
		return []byte("[]"), nil
	}

	outPut := make([]byte, 1, 2+(len(ids)*22))
	outPut[0] = '['

	for i, id := range ids {
		if i != 0 {
			outPut = append(outPut, '"', ',', '"')
		} else {
			outPut = append(outPut, '"')
		}
		outPut = append(outPut, []byte(strconv.FormatInt(id, 10))...)
	}

	outPut = append(outPut, '"', ']')
	return outPut, nil
}

// implement UnmarshalerJSONArray
func (ids *IDSlice) UnmarshalJSONArray(dec *gojay.Decoder) error {
	str := ""
	if err := dec.String(&str); err != nil {
		return err
	}

	parsed, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return err
	}

	*ids = append(*ids, parsed)
	return nil
}

type EmojiName struct {
	string
}

func (emoji EmojiName) String() string {
	if codepoint, ok := emojiCodeMap[emoji.string]; ok {
		emoji.string = codepoint
	}
	// Discord does not accept the emoji qualifier character.
	// return strings.Replace(emoji.string, "\uFE0F", "", 1)
	// this no longer the case? in fact its required?
	return url.PathEscape(emoji.string)
}

// Discord is super inconsistent with with types in some places (especially presence updates,
// might aswell change them to map[string]interface{} soon because there is 0 validation)
type DiscordFloat float64

func (df *DiscordFloat) UnmarshalJSON(data []byte) error {
	var dst json.Number
	err := json.Unmarshal(data, &dst)
	if err != nil {
		return err
	}

	parsed, err := dst.Float64()
	if err != nil {
		return err
	}

	*df = DiscordFloat(parsed)
	return nil
}

type DiscordInt64 int64

func (di *DiscordInt64) UnmarshalJSON(data []byte) error {
	var dst json.Number
	err := json.Unmarshal(data, &dst)
	if err != nil {
		return err
	}

	parsed, err := dst.Int64()
	if err != nil {
		// Attempt to fallback to float, we lost some precision but eh, what can you do when discord is so freaking inconsistent
		f, err := dst.Float64()
		if err != nil {
			return err
		}
		*di = DiscordInt64(int64(f))
		return nil
	}

	*di = DiscordInt64(parsed)
	return nil
}

func DecodeSnowflake(dst *int64, dec *gojay.Decoder) error {
	var str string
	err := dec.String(&str)
	if err != nil {
		return err
	}

	parsed, err := strconv.ParseInt(str, 10, 64)
	*dst = parsed
	return err
}

type GuildEvent interface {
	GetGuildID() int64
}

type ChannelEvent interface {
	GetChannelID() int64
}
