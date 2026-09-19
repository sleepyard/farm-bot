package decisioner

import (
	"testing"

	"github.com/flourbrain/mtga-farm-bot/internal/gamestate"
)

func TestCanAffordColored(t *testing.T) {
	pool := ManaPool{
		Colors:  map[string]struct{}{"white": {}, "blue": {}},
		Total:   2,
		Sources: []map[string]struct{}{{"white": {}}, {"blue": {}}},
	}
	cost := []gamestate.ManaPip{
		{Colors: []string{"ManaColor_White"}, Count: 1},
		{Colors: []string{"ManaColor_Generic"}, Count: 1},
	}
	if !CanAfford(cost, pool) {
		t.Fatal("expected affordable W+1")
	}
	cost2 := []gamestate.ManaPip{
		{Colors: []string{"ManaColor_Red"}, Count: 1},
	}
	if CanAfford(cost2, pool) {
		t.Fatal("red should not be affordable")
	}
	if CanAfford([]gamestate.ManaPip{{Colors: []string{"ManaColor_Generic"}, Count: 3}}, pool) {
		t.Fatal("3 generic > 2 sources")
	}
}

func TestTryCastSkipsUnaffordable(t *testing.T) {
	e := New()
	snap := gamestate.Snapshot{
		HasMulledKeep: true,
		SystemSeatID:  1,
		Turn:          gamestate.TurnInfo{TurnNumber: 1, Phase: gamestate.PhaseMain1, DecisionPlayer: 1},
		Actions: []gamestate.Action{
			{ActionType: gamestate.ActionActivateMana, InstanceID: 10, GrpID: 1, AbilityGrpID: 1001},
			{ActionType: gamestate.ActionCast, InstanceID: 20, GrpID: 200, ManaCost: []gamestate.ManaPip{
				{Colors: []string{"ManaColor_Generic"}, Count: 3},
				{Colors: []string{"ManaColor_White"}, Count: 1},
			}},
			{ActionType: gamestate.ActionCast, InstanceID: 21, GrpID: 201, ManaCost: []gamestate.ManaPip{
				{Colors: []string{"ManaColor_White"}, Count: 1},
			}},
		},
	}
	m := e.tryCast(snap)
	if m == nil || m.InstanceID != 21 {
		t.Fatalf("want cast 21 (1W), got %#v", m)
	}
}

func TestTryPlayLandWhenNoMana(t *testing.T) {
	e := New()
	snap := gamestate.Snapshot{
		HasMulledKeep: true,
		SystemSeatID:  1,
		Turn:          gamestate.TurnInfo{TurnNumber: 1, Phase: gamestate.PhaseMain1, DecisionPlayer: 1},
		Actions: []gamestate.Action{
			{ActionType: gamestate.ActionPlay, InstanceID: 100, GrpID: 91301},
			{ActionType: gamestate.ActionCast, InstanceID: 200, GrpID: 999, ManaCost: []gamestate.ManaPip{
				{Colors: []string{"ManaColor_Generic"}, Count: 4},
			}},
		},
	}
	m := e.tryPlayLand(snap)
	if m == nil || m.InstanceID != 100 {
		t.Fatalf("want play land 100, got %#v", m)
	}
	m2 := e.tryCast(snap)
	if m2 != nil {
		t.Fatalf("should not cast unaffordable with 0 mana, got %#v", m2)
	}
}

func TestSelectNPreferred(t *testing.T) {
	e := New()
	snap := gamestate.Snapshot{
		HasMulledKeep: true,
		SystemSeatID:  2,
		SelectN: &gamestate.SelectNPrompt{
			IDs:    []int{219, 222},
			MinSel: 1,
			MaxSel: 1,
		},
		Turn: gamestate.TurnInfo{TurnNumber: 9, Phase: gamestate.PhaseMain1, DecisionPlayer: 2},
		Objects: []gamestate.Object{
			{InstanceID: 219, GrpID: 100},
			{InstanceID: 222, GrpID: 200},
		},
	}
	m := e.NextMove(snap)
	if m.Kind != KindSelectN {
		t.Fatalf("want select_n, got %#v", m)
	}
	if m.InstanceID != 219 && m.InstanceID != 222 {
		t.Fatalf("bad id %#v", m)
	}
}

func TestSelectTargetOwnBuff(t *testing.T) {
	e := New()
	snap := gamestate.Snapshot{
		HasMulledKeep:     true,
		SystemSeatID:      1,
		NeedsTargetSelect: true,
		SelectTarget: &gamestate.SelectTargetPrompt{
			SourceID:     50,
			OwnCreatures: []int{10, 11},
			OppCreatures: []int{20},
			FaceLegal:    false,
		},
		Turn: gamestate.TurnInfo{TurnNumber: 3, Phase: gamestate.PhaseMain1, DecisionPlayer: 1},
		Objects: []gamestate.Object{
			{InstanceID: 50, GrpID: 1, ControllerSeatID: 1},
			{InstanceID: 10, GrpID: 2, ControllerSeatID: 1, CardTypes: []string{"CardType_Creature"}, Power: 2, Toughness: 2},
			{InstanceID: 11, GrpID: 3, ControllerSeatID: 1, CardTypes: []string{"CardType_Creature"}, Power: 5, Toughness: 3, AttackState: "AttackState_Attacking"},
			{InstanceID: 20, GrpID: 4, ControllerSeatID: 2, CardTypes: []string{"CardType_Creature"}, Power: 4, Toughness: 4},
		},
	}
	// 强制有益：合法目标含双方时靠牌名；这里把 source 伪装为 Bulk Up 风格仅用 onlyOwn 路径
	snap.SelectTarget.OppCreatures = nil
	m := e.NextMove(snap)
	if m.Kind != KindSelectTarget || !m.FriendlyTarget || m.Targets[0] != 11 {
		t.Fatalf("want own attacker 11, got %#v", m)
	}
}

