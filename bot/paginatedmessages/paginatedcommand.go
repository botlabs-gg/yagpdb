package paginatedmessages

import (
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

type CtxKey int

const CtxKeyNoPagination CtxKey = 1

type PaginatedCommandFunc func(data *dcmd.Data, p *PaginatedMessage, page int) (*discordgo.MessageEmbed, error)

func PaginatedCommand(pageArgIndex int, cb PaginatedCommandFunc) dcmd.RunFunc {
	return func(data *dcmd.Data) (interface{}, error) {
		page := 1
		if pageArgIndex > -1 {
			page = data.Args[pageArgIndex].Int()
		}

		if page < 1 {
			page = 1
		}

		if data.Context().Value(CtxKeyNoPagination) != nil {
			return cb(data, nil, page)
		}

		_, err := CreatePaginatedMessage(data.GuildData.GS.ID, data.GuildData.CS.ID, page, 0, func(p *PaginatedMessage, page int) (*discordgo.MessageEmbed, error) {
			return cb(data, p, page)
		})

		return nil, err
	}
}
