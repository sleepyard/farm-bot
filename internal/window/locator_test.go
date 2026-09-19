package window

import "testing"

func TestIsSupportedLiveSize(t *testing.T) {
	cases := []struct {
		name string
		w, h int
		want bool
	}{
		{"1280x720 参考", 1280, 720, true},
		{"1600x900", 1600, 900, true},
		{"1920x1080", 1920, 1080, true},
		{"1366x768 近似16:9", 1366, 768, true},
		{"1280x1024 非16:9", 1280, 1024, false},
		{"800x600 过小且非16:9", 800, 600, false},
		{"960x540 低于下限", 960, 540, false},
	}
	for _, c := range cases {
		if got := isSupportedLiveSize(c.w, c.h); got != c.want {
			t.Errorf("%s: isSupportedLiveSize(%d,%d) = %v, want %v", c.name, c.w, c.h, got, c.want)
		}
	}
}
