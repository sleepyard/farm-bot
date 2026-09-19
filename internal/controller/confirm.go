package controller

import (
	"fmt"
	"path/filepath"

	"github.com/flourbrain/mtga-farm-bot/internal/assets"
	"github.com/flourbrain/mtga-farm-bot/internal/input"
	"github.com/flourbrain/mtga-farm-bot/internal/vision"
)

// 确认按钮：整窗 1280×720 1:1 搜图，全部未命中才点 resolve 弱备。
var confirmTemplateRels = []string{
	"Buttons/next.png",
	"Buttons/scry_done.png",
	"Buttons/submit_btn.png",
}

const confirmConfidence = 0.80

// clickConfirmOrResolve 整窗依次搜 next.png、scry_done.png、submit_btn.png；都没有则点 resolve 弱备。
func (c *Controller) clickConfirmOrResolve(reason string) error {
	_ = input.ParkCursor(c.HWND)
	img, err := vision.CaptureClient(c.HWND)
	if err != nil {
		c.log(fmt.Sprintf("BOT操作: %s 截图失败，弱备 resolve @(%d,%d): %v", reason, ptResolve.X, ptResolve.Y, err))
		return input.ClickClient(c.HWND, ptResolve.X, ptResolve.Y)
	}
	roi := vision.FullROI()
	root := assets.RootDir()
	for _, rel := range confirmTemplateRels {
		path := filepath.Join(root, filepath.FromSlash(rel))
		m, ferr := vision.FindTemplateBest(img, path, roi, confirmConfidence, nil)
		if ferr != nil || m == nil {
			continue
		}
		c.log(fmt.Sprintf("BOT操作: %s 命中 %s @(%d,%d) score=%.3f", reason, rel, m.X, m.Y, m.Score))
		return input.ClickClient(c.HWND, m.X, m.Y)
	}
	c.log(fmt.Sprintf("BOT操作: %s 未命中 next/scry_done/submit_btn.png，弱备 resolve @(%d,%d)", reason, ptResolve.X, ptResolve.Y))
	return input.ClickClient(c.HWND, ptResolve.X, ptResolve.Y)
}
