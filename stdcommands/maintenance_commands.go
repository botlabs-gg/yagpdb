package stdcommands

import (
	"fmt"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/scheduledevents"
	"github.com/mediocregopher/radix.v2/redis"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
	"github.com/sirupsen/logrus"
	"runtime"
	"sort"
	"time"
)

func requireOwner(inner dcmd.RunFunc) dcmd.RunFunc {
	return func(data *dcmd.Data) (interface{}, error) {
		if data.Msg.Author.ID != common.Conf.Owner {
			return "", nil
		}

		return inner(data)
	}
}

var maintenanceCommands = []*commands.YAGCommand{
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

var cmdStateInfo = &commands.YAGCommand{
	Cooldown:             2,
	CmdCategory:          commands.CategoryDebug,
	HideFromCommandsPage: true,
	Name:                 "stateinfo",
	Description:          "Responds with state debug info",
	HideFromHelp:         true,
	RunFunc:              cmdFuncStateInfo,
}

func cmdFuncStateInfo(data *dcmd.Data) (interface{}, error) {
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

var cmdSecretCommand = &commands.YAGCommand{
	Cooldown:             2,
	CmdCategory:          commands.CategoryDebug,
	HideFromCommandsPage: true,
	Name:                 "secretcommand",
	Description:          ";))",
	HideFromHelp:         true,
	RunFunc: requireOwner(func(data *dcmd.Data) (interface{}, error) {
		return "<@" + discordgo.StrID(common.Conf.Owner) + "> Is my owner", nil
	}),
}

var cmdLeaveServer = &commands.YAGCommand{
	Cooldown:             2,
	CmdCategory:          commands.CategoryDebug,
	HideFromCommandsPage: true,
	Name:                 "leaveserver",
	Description:          ";))",
	HideFromHelp:         true,
	RequiredArgs:         1,
	Arguments: []*dcmd.ArgDef{
		{Name: "server", Type: dcmd.Int},
	},
	RunFunc: requireOwner(func(data *dcmd.Data) (interface{}, error) {
		err := common.BotSession.GuildLeave(data.Args[0].Int64())
		if err == nil {
			return "Left " + data.Args[0].Str(), nil
		}
		return err, err
	}),
}
var cmdBanServer = &commands.YAGCommand{
	Cooldown:             2,
	CmdCategory:          commands.CategoryDebug,
	HideFromCommandsPage: true,
	Name:                 "banserver",
	Description:          ";))",
	HideFromHelp:         true,
	RequiredArgs:         1,
	Arguments: []*dcmd.ArgDef{
		{Name: "server", Type: dcmd.Int},
	},
	RunFunc: requireOwner(func(data *dcmd.Data) (interface{}, error) {
		err := common.BotSession.GuildLeave(data.Args[0].Int64())
		if err == nil {
			client := data.Context().Value(commands.CtxKeyRedisClient).(*redis.Client)
			client.Cmd("SADD", "banned_servers", data.Args[0].Str())

			return "Banned " + data.Args[0].Str(), nil
		}
		return err, err
	}),
}

var cmdUnbanServer = &commands.YAGCommand{
	Cooldown:             2,
	CmdCategory:          commands.CategoryDebug,
	HideFromCommandsPage: true,
	Name:                 "unbanserver",
	Description:          ";))",
	HideFromHelp:         true,
	RequiredArgs:         1,
	Arguments: []*dcmd.ArgDef{
		{Name: "server", Type: dcmd.String},
	},
	RunFunc: requireOwner(func(data *dcmd.Data) (interface{}, error) {
		client := data.Context().Value(commands.CtxKeyRedisClient).(*redis.Client)
		unbanned, err := client.Cmd("SREM", "banned_servers", data.Args[0].Str()).Int()
		if err != nil {
			return err, err
		}

		if unbanned < 1 {
			return "Server wasn't banned", nil
		}

		return "Unbanned server", nil
	}),
}

var cmdTopCommands = &commands.YAGCommand{
	Cooldown:             2,
	CmdCategory:          commands.CategoryDebug,
	HideFromCommandsPage: true,
	Name:                 "topcommands",
	Description:          "Shows command usage stats",
	HideFromHelp:         true,
	Arguments: []*dcmd.ArgDef{
		{Name: "hours", Type: dcmd.Int, Default: 1},
	},
	RunFunc: cmdFuncTopCommands,
}

func cmdFuncTopCommands(data *dcmd.Data) (interface{}, error) {
	hours := data.Args[0].Int()
	within := time.Now().Add(time.Duration(-hours) * time.Hour)

	var results []*TopCommandsResult
	err := common.GORM.Table(common.LoggedExecutedCommand{}.TableName()).Select("command, COUNT(id)").Where("created_at > ?", within).Group("command").Order("count(id) desc").Scan(&results).Error
	if err != nil {
		return "Uh oh... Something bad happened :'(", err
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

var cmdTopEvents = &commands.YAGCommand{
	Cooldown:             2,
	CmdCategory:          commands.CategoryDebug,
	HideFromCommandsPage: true,
	Name:                 "topevents",
	Description:          "Shows gateway event processing stats for all or one shard",
	HideFromHelp:         true,
	Arguments: []*dcmd.ArgDef{
		{Name: "shard", Type: dcmd.Int},
	},
	RunFunc: cmdFuncTopEvents,
}

func cmdFuncTopEvents(data *dcmd.Data) (interface{}, error) {

	shardsTotal, lastPeriod := bot.EventLogger.GetStats()

	sortable := make([]*DiscordEvtEntry, len(eventsystem.AllDiscordEvents))
	for i, _ := range sortable {
		sortable[i] = &DiscordEvtEntry{
			Name: eventsystem.Event(i).String(),
		}
	}

	for i, _ := range shardsTotal {
		if data.Args[0].Value != nil && data.Args[0].Int() != i {
			continue
		}

		for de, j := range eventsystem.AllDiscordEvents {
			sortable[de].Total += shardsTotal[i][j]
			sortable[de].PerSecond += float64(lastPeriod[i][j]) / bot.EventLoggerPeriodDuration.Seconds()
		}
	}

	sort.Sort(DiscordEvtEntrySortable(sortable))

	out := "Total event stats across all shards:\n"
	if data.Args[0].Value != nil {
		out = fmt.Sprintf("Stats for shard %d:\n", data.Args[0].Int())
	}

	out += "```\n#     Total  -   /s  - Event\n"
	sum := int64(0)
	sumPerSecond := float64(0)
	for k, entry := range sortable {
		out += fmt.Sprintf("#%-2d: %7d - %5.1f - %s\n", k+1, entry.Total, entry.PerSecond, entry.Name)
		sum += entry.Total
		sumPerSecond += entry.PerSecond
	}

	out += fmt.Sprintf("\nTotal: %d, Events per minute: %.1f", sum, sumPerSecond)
	out += "\n```"

	return out, nil
}

var cmdCurrentShard = &commands.YAGCommand{
	CmdCategory:          commands.CategoryDebug,
	HideFromCommandsPage: true,
	Name:                 "CurentShard",
	Aliases:              []string{"cshard"},
	Description:          "Shows the current shard this server is on",
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		shard := bot.ShardManager.SessionForGuild(data.GS.ID())
		return fmt.Sprintf("On shard %d out of total %d shards.", shard.ShardID+1, shard.ShardCount), nil
	},
}

var cmdMemberFetcher = &commands.YAGCommand{
	CmdCategory:          commands.CategoryDebug,
	HideFromCommandsPage: true,
	Name:                 "MemberFetcher",
	Aliases:              []string{"memfetch"},
	Description:          "Shows the current status of the member fetcher",
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		fetching, notFetching := bot.MemberFetcher.Status()
		return fmt.Sprintf("Fetching: `%d`, Not fetching: `%d`", fetching, notFetching), nil
	},
}

var cmdYagStatus = &commands.YAGCommand{
	Cooldown:    5,
	CmdCategory: commands.CategoryDebug,
	Name:        "Yagstatus",
	Aliases:     []string{"status"},
	Description: "Shows yagpdb status, version, uptime, memory stats, and so on",
	RunInDM:     true,
	RunFunc:     cmdFuncYagStatus,
}

func cmdFuncYagStatus(data *dcmd.Data) (interface{}, error) {
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

	return embed, nil
	// return &commandsystem.FallbackEmebd{embed}, nil
}

var cmdTopServers = &commands.YAGCommand{
	Cooldown:    5,
	CmdCategory: commands.CategoryFun,
	Name:        "TopServers",
	Description: "Responds with the top 15 servers I'm on",

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
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
		return "Top servers the bot is on (by membercount):\n" + out + "\n```", nil
	},
}

type TopCommandsResult struct {
	Command string
	Count   int
}

type DiscordEvtEntry struct {
	Name      string
	Total     int64
	PerSecond float64
}

type DiscordEvtEntrySortable []*DiscordEvtEntry

func (d DiscordEvtEntrySortable) Len() int {
	return len(d)
}

func (d DiscordEvtEntrySortable) Less(i, j int) bool {
	return d[i].Total > d[j].Total
}

func (d DiscordEvtEntrySortable) Swap(i, j int) {
	temp := d[i]
	d[i] = d[j]
	d[j] = temp
}
