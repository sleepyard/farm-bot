package decisioner

import (
	"fmt"
	"strings"

	"github.com/flourbrain/mtga-farm-bot/internal/carddb"
	"github.com/flourbrain/mtga-farm-bot/internal/gamestate"
)

type castCand struct {
	a    gamestate.Action
	paid int
	prio int
}

// tryCast：只施放当前法力付得起的咒语。
// 新手：先花更多法力；同费时结界（含结界生物）> 生物 > 神器 > 瞬间 > 法术。
// 史迹：结界 > 生物 > 其余（其余里再尽量把法力花完）。
func (e *Engine) tryCast(snap gamestate.Snapshot) *Move {
	phase := snap.Turn.Phase
	if phase != "" && phase != gamestate.PhaseMain1 && phase != gamestate.PhaseMain2 {
		return nil
	}

	pool := AvailableMana(snap.Actions)
	var cands []castCand
	for _, a := range snap.Actions {
		if a.ActionType != gamestate.ActionCast || a.InstanceID == 0 {
			continue
		}
		cost := a.ManaCost
		if len(cost) == 0 {
			cmc := carddb.CardCMC(a.GrpID)
			if cmc >= 99 {
				continue
			}
			if cmc > 0 {
				cost = []gamestate.ManaPip{{Colors: []string{"ManaColor_Generic"}, Count: cmc}}
			}
		}
		if !CanAfford(cost, pool) {
			continue
		}
		paid := ManaCostTotal(cost)
		cands = append(cands, castCand{
			a:    a,
			paid: paid,
			prio: typePriority(a.GrpID, objectTypes(snap, a.InstanceID)),
		})
	}
	if len(cands) == 0 {
		return nil
	}

	better := betterCast
	if e.isHistoric() {
		better = betterHistoricCast
	}
	best := cands[0]
	for _, c := range cands[1:] {
		if better(c, best) {
			best = c
		}
	}
	prefix := "施放"
	if e.isHistoric() {
		prefix = "史迹施放"
	}
	return &Move{
		Kind:       KindCast,
		InstanceID: best.a.InstanceID,
		GrpID:      best.a.GrpID,
		CardName:   carddb.NameByGrp(best.a.GrpID),
		Reason:     fmt.Sprintf("%s%s（费用=%d 法力=%d）", prefix, prioLabel(best.prio), best.paid, pool.Total),
	}
}

// betterCast 新手：先花更多法力，同费再比牌类。
func betterCast(a, b castCand) bool {
	if a.paid != b.paid {
		return a.paid > b.paid
	}
	return a.prio > b.prio
}

// historicCastBucket 史迹出牌档：结界 3 > 生物 2 > 其他 1。
func historicCastBucket(prio int) int {
	switch {
	case prio >= 6:
		return 3
	case prio == 5:
		return 2
	default:
		return 1
	}
}

// betterHistoricCast 史迹：结界优先，再生物，同档再尽量用完法力。
func betterHistoricCast(a, b castCand) bool {
	ba, bb := historicCastBucket(a.prio), historicCastBucket(b.prio)
	if ba != bb {
		return ba > bb
	}
	if a.paid != b.paid {
		return a.paid > b.paid
	}
	return a.prio > b.prio
}

func objectTypes(snap gamestate.Snapshot, instanceID int) []string {
	for _, o := range snap.Objects {
		if o.InstanceID == instanceID {
			return o.CardTypes
		}
	}
	return nil
}

func typePriority(grpID int, objTypes []string) int {
	types := append([]string(nil), objTypes...)
	if c, ok := carddb.Global().Get(grpID); ok {
		types = append(types, c.Types...)
	}
	// 结界（含结界生物）最高；GRE 为 CardType_Enchantment。
	if typeHas(types, "enchantment") {
		return 6
	}
	if typeHas(types, "creature") {
		return 5
	}
	if typeHas(types, "artifact") {
		return 4
	}
	if typeHas(types, "instant") {
		return 3
	}
	if typeHas(types, "sorcery") {
		return 2
	}
	return 1
}

func prioLabel(prio int) string {
	switch prio {
	case 6:
		return "结界"
	case 5:
		return "生物"
	case 4:
		return "神器"
	case 3:
		return "瞬间"
	case 2:
		return "法术"
	default:
		return "咒语"
	}
}

func typeHas(types []string, needle string) bool {
	needle = strings.ToLower(needle)
	for _, t := range types {
		if strings.Contains(strings.ToLower(t), needle) {
			return true
		}
	}
	return false
}
