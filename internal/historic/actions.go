package historic

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"time"

	"github.com/flourbrain/mtga-farm-bot/internal/assets"
	"github.com/flourbrain/mtga-farm-bot/internal/debugdump"
	"github.com/flourbrain/mtga-farm-bot/internal/input"
	"github.com/flourbrain/mtga-farm-bot/internal/vision"
)

type locateOpts struct {
	quiet      bool
	noPreview  bool
	noFallback bool
}

func (n *navigator) locateTemplate(rel string, roi vision.ROI, threshold float64) (vision.Point, bool) {
	return n.locateTemplateOpts(rel, roi, threshold, locateOpts{})
}

func (n *navigator) locateTemplateQuiet(rel string, roi vision.ROI, threshold float64) (vision.Point, bool) {
	return n.locateTemplateOpts(rel, roi, threshold, locateOpts{quiet: true, noPreview: true, noFallback: true})
}

func (n *navigator) locateTemplateOpts(rel string, roi vision.ROI, threshold float64, opt locateOpts) (vision.Point, bool) {
	if n.stopped() {
		return vision.Point{}, false
	}
	path := assetPath(rel)
	if _, err := os.Stat(path); err != nil {
		if !opt.quiet {
			n.step("模板缺失 " + rel)
		}
		return vision.Point{}, false
	}
	paths := assets.ExistingVariants(rel, assets.TemplateLang())
	_ = input.ParkCursor(n.sess.HWND)
	img, err := vision.CaptureClient(n.sess.HWND)
	if err != nil {
		if !opt.quiet {
			n.step("截图失败: " + err.Error())
		}
		return vision.Point{}, false
	}
	if !opt.noPreview {
		n.publishPreview(rel, img)
	}
	luma := vision.MeanLuma(img)
	if luma < 5 && !opt.quiet {
		n.step(fmt.Sprintf("截图像素过暗(mean=%.1f)，可能被遮挡或截图失败", luma))
	}

	var best *vision.Match
	m, _, err := vision.FindTemplateAnyBestOut(img, paths, roi, threshold, &best)
	if err != nil {
		if !opt.quiet {
			n.step("匹配错误 " + rel + ": " + err.Error())
		}
		return vision.Point{}, false
	}
	if m != nil {
		if !opt.quiet {
			n.markClickDebug(img, m.X, m.Y, rel)
		}
		return vision.Point{X: m.X, Y: m.Y}, true
	}

	if opt.quiet || opt.noFallback {
		if !opt.quiet && best != nil {
			n.step(fmt.Sprintf("未达阈值 %s best=%.3f@(%d,%d) need≥%.2f", rel, best.Score, best.X, best.Y, threshold))
		}
		return vision.Point{}, false
	}

	fallbackThr := threshold - 0.05
	if fallbackThr < 0.70 {
		fallbackThr = 0.70
	}
	var bestFull *vision.Match
	m2, _, err := vision.FindTemplateAnyBestOut(img, paths, vision.FullROI(), fallbackThr, &bestFull)
	if err == nil && m2 != nil {
		n.step(fmt.Sprintf("ROI 未命中，全屏找到 %s @(%d,%d) score=%.3f", rel, m2.X, m2.Y, m2.Score))
		n.markClickDebug(img, m2.X, m2.Y, rel)
		return vision.Point{X: m2.X, Y: m2.Y}, true
	}

	scoreMsg := "n/a"
	if bestFull != nil && (best == nil || bestFull.Score > best.Score) {
		best = bestFull
	}
	if best != nil {
		scoreMsg = fmt.Sprintf("%.3f@(%d,%d)", best.Score, best.X, best.Y)
	}
	n.step(fmt.Sprintf("未达阈值 %s best=%s need≥%.2f luma=%.1f", rel, scoreMsg, threshold, luma))
	n.saveDebugFrame(img, rel)
	return vision.Point{}, false
}

func (n *navigator) saveDebugFrame(img *image.RGBA, rel string) {
	out := debugdump.SavePNG(img, debugdump.MissName(rel))
	if out != "" {
		n.step("已保存调试截图 " + out)
	}
}

func (n *navigator) markClickDebug(img *image.RGBA, cx, cy int, rel string) {
	if img == nil {
		return
	}
	b := img.Bounds()
	red := color.RGBA{R: 255, A: 255}
	for dx := -12; dx <= 12; dx++ {
		x := cx + dx
		if x >= b.Min.X && x < b.Max.X && cy >= b.Min.Y && cy < b.Max.Y {
			img.SetRGBA(x, cy, red)
		}
	}
	for dy := -12; dy <= 12; dy++ {
		y := cy + dy
		if y >= b.Min.Y && y < b.Max.Y && cx >= b.Min.X && cx < b.Max.X {
			img.SetRGBA(cx, y, red)
		}
	}
	_ = debugdump.SavePNG(img, debugdump.ClickName(rel))
	n.publishPreview(rel, img)
}

func (n *navigator) clickTemplate(rel, label string, roi vision.ROI, threshold float64) bool {
	return n.clickTemplateOpts(rel, label, roi, threshold, locateOpts{})
}

func (n *navigator) clickTemplateOpts(rel, label string, roi vision.ROI, threshold float64, opt locateOpts) bool {
	pt, ok := n.locateTemplateOpts(rel, roi, threshold, opt)
	if !ok {
		if !opt.quiet {
			n.step(fmt.Sprintf("未找到 %s (%s)", label, rel))
		}
		return false
	}
	if err := input.ClickClient(n.sess.HWND, pt.X, pt.Y); err != nil {
		n.step("点击失败: " + err.Error())
		return false
	}
	n.step(fmt.Sprintf("已点击 %s (%s) @(%d,%d)", label, rel, pt.X, pt.Y))
	_ = n.sleep(400 * time.Millisecond)
	return true
}

func (n *navigator) visible(rel string, roi vision.ROI, threshold float64) bool {
	_, ok := n.locateTemplate(rel, roi, threshold)
	return ok
}

func (n *navigator) visibleQuiet(rel string, roi vision.ROI, threshold float64) bool {
	_, ok := n.locateTemplateQuiet(rel, roi, threshold)
	return ok
}

// waitVisible 轮询直到模板出现或超时；返回 true 表示命中。
func (n *navigator) waitVisible(rel string, roi vision.ROI, threshold float64, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if n.stopped() {
			return false
		}
		if n.visibleQuiet(rel, roi, threshold) {
			n.step(fmt.Sprintf("已确认 %s", rel))
			return true
		}
		if n.sleep(250 * time.Millisecond) {
			return false
		}
	}
	return false
}

func (n *navigator) dismissClaimPopup() {
	n.step("==========正在检测是否有奖励领取弹窗==========")
	if n.clickTemplateOpts("Buttons/claim.png", "领取奖励", vision.FullROI(), 0.85, locateOpts{quiet: true, noFallback: true}) {
		n.step("已关闭奖励弹窗")
		_ = n.sleep(800 * time.Millisecond)
		return
	}
	n.step("未发现奖励领取弹窗")
}
