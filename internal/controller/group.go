package controller

import (
	"fmt"
	"time"

	"github.com/flourbrain/mtga-farm-bot/internal/assets"
	"github.com/flourbrain/mtga-farm-bot/internal/input"
	"github.com/flourbrain/mtga-farm-bot/internal/vision"
)

const scryDoneRel = "Buttons/scry_done.png"

const (
	scryDoneConfidence = 0.78
	scryDoneTimeout    = 1500 * time.Millisecond
	scryDonePoll       = 150 * time.Millisecond
)

// ClickScryDone 在 1280×720 客户区原尺寸截图上 1:1 匹配 scry_done.png 并点击。
// 模板本身已按该分辨率截取，不缩放截图、不缩放模板、不用 1920 坐标换算。
func (c *Controller) ClickScryDone() error {
	paths := assets.ExistingVariants(scryDoneRel, assets.TemplateLang())
	deadline := time.Now().Add(scryDoneTimeout)
	for {
		_ = input.ParkCursor(c.HWND)
		img, err := vision.CaptureClient(c.HWND)
		if err != nil {
			return err
		}
		m, _, err := vision.FindTemplateAnyBest(img, paths, vision.FullROI(), scryDoneConfidence)
		if err != nil {
			return err
		}
		if m != nil {
			c.log(fmt.Sprintf("BOT操作: GROUP_REQ Done 命中 scry_done.png @(%d,%d) score=%.3f", m.X, m.Y, m.Score))
			return input.ClickClient(c.HWND, m.X, m.Y)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("未找到 scry_done.png")
		}
		time.Sleep(scryDonePoll)
	}
}
