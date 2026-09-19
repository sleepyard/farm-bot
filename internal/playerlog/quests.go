package playerlog

import (
	"encoding/json"
	"strconv"
	"strings"
)

// 公会双色中文映射（由 locKey 中的公会英文名匹配）。
var guildColorMap = []struct {
	guild  string
	colors string
}{
	{"azorius", "蓝白"},
	{"dimir", "蓝黑"},
	{"rakdos", "红黑"},
	{"gruul", "红绿"},
	{"selesnya", "白绿"},
	{"orzhov", "黑白"},
	{"izzet", "红蓝"},
	{"golgari", "黑绿"},
	{"boros", "红白"},
	{"simic", "蓝绿"},
}

// ParseLatestQuests 定位文本中最后一次 "quests" 并 JSON 解码其所属对象。
func ParseLatestQuests(text string) (*QuestSnapshot, error) {
	idx := strings.LastIndex(text, `"quests"`)
	if idx < 0 {
		return nil, nil
	}
	start := strings.LastIndex(text[:idx], "{")
	if start < 0 {
		return nil, nil
	}

	dec := json.NewDecoder(strings.NewReader(text[start:]))
	var payload map[string]any
	if err := dec.Decode(&payload); err != nil {
		return nil, nil // 与原版一致：解析失败当作无块
	}
	rawQuests, ok := payload["quests"]
	if !ok {
		return nil, nil
	}
	list, ok := rawQuests.([]any)
	if !ok {
		return nil, nil
	}

	quests := make([]map[string]any, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, nil
		}
		quests = append(quests, m)
	}

	snap := &QuestSnapshot{Quests: quests}
	// Arena 序列化常省略 false；只有字面 true 才能授权换任务。
	if v, exists := payload["canSwap"]; exists {
		if b, ok := v.(bool); ok {
			snap.CanSwap = &b
		}
	}
	return snap, nil
}

// BuildQuestView 把原始 quest map 转成 UI 视图（颜色、locKey 名称、进度、金币）。
func BuildQuestView(raw []map[string]any) []Quest {
	out := make([]Quest, 0, len(raw))
	for _, q := range raw {
		locKey, _ := q["locKey"].(string)
		gold := 0
		if chest, ok := q["chestDescription"].(map[string]any); ok {
			if params, ok := chest["locParams"].(map[string]any); ok {
				gold = anyToInt(params["number1"])
			}
		}
		out = append(out, Quest{
			ID:         anyToString(q["questId"]),
			LocKey:     locKey,
			Name:       questDisplayName(locKey),
			Colors:     questColorsForLocKey(locKey),
			Progress:   anyToInt(q["endingProgress"]),
			Goal:       anyToInt(q["goal"]),
			RewardGold: gold,
		})
	}
	return out
}

func questColorsForLocKey(locKey string) string {
	lk := strings.ToLower(locKey)
	for _, g := range guildColorMap {
		if strings.Contains(lk, g.guild) {
			return g.colors
		}
	}
	return ""
}

// questDisplayName 把 locKey 还原成日志里的任务名：
// Quests/Quest_Simic_Manipulator → Simic Manipulator
func questDisplayName(locKey string) string {
	stem := strings.TrimSpace(locKey)
	if i := strings.LastIndex(stem, "/"); i >= 0 {
		stem = stem[i+1:]
	}
	if len(stem) >= 6 && strings.EqualFold(stem[:6], "quest_") {
		stem = stem[6:]
	}
	stem = strings.ReplaceAll(stem, "_", " ")
	return strings.TrimSpace(stem)
}

func anyToInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	default:
		return 0
	}
}

func anyToString(v any) string {
	switch s := v.(type) {
	case string:
		return s
	default:
		return ""
	}
}
