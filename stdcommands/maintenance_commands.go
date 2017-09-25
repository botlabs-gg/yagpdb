package stdcommands

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/scheduledevents"
	"github.com/mediocregopher/radix.v2/redis"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
	"runtime"
	"sort"
	"time"
)

func requireOwner(inner commandsystem.RunFunc) commandsystem.RunFunc {
	return func(data *commandsystem.ExecData) (interface{}, error) {
		if data.Message.Author.ID != common.Conf.Owner {
			return "", nil
		}

		return inner(data)
	}
}

var maintenanceCommands = []commandsystem.CommandHandler{
	cmdStateInfo,
	cmdSecretCommand,
	cmdLeaveServer,
	cmdBanServer,
	cmdUnbanServer,
	cmdTopServers,
	cmdTopCommands,
	cmdTopEvents,
	cmdCurrentShard,
	cmdMemberFetcher,
	cmdYagStatus,
}

var cmdStateInfo = &commands.CustomCommand{
	Cooldown:             2,
	Category:             commands.CategoryDebug,
	HideFromCommandsPage: true,
	Command: &commandsystem.Command{
		Name:         "stateinfo",
		Description:  "Responds with state debug info",
		HideFromHelp: true,
		Run:          cmdFuncStateInfo,
	},
}

func cmdFuncStateInfo(data *commandsystem.ExecData) (interface{}, error) {
	totalGuilds := 0
	totalMembers := 0
	totalChannels := 0
	totalMessages := 0

	state := bot.State
	state.RLock()
	defer state.RUnlock()

	totalGuilds = len(state.Guilds)

	for _, g := range state.Guilds {

		state.RUnlock()
		g.RLock()

		totalChannels += len(g.Channels)
		totalMembers += len(g.Members)

		for _, cState := range g.Channels {
			totalMessages += len(cState.Messages)
		}
		g.RUnlock()
		state.RLock()
	}

	embed := &discordgo.MessageEmbed{
		Title: "State size",
		Fields: []*discordgo.MessageEmbedField{
			&discordgo.MessageEmbedField{Name: "Guilds", Value: fmt.Sprint(totalGuilds), Inline: true},
			&discordgo.MessageEmbedField{Name: "Members", Value: fmt.Sprintf("%d", totalMembers), Inline: true},
			&discordgo.MessageEmbedField{Name: "Messages", Value: fmt.Sprintf("%d", totalMessages), Inline: true},
			&discordgo.MessageEmbedField{Name: "Channels", Value: fmt.Sprintf("%d", totalChannels), Inline: true},
		},
	}

	return embed, nil
}

var cmdSecretCommand = &commands.CustomCommand{
	Cooldown:             2,
	Category:             commands.CategoryDebug,
	HideFromCommandsPage: true,
	Command: &commandsystem.Command{
		Name:         "secretcommand",
		Description:  ";))",
		HideFromHelp: true,
		Run: requireOwner(func(data *commandsystem.ExecData) (interface{}, error) {
			return "<@" + common.Conf.Owner + "> Is my owner", nil
		}),
	},
}

var cmdLeaveServer = &commands.CustomCommand{
	Cooldown:             2,
	Category:             commands.CategoryDebug,
	HideFromCommandsPage: true,
	Command: &commandsystem.Command{
		Name:         "leaveserver",
		Description:  ";))",
		HideFromHelp: true,
		RequiredArgs: 1,
		Arguments: []*commandsystem.ArgDef{
			{Name: "server", Type: commandsystem.ArgumentString},
		},
		Run: requireOwner(func(data *commandsystem.ExecData) (interface{}, error) {
			err := common.BotSession.GuildLeave(data.Args[0].Str())
			if err == nil {
				return "Left " + data.Args[0].Str(), nil
			}
			return err, err
		}),
	},
}
var cmdBanServer = &commands.CustomCommand{
	Cooldown:             2,
	Category:             commands.CategoryDebug,
	HideFromCommandsPage: true,
	Command: &commandsystem.Command{
		Name:         "banserver",
		Description:  ";))",
		HideFromHelp: true,
		RequiredArgs: 1,
		Arguments: []*commandsystem.ArgDef{
			{Name: "server", Type: commandsystem.ArgumentString},
		},
		Run: requireOwner(func(data *commandsystem.ExecData) (interface{}, error) {
			err := common.BotSession.GuildLeave(data.Args[0].Str())
			if err == nil {
				client := data.Context().Value(commands.CtxKeyRedisClient).(*redis.Client)
				client.Cmd("SADD", "banned_servers", data.Args[0].Str())

				return "Banned " + data.Args[0].Str(), nil
			}
			return err, err
		}),
	},
}

