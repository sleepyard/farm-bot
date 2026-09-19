package controller

import (
	"fmt"
	"time"

	"github.com/flourbrain/mtga-farm-bot/internal/input"
	"github.com/flourbrain/mtga-farm-bot/internal/vision"
)

const assignDamageDoneRel = "Buttons/assign_damage_done.png"
const noAttacksRel = "Buttons/no_attacks.png"
const noAttacksConfidence = 0.80

const (
	assignDamageConfidence = 0.82
	assignDamageTimeout    = 4 * time.Second
	assignDamagePoll       = 200 * time.Millisecond
)

// ClickAssignDamageDone 在 1280×720 客户区原尺寸截图上 1:1 匹配 assign_damage_done.png 并点击。
// 模板已按该分辨率截取，不缩放截图、不缩放模板、不用 1920 坐标换算。
func (c *Controller) ClickAssignDamageDone() error {
	deadline := time.Now().Add(assignDamageTimeout)
	for {
		if pt, ok := c.findTemplate(assignDamageDoneRel, vision.FullROI(), assignDamageConfidence); ok {
			c.log(fmt.Sprintf("BOT操作: ASSIGN_DAMAGE 点击 %s @(%d,%d)", assignDamageDoneRel, pt.X, pt.Y))
			return input.ClickClient(c.HWND, pt.X, pt.Y)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("未找到 assign_damage_done.png")
		}
		time.Sleep(assignDamagePoll)
	}
}

// NoAttacks 三次全攻仍停在宣告攻击时，整窗搜 no_attacks.png 点击（不能攻则过这一步）。
func (c *Controller) NoAttacks() error {
	c.log("BOT操作: 不攻击 — 搜图 no_attacks.png")
	input.ClickClient(c.HWND, ptResolve.X, ptResolve.Y)
	if pt, ok := c.findTemplate(noAttacksRel, vision.FullROI(), noAttacksConfidence); ok {
		c.log(fmt.Sprintf("BOT操作: 不攻击 — 点击 %s @(%d,%d)", noAttacksRel, pt.X, pt.Y))
		return input.ClickClient(c.HWND, pt.X, pt.Y)
	}
	return fmt.Errorf("未找到 no_attacks.png")
}

// AssignAttackTargets 对方有鹏洛克：点一次全攻，再点对手头像，全部打脸。
func (c *Controller) AssignAttackTargets() error {
	c.log("BOT操作: 对方有鹏洛克 — 全攻后再点头像打脸")
	if err := c.AllAttack(); err != nil {
		return err
	}
	time.Sleep(300 * time.Millisecond)
	c.log(fmt.Sprintf("BOT操作: 鹏洛克攻击 — 点对手头像 @(%d,%d)", ptOpponentAvatar.X, ptOpponentAvatar.Y))
	return input.ClickClient(c.HWND, ptOpponentAvatar.X, ptOpponentAvatar.Y)
}
