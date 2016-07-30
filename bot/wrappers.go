// GENERATED using yagpdb/cmd/gen/bot_wrappers.go

// Custom event handlers that adds a redis connection to the handler
// They will also recover from panics

package bot

import (
	"github.com/bwmarrin/discordgo"
	"github.com/fzzy/radix/redis"
	"log"
	"runtime/debug"
)


func CustomChannelCreate(inner func(s *discordgo.Session, evt *discordgo.ChannelCreate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.ChannelCreate) {
	return func(s *discordgo.Session, evt *discordgo.ChannelCreate) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event ChannelCreate")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in ChannelCreate:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomChannelUpdate(inner func(s *discordgo.Session, evt *discordgo.ChannelUpdate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.ChannelUpdate) {
	return func(s *discordgo.Session, evt *discordgo.ChannelUpdate) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event ChannelUpdate")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in ChannelUpdate:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomChannelDelete(inner func(s *discordgo.Session, evt *discordgo.ChannelDelete, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.ChannelDelete) {
	return func(s *discordgo.Session, evt *discordgo.ChannelDelete) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event ChannelDelete")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in ChannelDelete:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomGuildCreate(inner func(s *discordgo.Session, evt *discordgo.GuildCreate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.GuildCreate) {
	return func(s *discordgo.Session, evt *discordgo.GuildCreate) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event GuildCreate")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in GuildCreate:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomGuildUpdate(inner func(s *discordgo.Session, evt *discordgo.GuildUpdate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.GuildUpdate) {
	return func(s *discordgo.Session, evt *discordgo.GuildUpdate) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event GuildUpdate")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in GuildUpdate:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomGuildDelete(inner func(s *discordgo.Session, evt *discordgo.GuildDelete, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.GuildDelete) {
	return func(s *discordgo.Session, evt *discordgo.GuildDelete) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event GuildDelete")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in GuildDelete:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomGuildBanAdd(inner func(s *discordgo.Session, evt *discordgo.GuildBanAdd, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.GuildBanAdd) {
	return func(s *discordgo.Session, evt *discordgo.GuildBanAdd) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event GuildBanAdd")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in GuildBanAdd:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomGuildBanRemove(inner func(s *discordgo.Session, evt *discordgo.GuildBanRemove, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.GuildBanRemove) {
	return func(s *discordgo.Session, evt *discordgo.GuildBanRemove) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event GuildBanRemove")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in GuildBanRemove:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomGuildMemberAdd(inner func(s *discordgo.Session, evt *discordgo.GuildMemberAdd, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.GuildMemberAdd) {
	return func(s *discordgo.Session, evt *discordgo.GuildMemberAdd) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event GuildMemberAdd")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in GuildMemberAdd:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomGuildMemberUpdate(inner func(s *discordgo.Session, evt *discordgo.GuildMemberUpdate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.GuildMemberUpdate) {
	return func(s *discordgo.Session, evt *discordgo.GuildMemberUpdate) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event GuildMemberUpdate")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in GuildMemberUpdate:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomGuildMemberRemove(inner func(s *discordgo.Session, evt *discordgo.GuildMemberRemove, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.GuildMemberRemove) {
	return func(s *discordgo.Session, evt *discordgo.GuildMemberRemove) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event GuildMemberRemove")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in GuildMemberRemove:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomGuildRoleCreate(inner func(s *discordgo.Session, evt *discordgo.GuildRoleCreate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.GuildRoleCreate) {
	return func(s *discordgo.Session, evt *discordgo.GuildRoleCreate) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event GuildRoleCreate")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in GuildRoleCreate:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomGuildRoleUpdate(inner func(s *discordgo.Session, evt *discordgo.GuildRoleUpdate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.GuildRoleUpdate) {
	return func(s *discordgo.Session, evt *discordgo.GuildRoleUpdate) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event GuildRoleUpdate")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in GuildRoleUpdate:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomGuildRoleDelete(inner func(s *discordgo.Session, evt *discordgo.GuildRoleDelete, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.GuildRoleDelete) {
	return func(s *discordgo.Session, evt *discordgo.GuildRoleDelete) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event GuildRoleDelete")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in GuildRoleDelete:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomGuildIntegrationsUpdate(inner func(s *discordgo.Session, evt *discordgo.GuildIntegrationsUpdate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.GuildIntegrationsUpdate) {
	return func(s *discordgo.Session, evt *discordgo.GuildIntegrationsUpdate) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event GuildIntegrationsUpdate")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in GuildIntegrationsUpdate:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomGuildEmojisUpdate(inner func(s *discordgo.Session, evt *discordgo.GuildEmojisUpdate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.GuildEmojisUpdate) {
	return func(s *discordgo.Session, evt *discordgo.GuildEmojisUpdate) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event GuildEmojisUpdate")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in GuildEmojisUpdate:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomMessageAck(inner func(s *discordgo.Session, evt *discordgo.MessageAck, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.MessageAck) {
	return func(s *discordgo.Session, evt *discordgo.MessageAck) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event MessageAck")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in MessageAck:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomMessageCreate(inner func(s *discordgo.Session, evt *discordgo.MessageCreate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.MessageCreate) {
	return func(s *discordgo.Session, evt *discordgo.MessageCreate) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event MessageCreate")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in MessageCreate:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomMessageUpdate(inner func(s *discordgo.Session, evt *discordgo.MessageUpdate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.MessageUpdate) {
	return func(s *discordgo.Session, evt *discordgo.MessageUpdate) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event MessageUpdate")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in MessageUpdate:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomMessageDelete(inner func(s *discordgo.Session, evt *discordgo.MessageDelete, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.MessageDelete) {
	return func(s *discordgo.Session, evt *discordgo.MessageDelete) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event MessageDelete")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in MessageDelete:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomPresenceUpdate(inner func(s *discordgo.Session, evt *discordgo.PresenceUpdate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.PresenceUpdate) {
	return func(s *discordgo.Session, evt *discordgo.PresenceUpdate) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event PresenceUpdate")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in PresenceUpdate:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomPresencesReplace(inner func(s *discordgo.Session, evt *discordgo.PresencesReplace, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.PresencesReplace) {
	return func(s *discordgo.Session, evt *discordgo.PresencesReplace) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event PresencesReplace")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in PresencesReplace:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomReady(inner func(s *discordgo.Session, evt *discordgo.Ready, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.Ready) {
	return func(s *discordgo.Session, evt *discordgo.Ready) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event Ready")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in Ready:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomUserUpdate(inner func(s *discordgo.Session, evt *discordgo.UserUpdate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.UserUpdate) {
	return func(s *discordgo.Session, evt *discordgo.UserUpdate) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event UserUpdate")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in UserUpdate:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomUserSettingsUpdate(inner func(s *discordgo.Session, evt *discordgo.UserSettingsUpdate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.UserSettingsUpdate) {
	return func(s *discordgo.Session, evt *discordgo.UserSettingsUpdate) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event UserSettingsUpdate")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in UserSettingsUpdate:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomUserGuildSettingsUpdate(inner func(s *discordgo.Session, evt *discordgo.UserGuildSettingsUpdate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.UserGuildSettingsUpdate) {
	return func(s *discordgo.Session, evt *discordgo.UserGuildSettingsUpdate) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event UserGuildSettingsUpdate")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in UserGuildSettingsUpdate:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomTypingStart(inner func(s *discordgo.Session, evt *discordgo.TypingStart, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.TypingStart) {
	return func(s *discordgo.Session, evt *discordgo.TypingStart) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event TypingStart")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in TypingStart:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomVoiceServerUpdate(inner func(s *discordgo.Session, evt *discordgo.VoiceServerUpdate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.VoiceServerUpdate) {
	return func(s *discordgo.Session, evt *discordgo.VoiceServerUpdate) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event VoiceServerUpdate")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in VoiceServerUpdate:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomVoiceStateUpdate(inner func(s *discordgo.Session, evt *discordgo.VoiceStateUpdate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.VoiceStateUpdate) {
	return func(s *discordgo.Session, evt *discordgo.VoiceStateUpdate) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event VoiceStateUpdate")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in VoiceStateUpdate:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomResumed(inner func(s *discordgo.Session, evt *discordgo.Resumed, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.Resumed) {
	return func(s *discordgo.Session, evt *discordgo.Resumed) {
		r, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client, cant handle event Resumed")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in Resumed:", r, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

