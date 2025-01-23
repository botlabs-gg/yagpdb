package reminders

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2"
	seventsmodels "github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2/models"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/botlabs-gg/yagpdb/v2/reminders/models"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

var logger = common.GetPluginLogger(&Plugin{})

var _ bot.BotInitHandler = (*Plugin)(nil)
var _ commands.CommandProvider = (*Plugin)(nil)

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(p, cmds...)
}

func (p *Plugin) BotInit() {
	scheduledevents2.RegisterHandler("reminders_check_user", int64(0), checkUserScheduledEvent)
	scheduledevents2.RegisterLegacyMigrater("reminders_check_user", migrateLegacyScheduledEvents)
}

const (
	MaxReminders = 25

	MaxReminderOffset            = time.Hour * 24 * 366
	MaxReminderOffsetExceededMsg = "Can be max 1 year from now..."
)

// Reminder management commands
var cmds = []*commands.YAGCommand{
	{
		CmdCategory:  commands.CategoryTool,
		Name:         "Remindme",
		Description:  "Schedules a reminder, example: 'remindme 1h30min are you still alive?'",
		Aliases:      []string{"remind", "reminder"},
		RequiredArgs: 2,
		Arguments: []*dcmd.ArgDef{
			{Name: "Time", Type: &commands.DurationArg{}},
			{Name: "Message", Type: dcmd.String},
		},
		ArgSwitches: []*dcmd.ArgDef{
			{Name: "channel", Type: dcmd.Channel},
		},
		SlashCommandEnabled: true,
		DefaultEnabled:      true,
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			uid := discordgo.StrID(parsed.Author.ID)
			count, _ := models.Reminders(models.ReminderWhere.UserID.EQ(uid)).CountG(parsed.Context())
			if count >= MaxReminders {
				return fmt.Sprintf("You can have a maximum of %d active reminders; list all your reminders with the `reminders` command in DM, doing it in a server will only show reminders set in the server", MaxReminders), nil
			}

			if parsed.Author.Bot {
				return nil, errors.New("cannot create reminder for bots; you're likely trying to use `execAdmin` to create a reminder (use `exec` instead)")
			}

			offsetFromNow := parsed.Args[0].Value.(time.Duration)
			if offsetFromNow > MaxReminderOffset {
				return MaxReminderOffsetExceededMsg, nil
			}

			id := parsed.ChannelID
			if c := parsed.Switch("channel"); c.Value != nil {
				cs := c.Value.(*dstate.ChannelState)
				id = cs.ID
				mention, _ := cs.Mention()

				hasPerms, err := bot.AdminOrPermMS(parsed.GuildData.GS.ID, cs.ID, parsed.GuildData.MS, discordgo.PermissionSendMessages|discordgo.PermissionViewChannel)
				if err != nil {
					return "Failed checking permissions; please try again or join the support server.", err
				}

				if !hasPerms {
					return fmt.Sprintf("You do not have permissions to send messages in %s", mention), nil
				}

				// Ensure the member can run the `remindme` command in the
				// target channel according to the configured command and
				// channel overrides, if any.
				yc := parsed.Cmd.Command.(*commands.YAGCommand)
				settings, err := yc.GetSettings(parsed.ContainerChain, cs, parsed.GuildData.GS)
				if err != nil {
					return "Failed fetching command settings", err
				}

				if !settings.Enabled {
					return fmt.Sprintf("The `remindme` command is disabled in %s", mention), nil
				}

				ms := parsed.GuildData.MS
				// If there are no required roles set, the member should be allowed to run the command.
				hasRequiredRoles := len(settings.RequiredRoles) == 0 || memberHasAnyRole(ms, settings.RequiredRoles)
				hasIgnoredRoles := memberHasAnyRole(ms, settings.IgnoreRoles)
				if !hasRequiredRoles || hasIgnoredRoles {
					return fmt.Sprintf("You cannot use the `remindme` command in %s", mention), nil
				}
			}

			when := time.Now().Add(offsetFromNow)
			_, err := NewReminder(parsed.Author.ID, parsed.GuildData.GS.ID, id, parsed.Args[1].Str(), when)
			if err != nil {
				return nil, err
			}

			durString := common.HumanizeDuration(common.DurationPrecisionSeconds, offsetFromNow)
			return fmt.Sprintf("Set a reminder in %s from now (<t:%d:f>)\nView reminders with the `reminders` command", durString, when.Unix()), nil
		},
	},
	{
		CmdCategory:         commands.CategoryTool,
		Name:                "Reminders",
		Description:         "Lists your active reminders in the server, use in DM to see all your reminders",
		SlashCommandEnabled: true,
		DefaultEnabled:      true,
		IsResponseEphemeral: true,
		RunInDM:             true,
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			uid := discordgo.StrID(parsed.Author.ID)
			qms := []qm.QueryMod{models.ReminderWhere.UserID.EQ(uid)}

			// if command used in server, only show reminders in that server
			var inServerSuffix string
			if inServer := parsed.GuildData != nil; inServer {
				inServerSuffix = " in this server"

				guildID := parsed.GuildData.GS.ID
				qms = append(qms, models.ReminderWhere.GuildID.EQ(guildID))
			}

			currentReminders, err := models.Reminders(qms...).AllG(parsed.Context())
			if err != nil {
				return nil, err
			}

			if len(currentReminders) == 0 {
				return fmt.Sprintf("You have no reminders%s. Create reminders with the `remindme` command", inServerSuffix), nil
			}

			out := fmt.Sprintf("Your reminders%s:\n", inServerSuffix)
			out += DisplayReminders(currentReminders, ModeDisplayUserReminders)
			out += "\nRemove a reminder with `delreminder/rmreminder (id)` where id is the first number for each reminder above.\nTo clear all reminders, use `delreminder` with the `-a` switch."
			return out, nil
		},
	},
	{
		CmdCategory:         commands.CategoryTool,
		Name:                "CReminders",
		Aliases:             []string{"channelreminders"},
		Description:         "Lists reminders in channel",
		RequireDiscordPerms: []int64{discordgo.PermissionManageChannels},
		SlashCommandEnabled: true,
		DefaultEnabled:      true,
		IsResponseEphemeral: true,
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			cid := discordgo.StrID(parsed.ChannelID)
			currentReminders, err := models.Reminders(models.ReminderWhere.ChannelID.EQ(cid)).AllG(parsed.Context())
			if err != nil {
				return nil, err
			}

			if len(currentReminders) == 0 {
				return "There are no reminders in this channel.", nil
			}

			out := "Reminders in this channel:\n"
			out += DisplayReminders(currentReminders, ModeDisplayChannelReminders)
			out += "\nRemove a reminder with `delreminder/rmreminder (id)` where id is the first number for each reminder above"
			return out, nil
		},
	},
	{
		CmdCategory:  commands.CategoryTool,
		Name:         "DelReminder",
		Aliases:      []string{"rmreminder"},
		Description:  "Deletes a reminder. You can delete reminders from other users provided you are running this command in the same guild the reminder was created in and have the Manage Channel permission in the channel the reminder was created in.",
		RequiredArgs: 0,
		RunInDM:      true,
		Arguments: []*dcmd.ArgDef{
			{Name: "ID", Type: dcmd.Int},
		},
		ArgSwitches: []*dcmd.ArgDef{
			{Name: "a", Help: "All"},
		},
		SlashCommandEnabled: true,
		DefaultEnabled:      true,
		IsResponseEphemeral: true,
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			if clearAll := parsed.Switch("a").Bool(); clearAll {
				uid := discordgo.StrID(parsed.Author.ID)
				count, err := models.Reminders(models.ReminderWhere.UserID.EQ(uid)).DeleteAllG(parsed.Context(), false /* hardDelete */)
				if err != nil {
					return "Error clearing reminders", err
				}

				if count == 0 {
					return "No reminders to clear", nil
				}
				return fmt.Sprintf("Cleared %d reminders", count), nil
			}

			if len(parsed.Args) == 0 || parsed.Args[0].Value == nil {
				return "No reminder ID provided", nil
			}

			reminder, err := models.FindReminderG(parsed.Context(), parsed.Args[0].Int())
			if err != nil {
				if err == sql.ErrNoRows {
					return "No reminder by that ID found", nil
				}
				return "Error retrieving reminder", err
			}

			// check perms
			if reminder.UserID != discordgo.StrID(parsed.Author.ID) {
				if reminder.GuildID != parsed.GuildData.GS.ID {
					return "You can only delete reminders that are not your own in the guild the reminder was originally created", nil
				}

				cid, _ := discordgo.ParseID(reminder.ChannelID)
				ok, err := bot.AdminOrPermMS(reminder.GuildID, cid, parsed.GuildData.MS, discordgo.PermissionManageChannels)
				if err != nil {
					return nil, err
				}
				if !ok {
					return "You need manage channel permission in the channel the reminder is in to delete reminders that are not your own", nil
				}
			}

			// just deleting from database is enough; we need not delete the
			// scheduled event since the handler will check database
			_, err = reminder.DeleteG(parsed.Context(), false /* hardDelete */)
			if err != nil {
				return nil, err
			}

			return fmt.Sprintf("Deleted reminder **#%d**: '%s'", reminder.ID, CutReminderShort(reminder.Message)), nil
		},
	},
}

