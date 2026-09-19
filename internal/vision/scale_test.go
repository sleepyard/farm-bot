package vision

import "testing"

func TestFromBase1920(t *testing.T) {
	p := FromBase1920(1755, 944)
	if p.X != 1170 || p.Y != 629 {
		t.Fatalf("resolve fallback got (%d,%d)", p.X, p.Y)
	}
	plain := FromBase1920(750, 505)
	if plain.X != 500 || plain.Y != 337 {
		t.Fatalf("kicker plain got (%d,%d) want (500,337)", plain.X, plain.Y)
	}
	modal := FromBase1920(1185, 505)
	if modal.X != 790 || modal.Y != 337 {
		t.Fatalf("modal second got (%d,%d) want (790,337)", modal.X, modal.Y)
	}
	sac := FromBase1920(1775, 978)
	if sac.X != 1183 || sac.Y != 652 {
		t.Fatalf("sacrifice got (%d,%d) want (1183,652)", sac.X, sac.Y)
	}
	roi := FromBase1920ROI(456, 552, 300, 150)
	if roi.X != 304 || roi.Y != 368 {
		t.Fatalf("roi origin got %+v", roi)
	}
}
