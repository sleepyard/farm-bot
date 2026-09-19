package decisioner

import "github.com/flourbrain/mtga-farm-bot/internal/gamestate"

// tryCounter 堆叠上有对方咒语且可支付反击时返回施放。首切片占位。
func (e *Engine) tryCounter(snap gamestate.Snapshot) *Move {
	_ = snap
	// TODO: 移植 CounterLogic
	return nil
}
