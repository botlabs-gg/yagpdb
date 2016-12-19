package botrest

import (
	"encoding/json"
	"errors"
	"github.com/jonas747/discordgo"
	"log"
	"net/http"
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
	return time.Now().Sub(lastPing) < time.Second*5
}
