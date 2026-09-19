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

// writePatternPNG writes a deterministic gradient pattern seeded by seed.
func writePatternPNG(t *testing.T, dir, name string, seed int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 50, 30))
	for y := 0; y < 30; y++ {
		for x := 0; x < 50; x++ {
			v := uint8((seed + x*3 + y*7) % 200)
			img.Set(x, y, color.RGBA{R: v, G: uint8((v + 40) % 255), B: 255 - v, A: 255})
		}
	}
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return path
}

func TestFindTemplateAnyBestPicksCNVariant(t *testing.T) {
	dir := t.TempDir()
	enPath := writePatternPNG(t, dir, "keep_hand.png", 11)
	cnPath := writePatternPNG(t, dir, "keep_hand.cn.png", 97)

	// Haystack only embeds the CN pattern.
	hay := image.NewRGBA(image.Rect(0, 0, 200, 120))
	for y := 0; y < 120; y++ {
		for x := 0; x < 200; x++ {
			hay.Set(x, y, color.RGBA{R: 30, G: 30, B: 30, A: 255})
		}
	}
	cnTpl, err := png.Decode(mustOpen(t, cnPath))
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 30; y++ {
		for x := 0; x < 50; x++ {
			hay.Set(60+x, 40+y, cnTpl.At(x, y))
		}
	}

	ClearTemplateCache()
	m, hitPath, err := FindTemplateAnyBest(hay, []string{enPath, cnPath}, ROI{}, 0.85)
	if err != nil {
		t.Fatal(err)
	}
	if m == nil {
		t.Fatal("expected match")
	}
	if hitPath != cnPath {
		t.Fatalf("hit path got %q want %q", hitPath, cnPath)
	}
	if abs(m.X-85) > 3 || abs(m.Y-55) > 3 {
		t.Fatalf("center got (%d,%d) want ~ (85,55) score=%v", m.X, m.Y, m.Score)
	}
}

func TestFindTemplateAnyBestNoFalseHit(t *testing.T) {
	dir := t.TempDir()
	enPath := writePatternPNG(t, dir, "keep_hand.png", 11)
	cnPath := writePatternPNG(t, dir, "keep_hand.cn.png", 97)

	// Uniform background matches neither candidate above threshold.
	hay := image.NewRGBA(image.Rect(0, 0, 200, 120))
	for y := 0; y < 120; y++ {
		for x := 0; x < 200; x++ {
			hay.Set(x, y, color.RGBA{R: 30, G: 30, B: 30, A: 255})
		}
	}

	ClearTemplateCache()
	m, hitPath, err := FindTemplateAnyBest(hay, []string{enPath, cnPath}, ROI{}, 0.90)
	if err != nil {
		t.Fatal(err)
	}
	if m != nil {
		t.Fatalf("expected no hit, got score=%.3f path=%s", m.Score, hitPath)
	}
	if hitPath != "" {
		t.Fatalf("expected empty hit path, got %q", hitPath)
	}
}

func mustOpen(t *testing.T, path string) *os.File {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}
