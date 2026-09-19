package playerlog

// Snapshot 是一次按需读取 Player.log 的结果，供 UI / API 展示。
// 当前只解析每日任务与 canSwap；不再解析账户名 / 金币。
type Snapshot struct {
	OK      bool    `json:"ok"`
	Path    string  `json:"path"`
	LogSize int64   `json:"log_size"`
	Floor   int64   `json:"floor"`
	CanSwap *bool   `json:"can_swap"` // 仅字面 true 表示可刷新；缺省/false 视为否
	Quests  []Quest `json:"quests"`
	Warning string  `json:"warning,omitempty"`
}

// Quest 是 UI 友好的任务视图（由原始 QuestGetQuests JSON 映射而来）。
type Quest struct {
	ID         string `json:"id"`
	LocKey     string `json:"loc_key,omitempty"`
	Name       string `json:"name,omitempty"` // 由 locKey 还原，如 Simic Manipulator
	Colors     string `json:"colors"`
	Progress   int    `json:"progress"`
	Goal       int    `json:"goal"`
	RewardGold int    `json:"reward_gold"`
}

// QuestSnapshot 是从日志解析出的原始任务块。
type QuestSnapshot struct {
	Quests  []map[string]any
	CanSwap *bool // nil=未知/缺省；true/false 来自字面布尔
}
