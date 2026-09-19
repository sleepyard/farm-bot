package decisioner

import (
	"testing"

	"github.com/flourbrain/mtga-farm-bot/internal/gamestate"
)

func TestDeclareAttackFallsBackToNoAttacksAfterThreeTries(t *testing.T) {
	e := New()
	snap := gamestate.Snapshot{
		HasMulledKeep: true,
		SystemSeatID:  1,
		Turn: gamestate.TurnInfo{
			TurnNumber: 2, Phase: gamestate.PhaseCombat,
			Step: gamestate.StepDeclareAttack, DecisionPlayer: 1,
		},
	}
	for i := 1; i <= 3; i++ {
		m := e.NextMove(snap)
		if m.Kind != KindAllAttack {
			t.Fatalf("try %d: want all_attack, got %#v", i, m)
		}
	}
	m := e.NextMove(snap)
	if m.Kind != KindNoAttacks {
		t.Fatalf("after 3 all_attack want no_attacks, got %#v", m)
	}
	m2 := e.NextMove(snap)
	if m2.Kind != KindNoAttacks {
		t.Fatalf("still stuck: keep no_attacks, got %#v", m2)
	}
}

func TestAttackAllTriesResetWhenLeavingDeclareAttack(t *testing.T) {
	e := New()
	atk := gamestate.Snapshot{
		HasMulledKeep: true,
		SystemSeatID:  1,
		Turn: gamestate.TurnInfo{
			TurnNumber: 2, Phase: gamestate.PhaseCombat,
			Step: gamestate.StepDeclareAttack, DecisionPlayer: 1,
		},
	}
	main := atk
	main.Turn.Phase = gamestate.PhaseMain2
	main.Turn.Step = ""
	_ = e.NextMove(atk)
	_ = e.NextMove(atk)
	_ = e.NextMove(main)
	m := e.NextMove(atk)
	if m.Kind != KindAllAttack {
		t.Fatalf("new combat should all_attack again, got %#v", m)
	}
}
