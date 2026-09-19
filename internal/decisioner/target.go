package decisioner

import (
	"fmt"
	"strings"

	"github.com/flourbrain/mtga-farm-bot/internal/carddb"
	"github.com/flourbrain/mtga-farm-bot/internal/gamestate"
)

// 选目标意图（无 oracle 时用牌名 + 合法目标几何推断）。
type targetIntent int

const (
	intentUnknown targetIntent = iota
	intentBuff                 // 有益：只能/应点己方生物
	intentHarm                 // 伤害/放逐/消灭：点对方生物（或打脸）
)

// trySelectTarget 选目标：有益→己方生物；伤害/放逐→对方生物；否则可打脸。
// 史迹模式：战场目标一律点对手头像；墓地/堆叠选牌仍走对应区域（Zombify / 反击）。
func (e *Engine) trySelectTarget(snap gamestate.Snapshot) *Move {
	if snap.Turn.Step == gamestate.StepDeclareAttack && snap.AttackTargetRequired && snap.SelectTarget == nil {
		return nil
	}
	if !snap.NeedsSpellTarget() {
		return nil
	}
	if p := snap.SelectTarget; p != nil {
		if m := pickChooserOrStack(snap, p); m != nil {
			return m
		}
	}
	if e.isHistoric() {
		return &Move{
			Kind:    KindSelectTarget,
			Targets: []int{-1},
			Reason:  "史迹模式：目标一律对手头像",
		}
	}
	if p := snap.SelectTarget; p != nil {
		return pickTargetMove(snap, p)
	}
	// 仅有 AAR Target 标记、无 SelectTargetsReq 细节时：保守点脸
	return &Move{
		Kind:    KindSelectTarget,
		Targets: []int{-1},
		Reason:  "选择目标：对手玩家（无合法目标列表）",
	}
}

func pickTargetMove(snap gamestate.Snapshot, p *gamestate.SelectTargetPrompt) *Move {
	srcGrp := grpOf(snap, p.SourceID)
	srcName := carddb.NameByGrp(srcGrp)
	intent := classifyTargetIntent(srcGrp, srcName)

	// 1) 明确有益 / 合法目标只有己方生物 → 己方最强攻击者
	onlyOwn := len(p.OwnCreatures) > 0 && len(p.OppCreatures) == 0 && !p.FaceLegal
	if intent == intentBuff || (onlyOwn && intent != intentHarm) {
		if id := bestOwnCreature(snap, p.OwnCreatures); id > 0 {
			return targetCreatureMove(id, srcGrp, srcName, true,
				fmt.Sprintf("有益目标：己方生物 %s", carddb.FormatInstance(id, grpOf(snap, id))))
		}
		if intent == intentBuff {
			return &Move{Kind: KindWait, Reason: "有益法术但无己方法定生物目标"}
		}
	}

	// 2) 伤害/放逐/消灭 → 对方生物；无可点则打脸
	if intent == intentHarm || len(p.OppCreatures) > 0 {
		if id := bestOppCreature(snap, p.OppCreatures); id > 0 {
			return targetCreatureMove(id, srcGrp, srcName, false,
				fmt.Sprintf("伤害/清除目标：对方生物 %s", carddb.FormatInstance(id, grpOf(snap, id))))
		}
		if p.FaceLegal {
			return targetFaceMove(srcName, "清除/伤害：打对手脸")
		}
		if intent == intentHarm {
			return &Move{Kind: KindWait, Reason: "伤害法术无可合法对方目标（拒点己方）"}
		}
	}

	// 3) 未知意图：脸合法优先打脸；否则己方；再否则对方
	if p.FaceLegal {
		return targetFaceMove(srcName, "选择目标：对手玩家")
	}
	if id := bestOwnCreature(snap, p.OwnCreatures); id > 0 {
		return targetCreatureMove(id, srcGrp, srcName, true,
			fmt.Sprintf("选择目标：己方生物 %s", carddb.FormatInstance(id, grpOf(snap, id))))
	}
	if id := bestOppCreature(snap, p.OppCreatures); id > 0 {
		return targetCreatureMove(id, srcGrp, srcName, false,
			fmt.Sprintf("选择目标：对方生物 %s", carddb.FormatInstance(id, grpOf(snap, id))))
	}
	return targetFaceMove(srcName, "选择目标：对手玩家（兜底）")
}

