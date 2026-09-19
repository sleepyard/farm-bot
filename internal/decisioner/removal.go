package decisioner

import "github.com/flourbrain/mtga-farm-bot/internal/gamestate"

// tryRemoval 清除类施放评估（由 cast 路径调用；独立文件便于扩牌表）。
func tryRemoval(snap gamestate.Snapshot, grpID int) bool {
	_, _ = snap, grpID
	// TODO: 移植 RemovalLogic
	return false
}
