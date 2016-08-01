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
			log.Println("Failed retrieving redis client, cant handle event ChannelCreate:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in ChannelCreate:", err, "\n", evt, "\n", stack)
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
			log.Println("Failed retrieving redis client, cant handle event ChannelUpdate:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in ChannelUpdate:", err, "\n", evt, "\n", stack)
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
			log.Println("Failed retrieving redis client, cant handle event ChannelDelete:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in ChannelDelete:", err, "\n", evt, "\n", stack)
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
			log.Println("Failed retrieving redis client, cant handle event GuildCreate:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in GuildCreate:", err, "\n", evt, "\n", stack)
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
			log.Println("Failed retrieving redis client, cant handle event GuildUpdate:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in GuildUpdate:", err, "\n", evt, "\n", stack)
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
			log.Println("Failed retrieving redis client, cant handle event GuildDelete:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in GuildDelete:", err, "\n", evt, "\n", stack)
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
			log.Println("Failed retrieving redis client, cant handle event GuildBanAdd:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in GuildBanAdd:", err, "\n", evt, "\n", stack)
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
			log.Println("Failed retrieving redis client, cant handle event GuildBanRemove:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in GuildBanRemove:", err, "\n", evt, "\n", stack)
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
			log.Println("Failed retrieving redis client, cant handle event GuildMemberAdd:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in GuildMemberAdd:", err, "\n", evt, "\n", stack)
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
			log.Println("Failed retrieving redis client, cant handle event GuildMemberUpdate:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in GuildMemberUpdate:", err, "\n", evt, "\n", stack)
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
			log.Println("Failed retrieving redis client, cant handle event GuildMemberRemove:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in GuildMemberRemove:", err, "\n", evt, "\n", stack)
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
			log.Println("Failed retrieving redis client, cant handle event GuildRoleCreate:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in GuildRoleCreate:", err, "\n", evt, "\n", stack)
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
			log.Println("Failed retrieving redis client, cant handle event GuildRoleUpdate:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in GuildRoleUpdate:", err, "\n", evt, "\n", stack)
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
			log.Println("Failed retrieving redis client, cant handle event GuildRoleDelete:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in GuildRoleDelete:", err, "\n", evt, "\n", stack)
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
			log.Println("Failed retrieving redis client, cant handle event GuildIntegrationsUpdate:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in GuildIntegrationsUpdate:", err, "\n", evt, "\n", stack)
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
			log.Println("Failed retrieving redis client, cant handle event GuildEmojisUpdate:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in GuildEmojisUpdate:", err, "\n", evt, "\n", stack)
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
			log.Println("Failed retrieving redis client, cant handle event MessageAck:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in MessageAck:", err, "\n", evt, "\n", stack)
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
			log.Println("Failed retrieving redis client, cant handle event MessageCreate:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in MessageCreate:", err, "\n", evt, "\n", stack)
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
			log.Println("Failed retrieving redis client, cant handle event MessageUpdate:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in MessageUpdate:", err, "\n", evt, "\n", stack)
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
			log.Println("Failed retrieving redis client, cant handle event MessageDelete:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in MessageDelete:", err, "\n", evt, "\n", stack)
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
			log.Println("Failed retrieving redis client, cant handle event PresenceUpdate:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in PresenceUpdate:", err, "\n", evt, "\n", stack)
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
			log.Println("Failed retrieving redis client, cant handle event PresencesReplace:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in PresencesReplace:", err, "\n", evt, "\n", stack)
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
			log.Println("Failed retrieving redis client, cant handle event Ready:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in Ready:", err, "\n", evt, "\n", stack)
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
			log.Println("Failed retrieving redis client, cant handle event UserUpdate:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in UserUpdate:", err, "\n", evt, "\n", stack)
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
			log.Println("Failed retrieving redis client, cant handle event UserSettingsUpdate:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in UserSettingsUpdate:", err, "\n", evt, "\n", stack)
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
			log.Println("Failed retrieving redis client, cant handle event UserGuildSettingsUpdate:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in UserGuildSettingsUpdate:", err, "\n", evt, "\n", stack)
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
			log.Println("Failed retrieving redis client, cant handle event TypingStart:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in TypingStart:", err, "\n", evt, "\n", stack)
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
			log.Println("Failed retrieving redis client, cant handle event VoiceServerUpdate:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in VoiceServerUpdate:", err, "\n", evt, "\n", stack)
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
			log.Println("Failed retrieving redis client, cant handle event VoiceStateUpdate:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in VoiceStateUpdate:", err, "\n", evt, "\n", stack)
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
			log.Println("Failed retrieving redis client, cant handle event Resumed:", err)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Println("Recovered from panic in Resumed:", err, "\n", evt, "\n", stack)
			}
			RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

