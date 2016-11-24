package reststate

import (
	"encoding/json"
	"errors"
	"github.com/jonas747/discordgo"
	"net/http"
)

var (
	ErrServerError = errors.New("reststate server is having issues")
)

func get(url string, dest interface{}) error {
	resp, err := http.Get(serverAddr + "/" + url)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return ErrServerError
	}

	return json.NewDecoder(resp.Body).Decode(dest)
}

func GetGuild(guildID string) (g *discordgo.Guild, err error) {
	err = get(guildID+"/guild", &g)
	return
}

func GetBotMember(guildID string) (m *discordgo.Member, err error) {
	err = get(guildID+"/botmember", &m)
	return
}
