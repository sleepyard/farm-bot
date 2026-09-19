package decisioner

import (
	"fmt"

	"github.com/flourbrain/mtga-farm-bot/internal/carddb"
	"github.com/flourbrain/mtga-farm-bot/internal/gamestate"
)

// tryPlayLand：选能让手上生物在「打出后」变得可施放的地（对齐 Python _choose_land_to_play）。
func (e *Engine) tryPlayLand(snap gamestate.Snapshot) *Move {
	if e.landPlayedThisTurn {
		return nil
	}
	phase := snap.Turn.Phase
	if phase != "" && phase != gamestate.PhaseMain1 && phase != gamestate.PhaseMain2 {
		return nil
	}

	var lands []gamestate.Action
	for _, a := range snap.Actions {
		if a.ActionType == gamestate.ActionPlay && a.InstanceID != 0 {
			lands = append(lands, a)
		}
	}
	if len(lands) == 0 {
		return nil
	}

	pool := AvailableMana(snap.Actions)
	type creatureCast struct {
		cost []gamestate.ManaPip
		cmc  int
	}
	var creatures []creatureCast
	for _, a := range snap.Actions {
		if a.ActionType != gamestate.ActionCast || a.InstanceID == 0 {
			continue
		}
		if !carddb.IsCreature(a.GrpID) {
			continue
		}
		creatures = append(creatures, creatureCast{
			cost: a.ManaCost,
			cmc:  carddb.CardCMC(a.GrpID),
		})
	}

	bestIdx := 0
	bestScore := [4]int{-1, -1, -1, -1}
	for i, land := range lands {
		produced := carddb.LandProducedColors(land.GrpID)
		sim := withSimLand(pool, produced)
		castable := 0
		bestCMC := 999
		for _, cr := range creatures {
			if CanAfford(cr.cost, sim) {
				castable++
				if cr.cmc < bestCMC {
					bestCMC = cr.cmc
				}
			}
		}
		newColors := 0
		for c := range produced {
			if _, ok := pool.Colors[c]; !ok {
				newColors++
			}
		}
		enable := 0
		if castable > 0 {
			enable = 1
		}
		score := [4]int{enable, castable, -bestCMC, newColors}
		if scoreGreater(score, bestScore) {
			bestScore = score
			bestIdx = i
		}
	}

	land := lands[bestIdx]
	return &Move{
		Kind:       KindPlayLand,
		InstanceID: land.InstanceID,
		GrpID:      land.GrpID,
		CardName:   carddb.NameByGrp(land.GrpID),
		Reason:     fmt.Sprintf("打出地牌（启用生物=%d）", bestScore[1]),
	}
}

func scoreGreater(a, b [4]int) bool {
	for i := 0; i < 4; i++ {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

func (e *Engine) MarkLandPlayed() {
	e.landPlayedThisTurn = true
}
