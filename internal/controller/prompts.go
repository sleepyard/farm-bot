package controller

import (
	"fmt"
	"time"

	"github.com/flourbrain/mtga-farm-bot/internal/input"
	"github.com/flourbrain/mtga-farm-bot/internal/playerlog"
)

// SelectN 弃牌弹窗手牌会抬高；对齐 Python select_hand_card_offset(-120/-200)。
var selectNScanYs = []int{handScanY, handScanY - 120, handScanY - 200}

// SelectHandCards 手牌单点选中（SelectN / 弃牌），然后点右下角 Submit。
func (c *Controller) SelectHandCards(instanceIDs []int, label string) error {
	if len(instanceIDs) == 0 {
		return fmt.Errorf("SelectN 无候选 instance")
	}
	if c.LogPath == "" {
		return fmt.Errorf("未配置 Player.log，无法扫描手牌")
	}
	reader := &playerlog.Reader{Path: c.LogPath}
	time.Sleep(800 * time.Millisecond) // 等弃牌/选牌弹窗抬起
	for _, id := range instanceIDs {
		tag := fmt.Sprintf("instance=%d||name=?", id)
		c.log(fmt.Sprintf("BOT操作: %s 扫描选中 %s", label, tag))
		hitX, hitY, err := c.scanSelectNCard(reader, id, tag)
		if err != nil {
			return err
		}
		time.Sleep(hoverSettle)
		if !c.freshHoverIs(reader, id) {
			return fmt.Errorf("选中前悬停已不是 %s", tag)
		}
		if err := input.MoveClient(c.HWND, hitX, hitY); err != nil {
			return err
		}
		time.Sleep(80 * time.Millisecond)
		if err := input.LeftClick(); err != nil {
			return err
		}
		c.log(fmt.Sprintf("BOT操作: %s 已单击选中 %s @(%d,%d)", label, tag, hitX, hitY))
		time.Sleep(200 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)
	return c.SubmitPrompt("SelectN 提交")
}

func (c *Controller) scanSelectNCard(reader *playerlog.Reader, instanceID int, tag string) (int, int, error) {
	var lastErr error
	for _, y := range selectNScanYs {
		x, hitY, err := c.scanHandAt(reader, instanceID, tag, handSweepPacing[0], y)
		if err == nil {
			return x, hitY, nil
		}
		lastErr = err
	}
	for _, pace := range handSweepPacing[1:] {
		time.Sleep(handScanRetryPause)
		x, hitY, err := c.scanHandAt(reader, instanceID, tag, pace, handScanY)
		if err == nil {
			return x, hitY, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("手牌扫描未悬停到 %s", tag)
	}
	return 0, 0, lastErr
}

// SubmitPrompt 点确认（SelectN / 过优先权共用）。
func (c *Controller) SubmitPrompt(label string) error {
	c.log("BOT操作: " + label + " (submit)")
	return c.clickConfirmOrResolve(label)
}
