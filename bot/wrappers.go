// GENERATED using yagpdb/cmd/gen/bot_wrappers.go

// Custom event handlers that adds a redis connection to the handler
// They will also recover from panics

package bot

import (
	log "github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"runtime/debug"
)

func CustomChannelCreate(inner func(s *discordgo.Session, evt *discordgo.ChannelCreate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.ChannelCreate) {
	return func(s *discordgo.Session, evt *discordgo.ChannelCreate) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "ChannelCreate").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "ChannelCreate").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomChannelUpdate(inner func(s *discordgo.Session, evt *discordgo.ChannelUpdate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.ChannelUpdate) {
	return func(s *discordgo.Session, evt *discordgo.ChannelUpdate) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "ChannelUpdate").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "ChannelUpdate").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomChannelDelete(inner func(s *discordgo.Session, evt *discordgo.ChannelDelete, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.ChannelDelete) {
	return func(s *discordgo.Session, evt *discordgo.ChannelDelete) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "ChannelDelete").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "ChannelDelete").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomChannelPinsUpdate(inner func(s *discordgo.Session, evt *discordgo.ChannelPinsUpdate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.ChannelPinsUpdate) {
	return func(s *discordgo.Session, evt *discordgo.ChannelPinsUpdate) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "ChannelPinsUpdate").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "ChannelPinsUpdate").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomGuildCreate(inner func(s *discordgo.Session, evt *discordgo.GuildCreate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.GuildCreate) {
	return func(s *discordgo.Session, evt *discordgo.GuildCreate) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "GuildCreate").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "GuildCreate").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomGuildUpdate(inner func(s *discordgo.Session, evt *discordgo.GuildUpdate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.GuildUpdate) {
	return func(s *discordgo.Session, evt *discordgo.GuildUpdate) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "GuildUpdate").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "GuildUpdate").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomGuildDelete(inner func(s *discordgo.Session, evt *discordgo.GuildDelete, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.GuildDelete) {
	return func(s *discordgo.Session, evt *discordgo.GuildDelete) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "GuildDelete").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "GuildDelete").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomGuildBanAdd(inner func(s *discordgo.Session, evt *discordgo.GuildBanAdd, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.GuildBanAdd) {
	return func(s *discordgo.Session, evt *discordgo.GuildBanAdd) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "GuildBanAdd").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "GuildBanAdd").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomGuildBanRemove(inner func(s *discordgo.Session, evt *discordgo.GuildBanRemove, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.GuildBanRemove) {
	return func(s *discordgo.Session, evt *discordgo.GuildBanRemove) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "GuildBanRemove").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "GuildBanRemove").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomGuildMemberAdd(inner func(s *discordgo.Session, evt *discordgo.GuildMemberAdd, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.GuildMemberAdd) {
	return func(s *discordgo.Session, evt *discordgo.GuildMemberAdd) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "GuildMemberAdd").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "GuildMemberAdd").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomGuildMemberUpdate(inner func(s *discordgo.Session, evt *discordgo.GuildMemberUpdate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.GuildMemberUpdate) {
	return func(s *discordgo.Session, evt *discordgo.GuildMemberUpdate) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "GuildMemberUpdate").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "GuildMemberUpdate").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomGuildMemberRemove(inner func(s *discordgo.Session, evt *discordgo.GuildMemberRemove, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.GuildMemberRemove) {
	return func(s *discordgo.Session, evt *discordgo.GuildMemberRemove) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "GuildMemberRemove").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "GuildMemberRemove").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomGuildMembersChunk(inner func(s *discordgo.Session, evt *discordgo.GuildMembersChunk, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.GuildMembersChunk) {
	return func(s *discordgo.Session, evt *discordgo.GuildMembersChunk) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "GuildMembersChunk").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "GuildMembersChunk").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomGuildRoleCreate(inner func(s *discordgo.Session, evt *discordgo.GuildRoleCreate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.GuildRoleCreate) {
	return func(s *discordgo.Session, evt *discordgo.GuildRoleCreate) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "GuildRoleCreate").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "GuildRoleCreate").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomGuildRoleUpdate(inner func(s *discordgo.Session, evt *discordgo.GuildRoleUpdate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.GuildRoleUpdate) {
	return func(s *discordgo.Session, evt *discordgo.GuildRoleUpdate) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "GuildRoleUpdate").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "GuildRoleUpdate").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomGuildRoleDelete(inner func(s *discordgo.Session, evt *discordgo.GuildRoleDelete, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.GuildRoleDelete) {
	return func(s *discordgo.Session, evt *discordgo.GuildRoleDelete) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "GuildRoleDelete").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "GuildRoleDelete").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomGuildIntegrationsUpdate(inner func(s *discordgo.Session, evt *discordgo.GuildIntegrationsUpdate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.GuildIntegrationsUpdate) {
	return func(s *discordgo.Session, evt *discordgo.GuildIntegrationsUpdate) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "GuildIntegrationsUpdate").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "GuildIntegrationsUpdate").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomGuildEmojisUpdate(inner func(s *discordgo.Session, evt *discordgo.GuildEmojisUpdate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.GuildEmojisUpdate) {
	return func(s *discordgo.Session, evt *discordgo.GuildEmojisUpdate) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "GuildEmojisUpdate").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "GuildEmojisUpdate").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomMessageAck(inner func(s *discordgo.Session, evt *discordgo.MessageAck, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.MessageAck) {
	return func(s *discordgo.Session, evt *discordgo.MessageAck) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "MessageAck").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "MessageAck").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomMessageCreate(inner func(s *discordgo.Session, evt *discordgo.MessageCreate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.MessageCreate) {
	return func(s *discordgo.Session, evt *discordgo.MessageCreate) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "MessageCreate").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "MessageCreate").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomMessageUpdate(inner func(s *discordgo.Session, evt *discordgo.MessageUpdate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.MessageUpdate) {
	return func(s *discordgo.Session, evt *discordgo.MessageUpdate) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "MessageUpdate").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "MessageUpdate").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomMessageDelete(inner func(s *discordgo.Session, evt *discordgo.MessageDelete, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.MessageDelete) {
	return func(s *discordgo.Session, evt *discordgo.MessageDelete) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "MessageDelete").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "MessageDelete").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomPresenceUpdate(inner func(s *discordgo.Session, evt *discordgo.PresenceUpdate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.PresenceUpdate) {
	return func(s *discordgo.Session, evt *discordgo.PresenceUpdate) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "PresenceUpdate").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "PresenceUpdate").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomPresencesReplace(inner func(s *discordgo.Session, evt *discordgo.PresencesReplace, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.PresencesReplace) {
	return func(s *discordgo.Session, evt *discordgo.PresencesReplace) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "PresencesReplace").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "PresencesReplace").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomReady(inner func(s *discordgo.Session, evt *discordgo.Ready, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.Ready) {
	return func(s *discordgo.Session, evt *discordgo.Ready) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "Ready").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "Ready").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomUserUpdate(inner func(s *discordgo.Session, evt *discordgo.UserUpdate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.UserUpdate) {
	return func(s *discordgo.Session, evt *discordgo.UserUpdate) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "UserUpdate").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "UserUpdate").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomUserSettingsUpdate(inner func(s *discordgo.Session, evt *discordgo.UserSettingsUpdate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.UserSettingsUpdate) {
	return func(s *discordgo.Session, evt *discordgo.UserSettingsUpdate) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "UserSettingsUpdate").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "UserSettingsUpdate").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomUserGuildSettingsUpdate(inner func(s *discordgo.Session, evt *discordgo.UserGuildSettingsUpdate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.UserGuildSettingsUpdate) {
	return func(s *discordgo.Session, evt *discordgo.UserGuildSettingsUpdate) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "UserGuildSettingsUpdate").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "UserGuildSettingsUpdate").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomTypingStart(inner func(s *discordgo.Session, evt *discordgo.TypingStart, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.TypingStart) {
	return func(s *discordgo.Session, evt *discordgo.TypingStart) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "TypingStart").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "TypingStart").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomVoiceServerUpdate(inner func(s *discordgo.Session, evt *discordgo.VoiceServerUpdate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.VoiceServerUpdate) {
	return func(s *discordgo.Session, evt *discordgo.VoiceServerUpdate) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "VoiceServerUpdate").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "VoiceServerUpdate").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomVoiceStateUpdate(inner func(s *discordgo.Session, evt *discordgo.VoiceStateUpdate, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.VoiceStateUpdate) {
	return func(s *discordgo.Session, evt *discordgo.VoiceStateUpdate) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "VoiceStateUpdate").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "VoiceStateUpdate").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}

func CustomResumed(inner func(s *discordgo.Session, evt *discordgo.Resumed, r *redis.Client)) func(s *discordgo.Session, evt *discordgo.Resumed) {
	return func(s *discordgo.Session, evt *discordgo.Resumed) {
		r, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).WithField("evt", "Resumed").Error("Failed retrieving redis client")
			return
		}

		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.WithField(log.ErrorKey, err).WithField("evt", "Resumed").Error("Recovered from panic\n" + stack)
			}
			common.RedisPool.Put(r)
		}()

		inner(s, evt, r)
	}
}