func TestSelectTargetOppHarm(t *testing.T) {
	e := New()
	snap := gamestate.Snapshot{
		HasMulledKeep:     true,
		SystemSeatID:      1,
		NeedsTargetSelect: true,
		SelectTarget: &gamestate.SelectTargetPrompt{
			SourceID:     50,
			OwnCreatures: []int{10},
			OppCreatures: []int{20, 21},
			FaceLegal:    true,
		},
		Turn: gamestate.TurnInfo{TurnNumber: 3, Phase: gamestate.PhaseMain1, DecisionPlayer: 1},
		Objects: []gamestate.Object{
			{InstanceID: 50, GrpID: 1, ControllerSeatID: 1},
			{InstanceID: 10, GrpID: 2, ControllerSeatID: 1, CardTypes: []string{"CardType_Creature"}, Power: 5, Toughness: 5},
			{InstanceID: 20, GrpID: 4, ControllerSeatID: 2, CardTypes: []string{"CardType_Creature"}, Power: 2, Toughness: 2},
			{InstanceID: 21, GrpID: 5, ControllerSeatID: 2, CardTypes: []string{"CardType_Creature"}, Power: 3, Toughness: 6},
		},
	}
	// 无牌名时：有对方生物 → 选最高韧性对方
	m := e.NextMove(snap)
	if m.Kind != KindSelectTarget || m.FriendlyTarget || m.Targets[0] != 21 {
		t.Fatalf("want opp 21 (tough 6), got %#v", m)
	}
}

func TestSelectTargetFace(t *testing.T) {
	e := New()
	snap := gamestate.Snapshot{
		HasMulledKeep:     true,
		SystemSeatID:      1,
		NeedsTargetSelect: true,
		SelectTarget: &gamestate.SelectTargetPrompt{
			SourceID:  50,
			FaceLegal: true,
		},
		Turn: gamestate.TurnInfo{TurnNumber: 3, Phase: gamestate.PhaseMain1, DecisionPlayer: 1},
	}
	m := e.NextMove(snap)
	if m.Kind != KindSelectTarget || len(m.Targets) == 0 || m.Targets[0] != -1 {
		t.Fatalf("want face -1, got %#v", m)
	}
}

func TestHistoricAlwaysFaceTarget(t *testing.T) {
	e := NewWithMode("historic")
	snap := gamestate.Snapshot{
		HasMulledKeep:     true,
		SystemSeatID:      1,
		NeedsTargetSelect: true,
		SelectTarget: &gamestate.SelectTargetPrompt{
			SourceID:     50,
			OwnCreatures: []int{10},
			OppCreatures: []int{20},
			FaceLegal:    false,
		},
		Turn: gamestate.TurnInfo{TurnNumber: 3, Phase: gamestate.PhaseMain1, DecisionPlayer: 1},
		Objects: []gamestate.Object{
			{InstanceID: 10, ControllerSeatID: 1, CardTypes: []string{"CardType_Creature"}, Power: 5},
			{InstanceID: 20, ControllerSeatID: 2, CardTypes: []string{"CardType_Creature"}, Toughness: 6},
		},
	}
	m := e.NextMove(snap)
	if m.Kind != KindSelectTarget || len(m.Targets) == 0 || m.Targets[0] != -1 {
		t.Fatalf("historic want face -1, got %#v", m)
	}
}

func TestZombifyChooserEvenInHistoric(t *testing.T) {
	e := NewWithMode("historic")
	snap := gamestate.Snapshot{
		HasMulledKeep:     true,
		SystemSeatID:      1,
		NeedsTargetSelect: true,
		SelectTarget: &gamestate.SelectTargetPrompt{
			SourceID:   99,
			ChooserIDs: []int{11, 12},
		},
		Turn: gamestate.TurnInfo{TurnNumber: 3, Phase: gamestate.PhaseMain1, DecisionPlayer: 1},
		Objects: []gamestate.Object{
			{InstanceID: 11, GrpID: 100, CardTypes: []string{"CardType_Creature"}, Power: 2, Toughness: 2},
			{InstanceID: 12, GrpID: 200, CardTypes: []string{"CardType_Creature"}, Power: 5, Toughness: 5},
		},
	}
	m := e.NextMove(snap)
	if m.Kind != KindSelectTarget || !m.Chooser || m.Targets[0] != 12 {
		t.Fatalf("want chooser 12 (bigger body when CMC unknown), got %#v", m)
	}
}

func TestHistoricNoBlocks(t *testing.T) {
	e := NewWithMode("historic")
	snap := gamestate.Snapshot{
		HasMulledKeep: true,
		SystemSeatID:  1,
		Turn: gamestate.TurnInfo{
			TurnNumber: 2, Phase: gamestate.PhaseCombat,
			Step: gamestate.StepDeclareBlock, DecisionPlayer: 1,
		},
	}
	m := e.NextMove(snap)
	if m.Kind != KindNoBlocks {
		t.Fatalf("want no_blocks, got %#v", m)
	}
}

func TestClassifyTargetIntent(t *testing.T) {
	if classifyTargetIntent(0, "Lightning Bolt") != intentHarm {
		t.Fatal("bolt should be harm")
	}
	if classifyTargetIntent(0, "Bulk Up") != intentBuff {
		t.Fatal("bulk up should be buff")
	}
	if classifyTargetIntent(0, "Fake Your Own Death") != intentBuff {
		t.Fatal("fake death should be buff")
	}
}