func targetCreatureMove(id, srcGrp int, srcName string, friendly bool, reason string) *Move {
	_ = srcGrp
	return &Move{
		Kind:           KindSelectTarget,
		InstanceID:     id,
		GrpID:          0,
		CardName:       srcName,
		Targets:        []int{id},
		FriendlyTarget: friendly,
		Reason:         reason,
	}
}

func targetFaceMove(srcName, reason string) *Move {
	return &Move{
		Kind:     KindSelectTarget,
		CardName: srcName,
		Targets:  []int{-1},
		Reason:   reason,
	}
}

func classifyTargetIntent(grpID int, name string) targetIntent {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		if db := carddb.Global(); db != nil {
			if c, ok := db.Get(grpID); ok {
				n = strings.ToLower(c.Name)
			}
		}
	}
	// 已知有益 / 泵类牌名关键词
	buffHints := []string{
		"courage", "bulk up", "fake your own death", "undying", "giant growth",
		"titanic", "might", "angelic destiny", "ethereal armor", "unflinching",
		"bless", "inspire", "valor", "hexproof", "indestructible", "regenerate",
		"your own",
	}
	for _, h := range buffHints {
		if strings.Contains(n, h) {
			return intentBuff
		}
	}
	harmHints := []string{
		"destroy", "exile", "shock", "lightning", "bolt", "murder", "doom",
		"smite", "mortify", "banish", "assassinate", "terminate", "cut down",
		"go for the throat", "flame", "fireball", "blaze", "burn", "blast",
		"pierce", "strike", "rage", "infernal", "kill", "wrath", "defeat",
		"eliminate", "obliterate", "annihilate", "bite", "fight",
	}
	for _, h := range harmHints {
		if strings.Contains(n, h) {
			return intentHarm
		}
	}
	// Aura 且无伤害关键词 → 倾向有益（贴己方）
	if db := carddb.Global(); db != nil {
		if c, ok := db.Get(grpID); ok {
			for _, t := range c.Types {
				tl := strings.ToLower(t)
				if strings.Contains(tl, "aura") || strings.Contains(tl, "enchant") {
					return intentBuff
				}
			}
		}
	}
	return intentUnknown
}

func bestOwnCreature(snap gamestate.Snapshot, ids []int) int {
	bestID, bestPow, bestAtk := 0, -1, false
	for _, id := range ids {
		o := objectByID(snap, id)
		if o == nil {
			continue
		}
		attacking := strings.Contains(strings.ToLower(o.AttackState), "attack")
		pow := o.Power
		if bestID == 0 || attacking && !bestAtk || (attacking == bestAtk && pow > bestPow) {
			bestID, bestPow, bestAtk = id, pow, attacking
		}
	}
	return bestID
}

func bestOppCreature(snap gamestate.Snapshot, ids []int) int {
	// 优先最高韧性（农场向：清最耐打的），力量作并列
	bestID, bestT, bestP := 0, -1, -1
	for _, id := range ids {
		o := objectByID(snap, id)
		if o == nil {
			if bestID == 0 {
				bestID = id
			}
			continue
		}
		t, p := o.Toughness, o.Power
		if bestID == 0 || t > bestT || (t == bestT && p > bestP) {
			bestID, bestT, bestP = id, t, p
		}
	}
	return bestID
}

func objectByID(snap gamestate.Snapshot, id int) *gamestate.Object {
	for i := range snap.Objects {
		if snap.Objects[i].InstanceID == id {
			return &snap.Objects[i]
		}
	}
	return nil
}
