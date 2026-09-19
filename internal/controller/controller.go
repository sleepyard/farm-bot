// Package controller 执行决策器给出的鼠标 / 键盘动作，不含策略。
package controller

import (
	"fmt"
	"time"

	"github.com/flourbrain/mtga-farm-bot/internal/decisioner"
	"github.com/flourbrain/mtga-farm-bot/internal/input"
	"github.com/flourbrain/mtga-farm-bot/internal/vision"
)

var (
	roiAttack = vision.ROI{X: 900, Y: 560, W: 360, H: 150}
)

// Controller 绑定一个 MTGA 窗口。
type Controller struct {
	HWND    uintptr
	LogPath string // Player.log，手牌悬停扫 objectId 用
	Log     func(string)
}

func (c *Controller) log(msg string) {
	if c.Log != nil {
		c.Log(msg)
	}
}

// Execute 执行一条 Move；所有真实点击都带「BOT操作」前缀，便于和玩家手动操作区分。
func (c *Controller) Execute(m decisioner.Move) error {
	switch m.Kind {
	case decisioner.KindKeep:
		return c.KeepHand()
	case decisioner.KindMulligan:
		return c.Mulligan()
	case decisioner.KindResolve:
		return c.Resolve()
	case decisioner.KindAllAttack:
		return c.AllAttack()
	case decisioner.KindAssignAttackTargets:
		return c.AssignAttackTargets()
	case decisioner.KindNoAttacks:
		return c.NoAttacks()
	case decisioner.KindNoBlocks:
		return c.NoBlocks()
	case decisioner.KindPlayLand:
		return c.PlayCard(m.InstanceID, "打出地牌", m.InstanceTag())
	case decisioner.KindCast:
		return c.PlayCard(m.InstanceID, "施放", m.InstanceTag())
	case decisioner.KindActivate:
		return fmt.Errorf("尚未实现: activate %s", m.InstanceTag())
	case decisioner.KindSelectTarget:
		return c.SelectTarget(m)
	case decisioner.KindSelectN:
		ids := m.Targets
		if len(ids) == 0 && m.InstanceID > 0 {
			ids = []int{m.InstanceID}
		}
		return c.SelectHandCards(ids, "SelectN")
	case decisioner.KindWait:
		return nil
	default:
		return fmt.Errorf("未知动作: %s", m.Kind)
	}
}

// Resolve 点击右下角过优先权 / 确认。
func (c *Controller) Resolve() error {
	c.log("BOT操作: 过优先权 (resolve)")
	return c.clickConfirmOrResolve("过优先权")
}

// AllAttack 全攻。
func (c *Controller) AllAttack() error {
	c.log("BOT操作: 全攻")
	if pt, ok := c.findTemplate("assert/attack_all.png", roiAttack, 0.80); ok {
		return input.ClickClient(c.HWND, pt.X, pt.Y)
	}
	return input.ClickClient(c.HWND, ptResolve.X, ptResolve.Y)
}

// NoBlocks 不阻挡：点右下角确认（与 resolve 同按钮搜图）。
func (c *Controller) NoBlocks() error {
	c.log("BOT操作: 不阻挡 (no blocks)")
	return c.clickConfirmOrResolve("不阻挡")
}

// SelectTarget 选目标。
// 战场/堆叠/打脸：扫描或点头像后 submit。中屏墓地选牌（Zombify）单击即选定，不再点右下角。
func (c *Controller) SelectTarget(m decisioner.Move) error {
	targets := m.Targets
	friendly := m.FriendlyTarget
	face := len(targets) == 0
	for _, t := range targets {
		if t <= 0 {
			face = true
			continue
		}
		face = false
		tag := fmt.Sprintf("instance=%d||name=?", t)
		var err error
		switch {
		case m.Chooser:
			c.log(fmt.Sprintf("BOT操作: 选择目标 → 中屏选牌（墓地/放逐） %s", tag))
			err = c.SelectChooserCard(t)
		case m.Stack:
			c.log(fmt.Sprintf("BOT操作: 选择目标 → 堆叠 %s", tag))
			err = c.SelectStackItem(t)
		default:
			side := "对方"
			if friendly {
				side = "己方"
			}
			c.log(fmt.Sprintf("BOT操作: 选择目标 → %s生物 %s", side, tag))
			err = c.SelectBattlefieldCreature(t, friendly)
		}
		if err != nil {
			return err
		}
		time.Sleep(200 * time.Millisecond)
	}
	if face {
		c.log(fmt.Sprintf("BOT操作: 选择目标 → 对手头像 @(%d,%d)", ptOpponentAvatar.X, ptOpponentAvatar.Y))
		if err := input.ClickClient(c.HWND, ptOpponentAvatar.X, ptOpponentAvatar.Y); err != nil {
			return err
		}
	}
	if m.Chooser {
		return nil
	}
	time.Sleep(250 * time.Millisecond)
	c.log("BOT操作: 选目标后确认 (submit)")
	return c.clickConfirmOrResolve("选目标确认")
}
