package decisioner

import (
	"fmt"

	"github.com/flourbrain/mtga-farm-bot/internal/gamestate"
)

const maxAllAttackTries = 3

// tryCombat 战斗步骤：宣告攻击前先确认无牌可出；全攻；对方有鹏洛克则全攻后再点头像打脸；三次仍停在宣告攻击则改点不攻击；宣告阻挡 → 不阻挡。
// 史迹模式同样永不阻挡（显式策略，避免日后加智能阻挡时误伤）。
func (e *Engine) tryCombat(snap gamestate.Snapshot) *Move {
	switch snap.Turn.Step {
	case gamestate.StepDeclareAttack:
		if e.hasPlayableBeforeAttack(snap) {
			return nil
		}
		if e.attackAllTries >= maxAllAttackTries {
			return &Move{Kind: KindNoAttacks, Reason: "三次全攻未过，点不攻击"}
		}
		e.attackAllTries++
		gate := e.noMorePlaysReason(snap)
		if snap.AttackTargetRequired {
			return &Move{
				Kind:   KindAssignAttackTargets,
				Reason: fmt.Sprintf("对方鹏洛克：全攻后点头像打脸（%d/%d，%s）", e.attackAllTries, maxAllAttackTries, gate),
			}
		}
		return &Move{Kind: KindAllAttack, Reason: fmt.Sprintf("宣告攻击：全攻（%d/%d，%s）", e.attackAllTries, maxAllAttackTries, gate)}
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

func (e *Engine) hasPlayableBeforeAttack(snap gamestate.Snapshot) bool {
	return e.tryPlayLand(snap) != nil || e.tryCast(snap) != nil
}

func (e *Engine) noMorePlaysReason(snap gamestate.Snapshot) string {
	pool := AvailableMana(snap.Actions)
	hasCast, affordable := false, false
	for _, a := range snap.Actions {
		if a.ActionType != gamestate.ActionCast || a.InstanceID == 0 {
			continue
		}
		hasCast = true
		cost, ok := actionCastCost(a)
		if ok && CanAfford(cost, pool) {
			affordable = true
		}
	}
	if affordable {
		return "仍有可出的牌"
	}
	if hasCast {
		return "有可出的牌但法力不足"
	}
	if pool.Total > 0 {
		return "有剩余法力但无可出的牌"
	}
	return "无可出的牌"
}
