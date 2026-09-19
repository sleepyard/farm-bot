package controller

import (
	"fmt"

	"github.com/flourbrain/mtga-farm-bot/internal/gamestate"
	"github.com/flourbrain/mtga-farm-bot/internal/input"
	"github.com/flourbrain/mtga-farm-bot/internal/vision"
)

// Python 1920×1080 Choose One 点击点 → 1280×720 ×2/3。
var (
	ptCastingTimePlain     = vision.FromBase1920(750, 505)  // 左侧不加 kicker
	ptCastingTimeModal2    = vision.FromBase1920(1185, 505) // 右侧模式（Valorous Stance 消灭）
	ptCastingTimeSacrifice = vision.FromBase1920(1775, 978) // 右下「牺牲生物」
)

func castingTimePoint(kind gamestate.CastingTimeKind) (vision.Point, string) {
	switch kind {
	case gamestate.CastingTimeSacrifice:
		return ptCastingTimeSacrifice, "CASTING_TIME_OPTION_SACRIFICE"
	case gamestate.CastingTimeModalSecond:
		return ptCastingTimeModal2, "CASTING_TIME_OPTION_MODAL_SECOND"
	default:
		return ptCastingTimePlain, "CASTING_TIME_OPTION_PLAIN"
	}
}

// AimCastingTime 移到 Choose One 选项（先瞄准，调用方在 settle 后再点）。
func (c *Controller) AimCastingTime(kind gamestate.CastingTimeKind) (vision.Point, string, error) {
	pt, label := castingTimePoint(kind)
	c.log(fmt.Sprintf("BOT操作: %s 瞄准 @(%d,%d)", label, pt.X, pt.Y))
	if err := input.MoveClient(c.HWND, pt.X, pt.Y); err != nil {
		return pt, label, err
	}
	return pt, label, nil
}

// ClickCastingTime 在当前光标处单击（对准 Python left_click(1)）。
func (c *Controller) ClickCastingTime() error {
	c.log("BOT操作: Choose One 单击")
	return input.LeftClick()
}
