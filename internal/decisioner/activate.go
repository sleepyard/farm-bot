package decisioner

import "github.com/flourbrain/mtga-farm-bot/internal/gamestate"

// tryActivate 启动式异能（Phoenix Chick / Skeleton 等）。首切片占位。
func (e *Engine) tryActivate(snap gamestate.Snapshot) *Move {
	_ = snap
	// TODO: 按牌名 / grpId 识别可启动异能
	return nil
}
