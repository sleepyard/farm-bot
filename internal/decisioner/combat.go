package decisioner

import (
	"fmt"

	"github.com/flourbrain/mtga-farm-bot/internal/gamestate"
)

const maxAllAttackTries = 3

// tryCombat 战斗步骤：宣告攻击 → 全攻；三次仍停在宣告攻击则改点不攻击；宣告阻挡 → 不阻挡。
// 史迹模式同样永不阻挡（显式策略，避免日后加智能阻挡时误伤）。
func (e *Engine) tryCombat(snap gamestate.Snapshot) *Move {
	switch snap.Turn.Step {
	case gamestate.StepDeclareAttack:
		if e.attackAllTries >= maxAllAttackTries {
			return &Move{Kind: KindNoAttacks, Reason: "三次全攻未过，点不攻击"}
		}
		e.attackAllTries++
		return &Move{Kind: KindAllAttack, Reason: fmt.Sprintf("宣告攻击：全攻（%d/%d）", e.attackAllTries, maxAllAttackTries)}
	case gamestate.StepDeclareBlock:
		reason := "宣告阻挡：不阻挡"
		if e.isHistoric() {
			reason = "史迹模式：不阻挡"
		}
		return &Move{Kind: KindNoBlocks, Reason: reason}
	default:
		return nil
	}
}
