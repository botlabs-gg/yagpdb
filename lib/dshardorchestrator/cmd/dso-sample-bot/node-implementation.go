package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dshardorchestrator"
	"github.com/botlabs-gg/yagpdb/v2/lib/dshardorchestrator/node"
)

type Bot struct {
	token       string
	sessions    []*discordgo.Session
	mu          sync.Mutex
	totalShards int
}

func (b *Bot) SessionEstablished(info node.SessionInfo) {
	b.mu.Lock()
	b.totalShards = info.TotalShards
	b.mu.Unlock()
}

func (b *Bot) StopShard(shard int) (sessionID string, sequence int64, resumeGatewayUrl string) {
	b.mu.Lock()
	for i, v := range b.sessions {
		if v.ShardID == shard {
			v.Close()
			sessionID, sequence, resumeGatewayUrl = v.GatewayManager.GetSessionInfo()
			b.sessions[i] = nil
			b.sessions = append(b.sessions[:i], b.sessions[i+1:]...)
		}
	}
	b.mu.Unlock()

	return
}

func (b *Bot) StartShard(shard int, sessionID string, sequence int64, resumeGatewayUrl string) {
	nodeID := Node.GetIDLock()

	b.mu.Lock()
	defer b.mu.Unlock()

	for _, v := range b.sessions {
		if v.ShardID == shard {
			log.Println("tried starting a shard thats already started: ", shard)
			return
		}
	}

	newSession, err := discordgo.New(b.token)
	if err != nil {
		log.Println("an error occured when creating shard session: ", err)
		return
	}

	newSession.GatewayManager.SetSessionInfo(sessionID, sequence, resumeGatewayUrl)
	newSession.ShardID = shard
	newSession.ShardCount = b.totalShards

	newSession.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if !strings.HasPrefix(m.Content, "<@232658301714825217>") && !strings.HasPrefix(m.Content, "<@!232658301714825217>") {
			return
		}

		split := strings.SplitN(m.Content, " ", 2)
		if len(split) < 2 {
			return
		}

		if strings.TrimSpace(split[1]) == "shard" {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("shard %d on node %s", s.ShardID, nodeID))
		}
	})

	err = newSession.Open()
	if err != nil {
		log.Println("an error occured when starting shard session: ", err)
		return
	}

	b.sessions = append(b.sessions, newSession)
}

// called when the bot should shut down, make sure to send EvtShutdown when completed
func (b *Bot) Shutdown() {
	os.Exit(0)
}

func (b *Bot) InitializeShardTransferFrom(shard int) (sessionID string, sequence int64, resumeGatewayUrl string) {
	return b.StopShard(shard)
}

func (b *Bot) InitializeShardTransferTo(shard int, sessionID string, sequence int64, resumeGatewayUrl string) {
	// this isn't actually needed, as startshard will be called with the same session details
}

// this should return when all user events has been sent, with the number of user events sent
func (b *Bot) StartShardTransferFrom(shard int) (numEventsSent int) {
	return 0
}

func (b *Bot) HandleUserEvent(evt dshardorchestrator.EventType, data interface{}) {

}

func (b *Bot) AddNewShards(shards ...int) {

}

func (b *Bot) ResumeShard(shard int, sessionID string, sequence int64, resumeGatewayUrl string) {

}
