package tickets

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/botlabs-gg/yagpdb/v2/tickets/models"
	"github.com/volatiletech/null"
)

func init() {
	templates.RegisterSetupFunc(func(ctx *templates.Context) {
		ctx.ContextFuncs["createTicket"] = tmplCreateTicket(ctx)
	})
}

// tmplRunCC either run another custom command immeditely with a max stack depth of 2
// or schedules a custom command to be run in the future sometime with the provided data placed in .ExecData
func tmplCreateTicket(ctx *templates.Context) interface{} {
	return func(author interface{}, topic string) (*TemplateTicket, error) {
		if ctx.IncreaseCheckCallCounterPremium("ticket", 1, 1) {
			return nil, templates.ErrTooManyCalls
		}

		var ms *dstate.MemberState
		if author != nil {
			var fetchID int64
			switch t := author.(type) {
			case *dstate.MemberState:
				ms = t
			case discordgo.User:
				fetchID = t.ID
			case *discordgo.User:
				fetchID = t.ID
			case int64:
				fetchID = t
			case int:
				fetchID = int64(t)
			case string:
				var err error
				fetchID, err = strconv.ParseInt(t, 10, 64)
				if err != nil {
					return nil, err
				}
			}

			if fetchID != 0 {
				var err error
				ms, err = bot.GetMember(ctx.GS.ID, fetchID)
				if err != nil {
					return nil, err
				}
			}

			if ms == nil {
				return nil, errors.New("no member provided")
			}

		} else {
			ms = ctx.MS
		}

		conf, err := models.FindTicketConfigG(context.Background(), ctx.GS.ID)
		if err != nil {
			if err != sql.ErrNoRows {
				return nil, err
			}

			conf = &models.TicketConfig{}
		}

		if !conf.Enabled {
			return nil, errors.New("tickets are disabled on this server")
		}

		gs, ticket, err := CreateTicket(context.Background(), ctx.GS, ms, conf, topic, true)
		ctx.GS = gs

		if err != nil {
			switch err.(type) {
			case TicketUserError:
				return nil, err
			case *TicketUserError:
				return nil, err
			}

			return nil, errors.New("an unknown error occured")
		}
		return &TemplateTicket{
			GuildID:               ticket.GuildID,
			LocalID:               ticket.LocalID,
			ChannelID:             ticket.ChannelID,
			Title:                 ticket.Title,
			CreatedAt:             ticket.CreatedAt,
			ClosedAt:              ticket.ClosedAt,
			LogsID:                ticket.LogsID,
			AuthorID:              ticket.AuthorID,
			AuthorUsernameDiscrim: ticket.AuthorUsernameDiscrim,
		}, nil
	}
}

type TemplateTicket struct {
	GuildID               int64
	LocalID               int64
	ChannelID             int64
	Title                 string
	CreatedAt             time.Time
	ClosedAt              null.Time
	LogsID                int64
	AuthorID              int64
	AuthorUsernameDiscrim string
}
