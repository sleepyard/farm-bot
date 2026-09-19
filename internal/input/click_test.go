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

func TestKbInputMatchesMouseInputSize(t *testing.T) {
	if unsafe.Sizeof(kbInput{}) != unsafe.Sizeof(mouseInput{}) {
		t.Fatalf("kbInput %d != mouseInput %d", unsafe.Sizeof(kbInput{}), unsafe.Sizeof(mouseInput{}))
	}
	if unsafe.Sizeof(kbInput{}) != 40 {
		t.Fatalf("INPUT size want 40, got %d", unsafe.Sizeof(kbInput{}))
	}
}
