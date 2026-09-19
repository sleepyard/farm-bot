package vision

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestFindTemplateExact(t *testing.T) {
	dir := t.TempDir()
	hay := image.NewRGBA(image.Rect(0, 0, 200, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 200; x++ {
			hay.Set(x, y, color.RGBA{R: 20, G: 20, B: 20, A: 255})
		}
	}
	// 非均匀图案，避免模板方差为 0
	for y := 20; y < 50; y++ {
		for x := 40; x < 90; x++ {
			v := uint8(80 + (x+y)%100)
			hay.Set(x, y, color.RGBA{R: v, G: 40, B: 200 - v/2, A: 255})
		}
	}
	tpl := image.NewRGBA(image.Rect(0, 0, 50, 30))
	for y := 0; y < 30; y++ {
		for x := 0; x < 50; x++ {
			v := uint8(80 + (40+x+20+y)%100)
			tpl.Set(x, y, color.RGBA{R: v, G: 40, B: 200 - v/2, A: 255})
		}
	}
	tplPath := filepath.Join(dir, "tpl.png")
	f, err := os.Create(tplPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, tpl); err != nil {
		t.Fatal(err)
	}
	f.Close()

	ClearTemplateCache()
	m, err := FindTemplate(hay, tplPath, ROI{}, 0.85)
	if err != nil {
		t.Fatal(err)
	}
	if m == nil {
		t.Fatal("expected match")
	}
	if abs(m.X-65) > 3 || abs(m.Y-35) > 3 {
		t.Fatalf("center got (%d,%d) want ~ (65,35) score=%v", m.X, m.Y, m.Score)
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
