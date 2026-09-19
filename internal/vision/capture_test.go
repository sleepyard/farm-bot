package vision

import (
	"image"
	"math/rand"
	"testing"
)

// 合成 1920×1080 图（1280 参考图上行 ×1.5），经 resizeRGBA 归一化回 1280×720 后，
// 1280 模板匹配应仍命中原中心（±2px）且 score ≥ 0.9。
func TestResizeRGBANormalizeKeepsTemplateMatch(t *testing.T) {
	ref := image.NewRGBA(image.Rect(0, 0, LiveWidth, LiveHeight))
	// 背景给轻微亮度梯度，避免全黑 / 零方差
	for y := 0; y < LiveHeight; y++ {
		for x := 0; x < LiveWidth; x++ {
			i := ref.PixOffset(x, y)
			v := uint8(20 + (x+y)%16)
			ref.Pix[i], ref.Pix[i+1], ref.Pix[i+2], ref.Pix[i+3] = v, v, v, 255
		}
	}
	// 已知位置图案：4×4 像素块级随机灰度（低频结构经双线性往返仍稳定）
	rng := rand.New(rand.NewSource(7))
	tw, th := 48, 40
	tx, ty := 396, 250 // 左上角，中心 (420,270)
	for cy := 0; cy < th; cy += 4 {
		for cx := 0; cx < tw; cx += 4 {
			v := uint8(40 + rng.Intn(200))
			for y := cy; y < cy+4; y++ {
				for x := cx; x < cx+4; x++ {
					i := ref.PixOffset(tx+x, ty+y)
					ref.Pix[i], ref.Pix[i+1], ref.Pix[i+2], ref.Pix[i+3] = v, v, v, 255
				}
			}
		}
	}
	tpl := toGray(ref.SubImage(image.Rect(tx, ty, tx+tw, ty+th)).(*image.RGBA))

	// 模拟实际客户区 1920×1080：先上行，再走 CaptureClient 的归一化路径
	actual := resizeRGBA(ref, 1920, 1080)
	normalized := resizeRGBA(actual, LiveWidth, LiveHeight)
	if normalized.Bounds().Dx() != LiveWidth || normalized.Bounds().Dy() != LiveHeight {
		t.Fatalf("归一化尺寸 %v，期望 %dx%d", normalized.Bounds(), LiveWidth, LiveHeight)
	}

	m, err := matchGray(normalized, tpl, FullROI(), 0.90, nil)
	if err != nil {
		t.Fatal(err)
	}
	if m == nil {
		t.Fatal("归一化后模板未命中（score < 0.90）")
	}
	if dx, dy := m.X-420, m.Y-270; dx < -2 || dx > 2 || dy < -2 || dy > 2 {
		t.Fatalf("命中中心 (%d,%d)，期望 (420,270)±2", m.X, m.Y)
	}
	if m.Score < 0.9 {
		t.Fatalf("score=%.3f < 0.9", m.Score)
	}
}

// 同尺寸 resize 应是无损恒等（目标像素中心恰好落在源像素上）。
func TestResizeRGBAIdentity(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 8, 6))
	for i := range src.Pix {
		src.Pix[i] = uint8(i * 7)
	}
	dst := resizeRGBA(src, 8, 6)
	for i := range src.Pix {
		if dst.Pix[i] != src.Pix[i] {
			t.Fatalf("pix[%d]=%d want %d", i, dst.Pix[i], src.Pix[i])
		}
	}
}
