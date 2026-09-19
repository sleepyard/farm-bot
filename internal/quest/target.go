// Package quest 从每日任务选出新手套牌目标双色字母。
package quest

import (
	"strings"

	"github.com/flourbrain/mtga-farm-bot/internal/playerlog"
)

// 公会 → 套牌双色字母（与原版 _GUILD_COLOR_MAP 对齐）。
var guildLetters = []struct {
	guild   string
	letters string
}{
	{"azorius", "WU"},
	{"dimir", "UB"},
	{"rakdos", "RB"},
	{"gruul", "RG"},
	{"selesnya", "GW"},
	{"orzhov", "WB"},
	{"izzet", "UR"},
	{"golgari", "BG"},
	{"boros", "RW"},
	{"simic", "UG"},
}

const colorLetters = "WUBRGC"

// Target 是一次任务解析结果。
type Target struct {
	Colors  string // 双色字母，如 "WB"；空=不换牌
	Reason  string
	AllDone bool
}

// ResolveTarget 从 Player.log 任务视图选出目标色。
// 规则：未完成公会任务里按金币最高；全部完成（有任务列表且均完成，或空列表且确认）→ WB；
// 无公会映射 → 空字符串（保留当前套牌）。
func ResolveTarget(quests []playerlog.Quest) Target {
	if len(quests) == 0 {
		// 无块时不强制换牌；调用方可再读日志。导航侧可将“确认无任务”视为全完成。
		return Target{Colors: "", Reason: "无任务数据，保留当前套牌"}
	}

	incomplete := make([]playerlog.Quest, 0, len(quests))
	for _, q := range quests {
		if q.Goal > 0 && q.Progress >= q.Goal {
			continue
		}
		incomplete = append(incomplete, q)
	}
	if len(incomplete) == 0 {
		return Target{Colors: "WB", Reason: "每日任务已全部完成，回退到胜率最高的黑白套牌", AllDone: true}
	}

	bestIdx := -1
	bestGold := -1
	for i, q := range incomplete {
		letters := lettersForLocKey(q.LocKey)
		if letters == "" {
			continue
		}
		if q.RewardGold > bestGold {
			bestGold = q.RewardGold
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		return Target{Colors: "", Reason: "进行中任务无颜色映射，保留当前套牌"}
	}
	q := incomplete[bestIdx]
	letters := lettersForLocKey(q.LocKey)
	return Target{
		Colors: letters,
		Reason: "最佳每日任务 → " + letters + "（金币 " + itoa(q.RewardGold) + "）",
	}
}

func lettersForLocKey(locKey string) string {
	lk := strings.ToLower(locKey)
	for _, g := range guildLetters {
		if strings.Contains(lk, g.guild) {
			return g.letters
		}
	}
	return ""
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// NormalizeDeckStem 规范化套牌文件名干（仅颜色字母，排序不强制）。
func NormalizeDeckStem(stem string) string {
	up := strings.ToUpper(stem)
	out := make([]byte, 0, len(up))
	for i := 0; i < len(up); i++ {
		c := up[i]
		if strings.ContainsRune(colorLetters, rune(c)) {
			out = append(out, c)
		}
	}
	return string(out)
}