func memberHasAnyRole(ms *dstate.MemberState, roles []int64) bool {
	for _, r := range ms.Member.Roles {
		if common.ContainsInt64Slice(roles, r) {
			return true
		}
	}
	return false
}

func checkUserScheduledEvent(evt *seventsmodels.ScheduledEvent, data interface{}) (retry bool, err error) {
	// IMPORTANT: evt.GuildID can be 1 in cases where it was migrated from the
	// legacy scheduled event system.

	userID := discordgo.StrID(*data.(*int64))
	reminders, err := models.Reminders(models.ReminderWhere.UserID.EQ(userID)).AllG(context.Background())
	if err != nil {
		return true, err
	}

	// TODO: can we move this filtering step into the database query?
	nowUnix := time.Now().Unix()
	for _, r := range reminders {
		if r.When <= nowUnix {
			err := TriggerReminder(r)
			if err != nil {
				// possibly try again
				return scheduledevents2.CheckDiscordErrRetry(err), err
			}
		}
	}

	return false, nil
}

func migrateLegacyScheduledEvents(t time.Time, data string) error {
	_, userID, ok := strings.Cut(data, ":")
	if !ok {
		logger.Error("invalid check user scheduled event: ", data)
		return nil
	}

	parsed, _ := discordgo.ParseID(userID)
	return scheduledevents2.ScheduleEvent("reminders_check_user", 1, t, parsed)
}