var cmdUnbanServer = &commands.CustomCommand{
	Cooldown:             2,
	Category:             commands.CategoryDebug,
	HideFromCommandsPage: true,
	Command: &commandsystem.Command{
		Name:         "unbanserver",
		Description:  ";))",
		HideFromHelp: true,
		RequiredArgs: 1,
		Arguments: []*commandsystem.ArgDef{
			{Name: "server", Type: commandsystem.ArgumentString},
		},
		Run: requireOwner(func(data *commandsystem.ExecData) (interface{}, error) {
			client := data.Context().Value(commands.CtxKeyRedisClient).(*redis.Client)
			unbanned, err := client.Cmd("SREM", "banned_servers", data.Args[0].Str()).Int()
			if err != nil {
				return err, err
			}

			if unbanned < 1 {
				return "Server wasnt banned", nil
			}

			return "Unbanned server", nil
		}),
	},
}

var cmdTopCommands = &commands.CustomCommand{
	Cooldown:             2,
	Category:             commands.CategoryDebug,
	HideFromCommandsPage: true,
	Command: &commandsystem.Command{
		Name:         "topcommands",
		Description:  "Shows command usage stats",
		HideFromHelp: true,
		Arguments: []*commandsystem.ArgDef{
			{Name: "hours", Type: commandsystem.ArgumentNumber, Default: float64(1)},
		},
		Run: cmdFuncTopCommands,
	},
}

func cmdFuncTopCommands(data *commandsystem.ExecData) (interface{}, error) {
	hours := data.Args[0].Int()
	within := time.Now().Add(time.Duration(-hours) * time.Hour)

	var results []*TopCommandsResult
	err := common.GORM.Table(common.LoggedExecutedCommand{}.TableName()).Select("command, COUNT(id)").Where("created_at > ?", within).Group("command").Order("count(id) desc").Scan(&results).Error
	if err != nil {
		return "Uh oh something bad happened", err
	}

	out := fmt.Sprintf("```\nCommand stats from now to %d hour(s) ago\n#    Total -  Command\n", hours)
	total := 0
	for k, result := range results {
		out += fmt.Sprintf("#%02d: %5d - %s\n", k+1, result.Count, result.Command)
		total += result.Count
	}

	cpm := float64(total) / float64(hours) / 60

	out += fmt.Sprintf("\nTotal: %d, Commands per minute: %.1f", total, cpm)
	out += "\n```"

	return out, nil
}

var cmdTopEvents = &commands.CustomCommand{
	Cooldown:             2,
	Category:             commands.CategoryDebug,
	HideFromCommandsPage: true,
	Command: &commandsystem.Command{
		Name:         "topevents",
		Description:  "Shows gateway event processing stats",
		HideFromHelp: true,
		Run:          cmdFuncTopEvents,
	},
}

func cmdFuncTopEvents(data *commandsystem.ExecData) (interface{}, error) {

	bot.EventLogger.Lock()

	sortable := make([]*DiscordEvtEntry, len(bot.EventLogger.Events))

	i := 0
	for k, v := range bot.EventLogger.Events {
		sortable[i] = &DiscordEvtEntry{
			Name:  k,
			Count: v,
		}
		i++
	}

	bot.EventLogger.Unlock()

	sort.Sort(DiscordEvtEntrySortable(sortable))

	uptime := time.Since(bot.Started)

	out := "```\n#   Total  -  Avg/m  - Event\n"
	total := 0
	for k, entry := range sortable {
		out += fmt.Sprintf("#%-2d: %5d - %5.2f - %s\n", k+1, entry.Count, float64(entry.Count)/(uptime.Seconds()/60), entry.Name)
		total += entry.Count
	}

	epm := float64(total) / (uptime.Seconds() / 60)

	out += fmt.Sprintf("\nTotal: %d, Events per minute: %.1f", total, epm)
	out += "\n```"

	return out, nil
}

var cmdCurrentShard = &commands.CustomCommand{
	Category:             commands.CategoryDebug,
	HideFromCommandsPage: true,
	Command: &commandsystem.Command{
		Name:        "CurentShard",
		Aliases:     []string{"cshard"},
		Description: "Shows the current shard this server is on",
		Run: func(data *commandsystem.ExecData) (interface{}, error) {
			shard := bot.ShardManager.SessionForGuildS(data.Guild.ID())
			return fmt.Sprintf("On shard %d out of total %d shards.", shard.ShardID+1, shard.ShardCount), nil
		},
	},
}

var cmdMemberFetcher = &commands.CustomCommand{
	Category:             commands.CategoryDebug,
	HideFromCommandsPage: true,
	Command: &commandsystem.Command{
		Name:        "MemberFetcher",
		Aliases:     []string{"memfetch"},
		Description: "Shows the current status of the member fetcher",
		Run: func(data *commandsystem.ExecData) (interface{}, error) {
			fetching, notFetching := bot.MemberFetcher.Status()
			return fmt.Sprintf("Fetching: `%d`, Not fetching: `%d`", fetching, notFetching), nil

		},
	},
}

var cmdYagStatus = &commands.CustomCommand{
	Cooldown: 5,
	Category: commands.CategoryDebug,
	Command: &commandsystem.Command{
		Name:        "Yagstatus",
		Aliases:     []string{"Status"},
		Description: "Shows yagpdb status, version, uptime, memory stats and so on",
		RunInDm:     true,
		Run:         cmdFuncYagStatus,
	},
}

