package controller

import (
	"fmt"

	"github.com/flourbrain/mtga-farm-bot/internal/gamestate"
	"github.com/flourbrain/mtga-farm-bot/internal/input"
	"github.com/flourbrain/mtga-farm-bot/internal/vision"
)

// Python 竖排 Choose One：中心 (956, 408)，行距 107；横排 Wardens 左右偏移 216、y=480。
const (
	modalCenterX1920   = 956
	modalGroupY1920    = 408
	modalSpacingY1920  = 107
	wardensOffsetX1920 = 216
	wardensY1920       = 480
)

func modalLastPoint(nOptions int) vision.Point {
	if nOptions < 1 {
		nOptions = 1
	}
	// Python: int(group_center_y + ((n-1)/2.0)*spacing_y) — 向零截断
	y := int(float64(modalGroupY1920) + (float64(nOptions-1)/2.0)*float64(modalSpacingY1920))
	return vision.FromBase1920(modalCenterX1920, y)
}

func modalChoicePoint(job gamestate.ModalChoiceJob) (vision.Point, string) {
	switch job.Kind {
	case gamestate.ModalChoiceWardensGainLife:
		return vision.FromBase1920(modalCenterX1920-wardensOffsetX1920, wardensY1920), "MODAL_WARDENS_GAIN_LIFE"
	case gamestate.ModalChoiceWardensDraw:
		return vision.FromBase1920(modalCenterX1920+wardensOffsetX1920, wardensY1920), "MODAL_WARDENS_DRAW"
	default:
		return modalLastPoint(job.NOptions), "MODAL_LOSE_LIFE"
	}
}

// ClickModalChoice 点击结算异能 Choose One。
func (c *Controller) ClickModalChoice(job gamestate.ModalChoiceJob) error {
	pt, label := modalChoicePoint(job)
	c.log(fmt.Sprintf("BOT操作: %s 点击 @(%d,%d)", label, pt.X, pt.Y))
	return input.ClickClient(c.HWND, pt.X, pt.Y)
}
