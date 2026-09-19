package controller

import (
	"fmt"
	"path/filepath"
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

// ClickScryDone 在 1280×720 参考空间截图上 1:1 匹配 scry_done.png 并点击。
// 截图已在捕获时归一化到参考空间；模板按该空间截取，不缩放模板、不用 1920 坐标换算。
func (c *Controller) ClickScryDone() error {
	path := filepath.Join(assets.RootDir(), filepath.FromSlash(scryDoneRel))
	deadline := time.Now().Add(scryDoneTimeout)
	for {
		_ = input.ParkCursor(c.HWND)
		img, err := vision.CaptureClient(c.HWND)
		if err != nil {
			return err
		}
		m, err := vision.FindTemplate(img, path, vision.FullROI(), scryDoneConfidence)
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
