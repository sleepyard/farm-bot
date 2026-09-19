package controller

import (
	"fmt"
	"time"

	"github.com/flourbrain/mtga-farm-bot/internal/input"
	"github.com/flourbrain/mtga-farm-bot/internal/vision"
)

// Concede ESC 打开菜单，等 5 秒后用现有搜图点击 concede.png。
func (c *Controller) Concede() error {
	c.log("BOT操作: 投降 — 按 Esc")
	input.FocusIfNeeded(c.HWND)
	if err := input.TapEscape(); err != nil {
		return fmt.Errorf("Esc: %w", err)
	}
	time.Sleep(5 * time.Second)
	c.log("BOT操作: 投降 — 搜图 concede.png")
	if pt, ok := c.findTemplate("assert/concede.png", vision.FullROI(), 0.80); ok {
		c.log(fmt.Sprintf("BOT操作: 投降 — 点击 assert/concede.png @(%d,%d)", pt.X, pt.Y))
		return input.ClickClient(c.HWND, pt.X, pt.Y)
	}
	if pt, ok := c.findTemplate("Buttons/concede.png", vision.FullROI(), 0.80); ok {
		c.log(fmt.Sprintf("BOT操作: 投降 — 点击 Buttons/concede.png @(%d,%d)", pt.X, pt.Y))
		return input.ClickClient(c.HWND, pt.X, pt.Y)
	}
	return fmt.Errorf("未找到 concede.png")
}
