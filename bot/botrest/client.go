package botrest

import (
	"encoding/json"
	"errors"
	"github.com/jonas747/discordgo"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"
)

var (
	ErrServerError = errors.New("reststate server is having issues")
)

func get(url string, dest interface{}) error {
	resp, err := http.Get("http://" + serverAddr + "/" + url)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		var errDest string
		err := json.NewDecoder(resp.Body).Decode(&errDest)
		if err != nil {
			return ErrServerError
		}

		return errors.New(errDest)
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

func GetMembers(guildID string, members ...string) (m []*discordgo.Member, err error) {
	query := url.Values{"users": members}
	encoded := query.Encode()

	err = get(guildID+"/members?"+encoded, &m)
	return
}

func GetChannelPermissions(guildID, channelID string) (perms int64, err error) {
	err = get(guildID+"/channelperms/"+channelID, &perms)
	return
}

var (
	lastPing      time.Time
	lastPingMutex sync.RWMutex
)

func RunPinger() {
	lastFailed := false
	for {
		time.Sleep(time.Second)

		var dest string
		err := get("ping", &dest)
		if err != nil {
			if !lastFailed {
				log.Println("Ping failed", err)
				lastFailed = true
			}
			continue
		}

		lastPingMutex.Lock()
		lastPing = time.Now()
		lastPingMutex.Unlock()
		lastFailed = false
	}
}

// Returns wether the bot is running or not, (time since last sucessfull ping was less than 5 seconds)
func BotIsRunning() bool {
	lastPingMutex.RLock()
	defer lastPingMutex.RUnlock()
	return time.Since(lastPing) < time.Second*5
}
