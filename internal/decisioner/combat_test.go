package decisioner

import (
	"strings"
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

func TestDeclareAttackPlaneswalkerAssignsTargets(t *testing.T) {
	e := New()
	snap := gamestate.Snapshot{
		HasMulledKeep:        true,
		SystemSeatID:         1,
		AttackTargetRequired: true,
		AttackerIDs:          []int{11, 12},
		Turn: gamestate.TurnInfo{
			TurnNumber: 2, Phase: gamestate.PhaseCombat,
			Step: gamestate.StepDeclareAttack, DecisionPlayer: 1,
		},
	}
	m := e.NextMove(snap)
	if m.Kind != KindAssignAttackTargets {
		t.Fatalf("want assign_attack_targets, got %#v", m)
	}
}

func declareAttackSnap(actions []gamestate.Action) gamestate.Snapshot {
	return gamestate.Snapshot{
		HasMulledKeep: true,
		SystemSeatID:  1,
		Turn: gamestate.TurnInfo{
			TurnNumber: 2, Phase: gamestate.PhaseCombat,
			Step: gamestate.StepDeclareAttack, DecisionPlayer: 1, ActivePlayer: 1,
		},
		Actions: actions,
	}
}

func TestDeclareAttackCastsAffordableFirst(t *testing.T) {
	e := New()
	snap := declareAttackSnap([]gamestate.Action{
		{ActionType: gamestate.ActionActivateMana, InstanceID: 1, GrpID: 1, AbilityGrpID: 1001},
		{ActionType: gamestate.ActionCast, InstanceID: 20, GrpID: 200, ManaCost: []gamestate.ManaPip{
			{Colors: []string{"ManaColor_Generic"}, Count: 1},
		}},
	})
	m := e.NextMove(snap)
	if m.Kind != KindCast || m.InstanceID != 20 {
		t.Fatalf("affordable spell before attack, got %#v", m)
	}
}

func TestDeclareAttackWhenCardsButNotEnoughMana(t *testing.T) {
	e := New()
	snap := declareAttackSnap([]gamestate.Action{
		{ActionType: gamestate.ActionActivateMana, InstanceID: 1, GrpID: 1, AbilityGrpID: 1001},
		{ActionType: gamestate.ActionCast, InstanceID: 20, GrpID: 200, ManaCost: []gamestate.ManaPip{
			{Colors: []string{"ManaColor_Generic"}, Count: 3},
		}},
	})
	m := e.NextMove(snap)
	if m.Kind != KindAllAttack {
		t.Fatalf("unaffordable spell should still attack, got %#v", m)
	}
	if m.Reason == "" || !strings.Contains(m.Reason, "法力不足") {
		t.Fatalf("reason=%q", m.Reason)
	}
}

func TestDeclareAttackWhenManaButNoCasts(t *testing.T) {
	e := New()
	snap := declareAttackSnap([]gamestate.Action{
		{ActionType: gamestate.ActionActivateMana, InstanceID: 1, GrpID: 1, AbilityGrpID: 1001},
	})
	m := e.NextMove(snap)
	if m.Kind != KindAllAttack {
		t.Fatalf("leftover mana no spells should attack, got %#v", m)
	}
	if m.Reason == "" || !strings.Contains(m.Reason, "无可出的牌") {
		t.Fatalf("reason=%q", m.Reason)
	}
}
