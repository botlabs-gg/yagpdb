package discordgo

import (
	"fmt"
	"os"
)

//////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////// VARS NEEDED FOR TESTING
var (
	dgBot       *Session                 // Stores a global discordgo bot session
	envBotToken = os.Getenv("DGB_TOKEN") // Token to use when authenticating the bot account
)

func init() {
	fmt.Println("Init is being called.")
	if envBotToken != "" {
		if d, err := New(envBotToken); err == nil {
			dgBot = d
		}
	}
}
