package decisioner

import (
	"testing"

	"github.com/flourbrain/mtga-farm-bot/internal/gamestate"
)

func TestBetterCastPrefersEnchantmentAtSameMana(t *testing.T) {
	enc := castCand{paid: 2, prio: 6}
	cr := castCand{paid: 2, prio: 5}
	inst := castCand{paid: 2, prio: 3}
	sorc := castCand{paid: 2, prio: 2}
	if !betterCast(enc, cr) || !betterCast(enc, inst) || !betterCast(enc, sorc) {
		t.Fatal("same mana: enchantment should beat creature/instant/sorcery")
	}
}

func TestBetterCastManaBeatsType(t *testing.T) {
	enc := castCand{paid: 1, prio: 6}
	cr := castCand{paid: 3, prio: 5}
	if !betterCast(cr, enc) {
		t.Fatal("higher mana creature should beat cheaper enchantment")
	}
	if betterCast(enc, cr) {
		t.Fatal("cheaper enchantment should not beat higher mana")
	}
}

func TestBetterHistoricCastTypeBeatsMana(t *testing.T) {
	enc := castCand{paid: 1, prio: 6}
	cr := castCand{paid: 4, prio: 5}
	inst := castCand{paid: 5, prio: 3}
	if !betterHistoricCast(enc, cr) {
		t.Fatal("historic: cheap enchantment should beat expensive creature")
	}
	if !betterHistoricCast(cr, inst) {
		t.Fatal("historic: creature should beat higher-mana instant")
	}
	if betterHistoricCast(inst, enc) {
		t.Fatal("historic: instant should not beat enchantment")
	}
}

func TestBetterHistoricCastDumpsManaWithinBucket(t *testing.T) {
	cheapEnc := castCand{paid: 1, prio: 6}
	dearEnc := castCand{paid: 3, prio: 6}
	if !betterHistoricCast(dearEnc, cheapEnc) {
		t.Fatal("historic: among enchantments dump more mana")
	}
	inst := castCand{paid: 2, prio: 3}
	sorc := castCand{paid: 4, prio: 2}
	art := castCand{paid: 1, prio: 4}
	if !betterHistoricCast(sorc, inst) || !betterHistoricCast(inst, art) {
		t.Fatal("historic: among other, dump more mana")
	}
}

func TestTryCastHistoricPrefersEnchantment(t *testing.T) {
	e := NewWithMode("historic")
	snap := gamestate.Snapshot{
		HasMulledKeep: true,
		SystemSeatID:  1,
		Turn:          gamestate.TurnInfo{TurnNumber: 1, Phase: gamestate.PhaseMain1, DecisionPlayer: 1},
		Objects: []gamestate.Object{
			{InstanceID: 20, CardTypes: []string{"CardType_Enchantment"}},
			{InstanceID: 21, CardTypes: []string{"CardType_Creature"}},
			{InstanceID: 22, CardTypes: []string{"CardType_Instant"}},
		},
		Actions: []gamestate.Action{
			{ActionType: gamestate.ActionActivateMana, InstanceID: 1, GrpID: 1, AbilityGrpID: 1001},
			{ActionType: gamestate.ActionActivateMana, InstanceID: 2, GrpID: 1, AbilityGrpID: 1001},
			{ActionType: gamestate.ActionActivateMana, InstanceID: 3, GrpID: 1, AbilityGrpID: 1001},
			{ActionType: gamestate.ActionCast, InstanceID: 20, GrpID: 200, ManaCost: []gamestate.ManaPip{
				{Colors: []string{"ManaColor_Generic"}, Count: 1},
			}},
			{ActionType: gamestate.ActionCast, InstanceID: 21, GrpID: 201, ManaCost: []gamestate.ManaPip{
				{Colors: []string{"ManaColor_Generic"}, Count: 3},
			}},
			{ActionType: gamestate.ActionCast, InstanceID: 22, GrpID: 202, ManaCost: []gamestate.ManaPip{
				{Colors: []string{"ManaColor_Generic"}, Count: 2},
			}},
		},
	}
	m := e.tryCast(snap)
	if m == nil || m.InstanceID != 20 {
		t.Fatalf("historic want cheap enchantment 20, got %#v", m)
	}
}

func TestTypeHasEnchantmentCreature(t *testing.T) {
	if typePriority(0, []string{"Enchantment", "Creature"}) != 6 {
		t.Fatal("carddb-style enchantment creature")
	}
	if typePriority(0, []string{"CardType_Enchantment", "CardType_Creature"}) != 6 {
		t.Fatal("GRE enchantment creature")
	}
	if typePriority(0, []string{"CardType_Creature"}) != 5 {
		t.Fatal("plain creature")
	}
}
