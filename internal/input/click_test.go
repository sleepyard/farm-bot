package input

import (
	"testing"
	"unsafe"
)

func TestNormaliseMatchesPython(t *testing.T) {
	// 与 Python tests/test_input_motion.py 同组用例
	nx, ny := normalise(0, 0, 0, 0, 3440, 1440)
	if nx != 0 || ny != 0 {
		t.Fatalf("origin: got %d,%d", nx, ny)
	}
	nx, ny = normalise(3439, 1439, 0, 0, 3440, 1440)
	if nx != 65535 || ny != 65535 {
		t.Fatalf("last pixel: got %d,%d", nx, ny)
	}
	nx, ny = normalise(-1920, 0, -1920, 0, 5360, 1440)
	if nx != 0 || ny != 0 {
		t.Fatalf("neg origin: got %d,%d", nx, ny)
	}
	nx, _ = normalise(0, 0, -1920, 0, 5360, 1440)
	if nx <= 0 {
		t.Fatalf("primary must not map to edge, got %d", nx)
	}
	// 多显示器上用户实测点
	nx, ny = normalise(3507, 2000, 0, 0, 3840, 2160)
	if nx < 0 || nx > 65535 || ny < 0 || ny > 65535 {
		t.Fatalf("out of range: %d,%d", nx, ny)
	}
}

func TestRefToActual(t *testing.T) {
	cases := []struct {
		name             string
		x, y             int
		clientW, clientH int
		wantX, wantY     int
	}{
		{"identity center", 640, 360, 1280, 720, 640, 360},
		{"identity corner", 1279, 719, 1280, 720, 1279, 719},
		{"origin", 0, 0, 1920, 1080, 0, 0},
		{"upscale 1.5x", 640, 360, 1920, 1080, 960, 540},
		{"upscale 1.5x odd", 100, 200, 1920, 1080, 150, 300},
		{"1366x768 rounding", 639, 719, 1366, 768, 682, 767},
		{"1366x768 center", 640, 360, 1366, 768, 683, 384},
	}
	for _, c := range cases {
		gotX, gotY := refToActual(c.x, c.y, c.clientW, c.clientH)
		if gotX != c.wantX || gotY != c.wantY {
			t.Errorf("%s: refToActual(%d,%d,%d,%d) = (%d,%d), want (%d,%d)",
				c.name, c.x, c.y, c.clientW, c.clientH, gotX, gotY, c.wantX, c.wantY)
		}
	}
}

func TestKbInputMatchesMouseInputSize(t *testing.T) {
	if unsafe.Sizeof(kbInput{}) != unsafe.Sizeof(mouseInput{}) {
		t.Fatalf("kbInput %d != mouseInput %d", unsafe.Sizeof(kbInput{}), unsafe.Sizeof(mouseInput{}))
	}
	if unsafe.Sizeof(kbInput{}) != 40 {
		t.Fatalf("INPUT size want 40, got %d", unsafe.Sizeof(kbInput{}))
	}
}
