package decisioner

import (
	"fmt"
	"strings"

	"github.com/flourbrain/mtga-farm-bot/internal/carddb"
	"github.com/flourbrain/mtga-farm-bot/internal/gamestate"
)

func pickChooserOrStack(snap gamestate.Snapshot, p *gamestate.SelectTargetPrompt) *Move {
	if p == nil {
		return nil
	}
	srcName := carddb.NameByGrp(grpOf(snap, p.SourceID))
	if len(p.StackIDs) > 0 {
		id := p.StackIDs[0]
		return &Move{
			Kind:       KindSelectTarget,
			InstanceID: id,
			CardName:   srcName,
			Targets:    []int{id},
			Stack:      true,
			Reason:     fmt.Sprintf("堆叠目标 %s", carddb.FormatInstance(id, grpOf(snap, id))),
		}
	}
	if len(p.ChooserIDs) == 0 {
		return nil
	}
	id := bestChooserCreature(snap, p.ChooserIDs)
	if id <= 0 {
		id = p.ChooserIDs[0]
	}
	return &Move{
		Kind:       KindSelectTarget,
		InstanceID: id,
		CardName:   srcName,
		Targets:    []int{id},
		Chooser:    true,
		Reason:     fmt.Sprintf("墓地/放逐选牌 %s", carddb.FormatInstance(id, grpOf(snap, id))),
	}
}

// bestChooserCreature 对齐 Python LifegainLogic.best_creature：先生物，再 CMC，再体型。
func bestChooserCreature(snap gamestate.Snapshot, ids []int) int {
	bestID, bestCMC, bestBody := 0, -1, -1
	sawCreature := false
	for _, id := range ids {
		o := objectByID(snap, id)
		creature := o != nil && objectIsCreature(*o)
		if sawCreature && !creature {
			continue
		}
		if creature && !sawCreature {
			sawCreature = true
			bestID, bestCMC, bestBody = 0, -1, -1
		}
		cmc, body := 0, 0
		if o != nil {
			body = o.Power + o.Toughness
			cmc = carddb.CardCMC(o.GrpID)
			if cmc == 99 {
				cmc = 0
			}
		}
		if bestID == 0 || cmc > bestCMC || (cmc == bestCMC && body > bestBody) {
			bestID, bestCMC, bestBody = id, cmc, body
		}
	}
	return bestID
}

func objectIsCreature(o gamestate.Object) bool {
	for _, t := range o.CardTypes {
		if t == "CardType_Creature" || strings.EqualFold(t, "Creature") {
			return true
		}
	}
	return false
}
