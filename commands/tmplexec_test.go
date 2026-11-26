package commands

import (
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
)

func TestExecInvoker(t *testing.T) {
	targetAuthor := &discordgo.User{ID: 1}
	invoker := &discordgo.User{ID: 2}

	ctx := &templates.Context{
		GS:  &dstate.GuildSet{GuildState: dstate.GuildState{ID: 100}},
		Msg: &discordgo.Message{Author: targetAuthor, Member: &discordgo.Member{User: targetAuthor}},
		CurrentFrame: &templates.ContextFrame{Interaction: &templates.CustomCommandInteraction{
			Interaction: &discordgo.Interaction{Member: &discordgo.Member{User: invoker}},
		}},
	}

	user, member := execInvoker(ctx)
	if user.ID != invoker.ID || member.User.ID != invoker.ID || member.GuildID != 100 {
		t.Fatalf("expected exec to run as the interaction user, got user %d member %+v", user.ID, member)
	}

	ctx.CurrentFrame.Interaction = nil
	user, member = execInvoker(ctx)
	if user.ID != targetAuthor.ID || member.User.ID != targetAuthor.ID {
		t.Fatalf("expected exec to run as the message author without an interaction, got user %d", user.ID)
	}
}
