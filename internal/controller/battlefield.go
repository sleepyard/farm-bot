package controller

import (
	"fmt"
	"time"

	"github.com/flourbrain/mtga-farm-bot/internal/input"
	"github.com/flourbrain/mtga-farm-bot/internal/playerlog"
)

// 1280×720：对齐 Python __select_object_in_region + battlefield_scan_*。
const (
	bfScanX0     = 128  // 0.10 * 1280
	bfScanX1     = 1178 // 0.92 * 1280
	bfOwnY0      = 360  // 0.50 * 720
	bfOwnY1      = 648  // 0.90 * 720
	bfOppY0      = 173  // 0.24 * 720
	bfOppY1      = 324  // 0.45 * 720
	bfScanStep   = 55   // Python battlefield_scan_step
	bfDwell      = 50 * time.Millisecond
	bfScanMax    = 4 * time.Second
	bfResetAbove = 80
	bfResetWait  = 100 * time.Millisecond
)

// SelectBattlefieldCreature 悬停扫描战场直到 objectId 命中，再原地单击。
func (c *Controller) SelectBattlefieldCreature(instanceID int, friendly bool) error {
	if instanceID <= 0 {
		return fmt.Errorf("无效战场 instanceId")
	}
	if c.LogPath == "" {
		return fmt.Errorf("未配置 Player.log，无法扫描战场")
	}
	y0, y1 := bfOppY0, bfOppY1
	label := "对方战场"
	if friendly {
		y0, y1 = bfOwnY0, bfOwnY1
		label = "己方战场"
	}
	tag := fmt.Sprintf("instance=%d||name=?", instanceID)
	c.log(fmt.Sprintf("BOT操作: %s扫描选目标 %s", label, tag))

	reader := &playerlog.Reader{Path: c.LogPath}
	hitX, hitY, err := c.scanRegionFor(reader, instanceID, tag, "战场", bfScanX0, bfScanX1, y0, y1, bfScanStep, bfScanMax)
	if err != nil {
		return err
	}
	c.log(fmt.Sprintf("BOT操作: %s已选中 %s @(%d,%d)", label, tag, hitX, hitY))
	return nil
}

// Python chooser_scan：1920 客户区 (0.20,0.29)-(0.78,0.70)，步长同战场，超时 8s。
const (
	chooserX0  = 256
	chooserX1  = 998
	chooserY0  = 209
	chooserY1  = 504
	chooserMax = 8 * time.Second
	stackX0    = 832
	stackX1    = 1216
	stackY0    = 180
	stackY1    = 432
	stackStep  = 80
	stackMax   = 3 * time.Second
)

// SelectChooserCard 中屏墓地/放逐选牌（Zombify 等）。
func (c *Controller) SelectChooserCard(instanceID int) error {
	if instanceID <= 0 {
		return fmt.Errorf("无效选牌 instanceId")
	}
	if c.LogPath == "" {
		return fmt.Errorf("未配置 Player.log，无法扫描选牌 overlay")
	}
	tag := fmt.Sprintf("instance=%d||name=?", instanceID)
	c.log(fmt.Sprintf("BOT操作: 中屏选牌扫描 %s", tag))
	reader := &playerlog.Reader{Path: c.LogPath}
	hitX, hitY, err := c.scanRegionFor(reader, instanceID, tag, "中屏选牌", chooserX0, chooserX1, chooserY0, chooserY1, bfScanStep, chooserMax)
	if err != nil {
		return err
	}
	c.log(fmt.Sprintf("BOT操作: 中屏选牌已选中 %s @(%d,%d)", tag, hitX, hitY))
	return nil
}

// SelectStackItem 扫描堆叠上的法术。
func (c *Controller) SelectStackItem(instanceID int) error {
	if instanceID <= 0 {
		return fmt.Errorf("无效堆叠 instanceId")
	}
	if c.LogPath == "" {
		return fmt.Errorf("未配置 Player.log，无法扫描堆叠")
	}
	tag := fmt.Sprintf("instance=%d||name=?", instanceID)
	c.log(fmt.Sprintf("BOT操作: 堆叠扫描 %s", tag))
	reader := &playerlog.Reader{Path: c.LogPath}
	hitX, hitY, err := c.scanRegionFor(reader, instanceID, tag, "堆叠", stackX0, stackX1, stackY0, stackY1, stackStep, stackMax)
	if err != nil {
		return err
	}
	c.log(fmt.Sprintf("BOT操作: 堆叠已选中 %s @(%d,%d)", tag, hitX, hitY))
	return nil
}

func (c *Controller) scanRegionFor(reader *playerlog.Reader, instanceID int, tag, label string, x0, x1, y0, y1, step int, timeout time.Duration) (int, int, error) {
	if step < 1 {
		step = 1
	}
	resetY := y0 - bfResetAbove
	if resetY < 0 {
		resetY = 0
	}
	_ = input.MoveClient(c.HWND, x0, resetY)
	time.Sleep(bfResetWait)

	deadline := time.Now().Add(timeout)
	for y := y0; y <= y1; y += step {
		for x := x0; x <= x1; x += step {
			if time.Now().After(deadline) {
				return 0, 0, fmt.Errorf("%s扫描超时未悬停到 %s", label, tag)
			}
			if err := input.MoveClient(c.HWND, x, y); err != nil {
				return 0, 0, err
			}
			base, err := reader.Size()
			if err != nil {
				return 0, 0, err
			}
			time.Sleep(bfDwell)
			chunk, err := reader.Since(base, 512_000, false)
			if err != nil {
				continue
			}
			if lastObjectID(chunk) != instanceID {
				continue
			}
			c.log(fmt.Sprintf("BOT操作: %s悬停命中 %s @(%d,%d)，结束扫描", label, tag, x, y))
			if err := input.LeftClick(); err != nil {
				return 0, 0, err
			}
			time.Sleep(100 * time.Millisecond)
			return x, y, nil
		}
	}
	return 0, 0, fmt.Errorf("%s扫描未悬停到 %s", label, tag)
}