func cmdFuncYagStatus(data *commandsystem.ExecData) (interface{}, error) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	bot.State.RLock()
	servers := len(bot.State.Guilds)
	bot.State.RUnlock()

	sysMem, err := mem.VirtualMemory()
	sysMemStats := ""
	if err == nil {
		sysMemStats = fmt.Sprintf("%dMB (%.0f%%), %dMB", sysMem.Used/1000000, sysMem.UsedPercent, sysMem.Total/1000000)
	} else {
		sysMemStats = "Failed collecting mem stats"
		logrus.WithError(err).Error("Failed collecting memory stats")
	}

	sysLoad, err := load.Avg()
	sysLoadStats := ""
	if err == nil {
		sysLoadStats = fmt.Sprintf("%.2f, %.2f, %.2f", sysLoad.Load1, sysLoad.Load5, sysLoad.Load15)
	} else {
		sysLoadStats = "Failed collecting"
		logrus.WithError(err).Error("Failed collecting load stats")
	}

	uptime := time.Since(bot.Started)

	allocated := float64(memStats.Alloc) / 1000000

	numGoroutines := runtime.NumGoroutine()

	numScheduledEvent, _ := scheduledevents.NumScheduledEvents(data.Context().Value(commands.CtxKeyRedisClient).(*redis.Client))

	botUser := bot.State.User(true)

	embed := &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name:    botUser.Username,
			IconURL: discordgo.EndpointUserAvatar(botUser.ID, botUser.Avatar),
		},
		Title: "YAGPDB Status, version " + common.VERSION,
		Fields: []*discordgo.MessageEmbedField{
			&discordgo.MessageEmbedField{Name: "Servers", Value: fmt.Sprint(servers), Inline: true},
			&discordgo.MessageEmbedField{Name: "Go version", Value: runtime.Version(), Inline: true},
			&discordgo.MessageEmbedField{Name: "Uptime", Value: common.HumanizeDuration(common.DurationPrecisionSeconds, uptime), Inline: true},
			&discordgo.MessageEmbedField{Name: "Goroutines", Value: fmt.Sprint(numGoroutines), Inline: true},
			&discordgo.MessageEmbedField{Name: "GC Pause Fraction", Value: fmt.Sprintf("%.3f%%", memStats.GCCPUFraction*100), Inline: true},
			&discordgo.MessageEmbedField{Name: "Process Mem (alloc, sys, freed)", Value: fmt.Sprintf("%.1fMB, %.1fMB, %.1fMB", float64(memStats.Alloc)/1000000, float64(memStats.Sys)/1000000, (float64(memStats.TotalAlloc)/1000000)-allocated), Inline: true},
			&discordgo.MessageEmbedField{Name: "System Mem (used, total)", Value: sysMemStats, Inline: true},
			&discordgo.MessageEmbedField{Name: "System load (1, 5, 15)", Value: sysLoadStats, Inline: true},
			&discordgo.MessageEmbedField{Name: "Scheduled events (reminders etc)", Value: fmt.Sprint(numScheduledEvent), Inline: true},
		},
	}

	for _, v := range common.Plugins {
		if cast, ok := v.(PluginStatus); ok {
			name, val := cast.Status(data.Context().Value(commands.CtxKeyRedisClient).(*redis.Client))
			if name == "" || val == "" {
				continue
			}
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: v.Name() + ": " + name, Value: val, Inline: true})
		}
	}

	return &commandsystem.FallbackEmebd{embed}, nil
}

var cmdTopServers = &commands.CustomCommand{
	Cooldown: 5,
	Category: commands.CategoryFun,
	Command: &commandsystem.Command{
		Name:        "TopServers",
		Description: "Responds with the top 15 servers im on",

		Run: func(data *commandsystem.ExecData) (interface{}, error) {
			state := bot.State
			state.RLock()

			guilds := make([]*discordgo.Guild, len(state.Guilds))
			i := 0
			for _, v := range state.Guilds {
				state.RUnlock()
				guilds[i] = v.LightCopy(true)
				state.RLock()
				i++
			}
			state.RUnlock()

			sortable := GuildsSortUsers(guilds)
			sort.Sort(sortable)

			out := "```"
			for k, v := range sortable {
				if k > 14 {
					break
				}

				out += fmt.Sprintf("\n#%-2d: %-25s (%d members)", k+1, v.Name, v.MemberCount)
			}
			return "Top servers the bot is on (membercount):\n" + out + "\n```", nil
		},
	},
}

type TopCommandsResult struct {
	Command string
	Count   int
}

type DiscordEvtEntry struct {
	Name  string
	Count int
}

type DiscordEvtEntrySortable []*DiscordEvtEntry

func (d DiscordEvtEntrySortable) Len() int {
	return len(d)
}

func (d DiscordEvtEntrySortable) Less(i, j int) bool {
	return d[i].Count > d[j].Count
}

func (d DiscordEvtEntrySortable) Swap(i, j int) {
	temp := d[i]
	d[i] = d[j]
	d[j] = temp
}
