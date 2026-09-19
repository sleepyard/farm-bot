package controller

import (
	"testing"
	"time"

	"github.com/flourbrain/mtga-farm-bot/internal/gamestate"
	"github.com/flourbrain/mtga-farm-bot/internal/vision"
)

func TestHandSweepPacingMatchesPython(t *testing.T) {
	if len(handSweepPacing) != 3 {
		t.Fatalf("pacing len=%d want 3", len(handSweepPacing))
	}
	want := []sweepPace{
		{10, 10 * time.Millisecond},
		{8, 18 * time.Millisecond},
		{7, 24 * time.Millisecond},
	}
	for i, p := range handSweepPacing {
		if p.step != want[i].step || p.dwell != want[i].dwell {
			t.Fatalf("attempt %d = %dpx/%v want %dpx/%v", i, p.step, p.dwell, want[i].step, want[i].dwell)
		}
	}
}

func TestBattlefieldScanMatchesPython(t *testing.T) {
	if bfScanStep != 55 {
		t.Fatalf("step=%d want 55", bfScanStep)
	}
	if bfDwell != 50*time.Millisecond {
		t.Fatalf("dwell=%v want 50ms", bfDwell)
	}
	if bfScanMax != 4*time.Second {
		t.Fatalf("timeout=%v want 4s", bfScanMax)
	}
	if bfResetAbove != 80 {
		t.Fatalf("resetAbove=%d want 80", bfResetAbove)
	}
	if bfScanX0 != 128 || bfScanX1 != 1178 || bfOwnY0 != 360 || bfOwnY1 != 648 {
		t.Fatalf("own band (%d,%d)-(%d,%d)", bfScanX0, bfOwnY0, bfScanX1, bfOwnY1)
	}
	if bfOppY0 != 173 || bfOppY1 != 324 {
		t.Fatalf("opp y=%d..%d", bfOppY0, bfOppY1)
	}
}

func TestConfirmTemplatesOrder(t *testing.T) {
	want := []string{"Buttons/next.png", "Buttons/scry_done.png", "Buttons/submit_btn.png"}
	if len(confirmTemplateRels) != len(want) {
		t.Fatalf("rels=%v", confirmTemplateRels)
	}
	for i, rel := range want {
		if confirmTemplateRels[i] != rel {
			t.Fatalf("rel[%d]=%s want %s", i, confirmTemplateRels[i], rel)
		}
	}
}

func TestModalLastPointMatchesPython(t *testing.T) {
	// Python n=2: int(408 + 0.5*107)=461 → FromBase1920(956,461)
	pt := modalLastPoint(2)
	want := vision.FromBase1920(956, 461)
	if pt != want {
		t.Fatalf("n=2 %v want %v", pt, want)
	}
	right, _ := modalChoicePoint(gamestate.ModalChoiceJob{Kind: gamestate.ModalChoiceWardensDraw})
	wantRight := vision.FromBase1920(1172, 480)
	if right != wantRight {
		t.Fatalf("wardens right %v want %v", right, wantRight)
	}
}
