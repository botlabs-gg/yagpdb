package yagstatus

import (
	"fmt"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
	"github.com/sirupsen/logrus"
	"runtime"
	"time"
)

var Command = &commands.YAGCommand{
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

	servers, _ := common.GetJoinedServerCount()

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

	botUser := common.BotUser

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
		},
	}

	for _, v := range common.Plugins {
		if cast, ok := v.(PluginStatus); ok {
			started := time.Now()
			name, val := cast.Status()
			if name == "" || val == "" {
				continue
			}
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: v.PluginInfo().Name + ": " + name, Value: val, Inline: true})
			elapsed := time.Since(started)
			logrus.Println("Took ", elapsed.Seconds(), " to gather stats from ", v.PluginInfo().Name)
		}
	}

	return embed, nil
	// return &commandsystem.FallbackEmebd{embed}, nil
}

type PluginStatus interface {
	Status() (string, string)
}
