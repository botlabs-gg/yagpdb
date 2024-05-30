package reddit

import (
	"context"
	"fmt"
	"strings"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/reddit/models"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/util"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

var _ bot.RemoveGuildHandler = (*Plugin)(nil)

func (p *Plugin) RemoveGuild(g int64) error {
	_, err := models.RedditFeeds(models.RedditFeedWhere.GuildID.EQ(g)).UpdateAllG(context.Background(), models.M{
		"disabled": true,
	})
	if err != nil {
		return errors.WrapIf(err, "failed removing reddit feeds")
	}

	return nil
}

func (p *Plugin) OnRemovedPremiumGuild(guildID int64) error {
	logger.WithField("guild_id", guildID).Infof("Removed Excess Reddit Feeds")
	feeds, err := models.RedditFeeds(qm.Where("guild_id = ? and disabled = ?", guildID, false), qm.Offset(GuildMaxFeedsNormal)).AllG(context.Background())

	if err != nil {
		return errors.WrapIf(err, "failed getting reddit feeds")
	}

	if len(feeds) > 0 {
		_, err = feeds.UpdateAllG(context.Background(), models.M{"disabled": true})
		if err != nil {
			return errors.WrapIf(err, "failed disabling reddit feeds on premium removal")
		}
	}

	return nil
}

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(p, &commands.YAGCommand{
		CmdCategory:          commands.CategoryDebug,
		HideFromCommandsPage: true,
		Name:                 "testreddit",
		Description:          "Tests the reddit feeds in this server by checking the specified post. Bot Owner Only",
		HideFromHelp:         true,
		RequiredArgs:         1,
		Arguments: []*dcmd.ArgDef{
			{Name: "post-id", Type: dcmd.String},
		},
		RunFunc: util.RequireOwner(func(data *dcmd.Data) (interface{}, error) {
			pID := data.Args[0].Str()
			if !strings.HasPrefix(pID, "t3_") {
				pID = "t3_" + pID
			}

			resp, err := p.redditClient.LinksInfo([]string{pID})
			if err != nil {
				return nil, err
			}

			if len(resp) < 1 {
				return "Unknown post", nil
			}

			handlerSlow := &PostHandlerImpl{
				Slow:        true,
				ratelimiter: NewRatelimiter(),
			}

			handlerFast := &PostHandlerImpl{
				Slow:        false,
				ratelimiter: NewRatelimiter(),
			}

			err1 := handlerSlow.handlePost(resp[0], data.GuildData.GS.ID)
			err2 := handlerFast.handlePost(resp[0], data.GuildData.GS.ID)

			return fmt.Sprintf("SlowErr: `%v`, fastErr: `%v`", err1, err2), nil
		}),
	})
}

func (p *Plugin) Status() (string, string) {
	feeds, err := models.RedditFeeds(models.RedditFeedWhere.Disabled.EQ(false)).CountG(context.Background())
	if err != nil {
		logger.WithError(err).Error("Failed Checking Reddit feeds")
		return "Total Feeds", "error"
	}

	return "Total Feeds", fmt.Sprintf("%d", feeds)
}
