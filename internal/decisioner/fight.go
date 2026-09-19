package decisioner

import "github.com/flourbrain/mtga-farm-bot/internal/gamestate"

// tryFight 互斗类（由 cast 路径调用）。
func tryFight(snap gamestate.Snapshot, grpID int) bool {
	_, _ = snap, grpID
	// TODO: 移植 FightLogic
	return false
}
