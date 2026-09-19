package controller

import (
	"fmt"
	"path/filepath"

	"github.com/flourbrain/mtga-farm-bot/internal/assets"
	"github.com/flourbrain/mtga-farm-bot/internal/input"
	"github.com/flourbrain/mtga-farm-bot/internal/vision"
)

// 1280×720 参考空间固定点（由原版 1920 参考 ×2/3）；仅作无模板时的弱备。
var (
	ptResolve  = vision.Point{X: 1170, Y: 629} // (1755,944) 右下「下一优先权」
	ptMulligan = vision.Point{X: 546, Y: 580}
	// 对手头像：对齐 Python __get_avatar_retry_points 主点 (arena 宽×0.50, 高×0.10)。
	// 旧值 (857,144)=1920×(0.67,0.20)×2/3 偏右偏下，点不中头像。
	ptOpponentAvatar = vision.Point{X: 640, Y: 72}
)

// 起手 Keep 按钮搜索区（屏幕下半中部）。
var roiKeepHand = vision.ROI{X: 400, Y: 480, W: 480, H: 220}

const keepHandRel = "Buttons/keep_hand.png"

// FindKeepHand 只搜寻，不点击。阈值较高，避免把别的橙按钮当成 Keep。
func (c *Controller) FindKeepHand() (vision.Point, error) {
	pt, ok := c.findTemplate(keepHandRel, roiKeepHand, 0.88)
	if !ok {
		return vision.Point{}, fmt.Errorf("未找到 keep_hand.png")
	}
	return pt, nil
}

// ClickKeepHand 点击已找到的 Keep 位置。
func (c *Controller) ClickKeepHand(pt vision.Point) error {
	c.log("BOT操作: 保留手牌 (Buttons/keep_hand.png)")
	return input.ClickClient(c.HWND, pt.X, pt.Y)
}

// KeepHand 搜寻并点击 Buttons/keep_hand.png；找不到则报错（绝不点固定坐标）。
func (c *Controller) KeepHand() error {
	pt, err := c.FindKeepHand()
	if err != nil {
		return err
	}
	return c.ClickKeepHand(pt)
}

// Mulligan 点击再调度（暂用固定点；需要时可改模板）。
func (c *Controller) Mulligan() error {
	c.log("控制器: 再调度")
	return input.ClickClient(c.HWND, ptMulligan.X, ptMulligan.Y)
}

func (c *Controller) findTemplate(rel string, roi vision.ROI, thr float64) (vision.Point, bool) {
	_ = input.ParkCursor(c.HWND)
	path := filepath.Join(assets.RootDir(), filepath.FromSlash(rel))
	img, err := vision.CaptureClient(c.HWND)
	if err != nil {
		return vision.Point{}, false
	}
	m, err := vision.FindTemplateBest(img, path, roi, thr, nil)
	if err != nil || m == nil {
		return vision.Point{}, false
	}
	return vision.Point{X: m.X, Y: m.Y}, true
}
