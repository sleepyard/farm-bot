// Package starter 实现新手套牌对决开打前导航（任务 → 换牌 → Play）。
// 不包含对局内决策 / 出牌。坐标与模板均按 1280×720 参考空间；截图捕获时归一化，点击发送时换算。
package starter

import (
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/flourbrain/mtga-farm-bot/internal/assets"
	"github.com/flourbrain/mtga-farm-bot/internal/input"
	"github.com/flourbrain/mtga-farm-bot/internal/playerlog"
	"github.com/flourbrain/mtga-farm-bot/internal/quest"
	"github.com/flourbrain/mtga-farm-bot/internal/vision"
)

// Session 是一次导航所需的窗口句柄与客户区尺寸。
type Session struct {
	HWND   uintptr
	Width  int
	Height int
}

// Result 是导航结果。
type Result struct {
	OK            bool     `json:"ok"`
	Message       string   `json:"message"`
	TargetColors  string   `json:"target_colors,omitempty"`
	TargetReason  string   `json:"target_reason,omitempty"`
	Steps         []string `json:"steps"`
	UsedShortPath bool     `json:"used_short_path,omitempty"`
}

type navigator struct {
	sess  Session
	steps []string
	hooks Hooks
}

func (n *navigator) step(msg string) {
	n.steps = append(n.steps, msg)
	if n.hooks.Log != nil {
		n.hooks.Log(msg)
	} else {
		log.Print(msg)
	}
}

func assetPath(rel string) string {
	return filepath.Join(assets.RootDir(), filepath.FromSlash(rel))
}

// Navigate 执行完整新手导航，直到点击 event_play（或失败 / 停止）。
func Navigate(sess Session, quests []playerlog.Quest, hooks Hooks) Result {
	n := &navigator{sess: sess, hooks: hooks}
	if hooks.Log == nil {
		n.hooks.Log = func(msg string) { log.Print(msg) }
	}
	if sess.HWND == 0 {
		return Result{OK: false, Message: "未捕获 MTGA 窗口", Steps: n.steps}
	}
	if sess.Width != vision.LiveWidth || sess.Height != vision.LiveHeight {
		n.step(fmt.Sprintf("警告: 非推荐分辨率 %dx%d（推荐 %dx%d），截图将归一化到参考空间", sess.Width, sess.Height, vision.LiveWidth, vision.LiveHeight))
	}

	target := quest.ResolveTarget(quests)
	n.step("" + target.Reason)
	if target.Colors != "" {
		n.step("目标套牌颜色 " + target.Colors)
	}

	input.Focus(sess.HWND)
	if n.stopped() {
		return failResult(n, target, "已停止", false)
	}

	// 1) 先清奖励弹窗
	n.dismissClaimPopup()
	if n.stopped() {
		return failResult(n, target, "已停止", false)
	}

	// 2) 再判断是否已在事件活动页
	onEvent := n.detectEventLanding()
	if n.stopped() {
		return failResult(n, target, "已停止", false)
	}

	// 3) 进入导航链路：已在事件页走短路，否则从 Home 完整导航
	if onEvent {
		n.step("====================")
		n.step("开始导航链路（事件页短路：换卡组 → Play）")
		n.swapDeck(target.Colors)
		if n.stopped() {
			return failResult(n, target, "已停止", true)
		}
		if n.pressEventPlay() {
			return okResult(n, target, true)
		}
		return failResult(n, target, "事件页未找到 Play 按钮", true)
	}

	n.step("====================")
	n.step("开始导航链路 Home → Play → Events → 进行中 → 横幅 → 换卡组 → Play")

	_ = n.clickTemplate("Buttons/play_btn.png", "打开对战菜单", roiHomePlay, 0.75)
	if n.sleep(800 * time.Millisecond) {
		return failResult(n, target, "已停止", false)
	}

	if !n.clickTemplate("assert/events_tab.png", "打开活动页", roiEventsTab, 0.74) {
		// Events 未命中时再探一次事件页，避免漏掉局间落地
		if n.detectEventLanding() {
			n.step("Events 未命中但已在事件页，改走短路")
			n.swapDeck(target.Colors)
			if n.pressEventPlay() {
				return okResult(n, target, true)
			}
			return failResult(n, target, "事件页未找到 Play", true)
		}
		return failResult(n, target, "未找到活动页签", false)
	}
	if n.sleep(800 * time.Millisecond) {
		return failResult(n, target, "已停止", false)
	}

	inProgPt, foundIP := n.locateTemplate("assert/in_progress_label.png", roiInProgress, 0.80)
	if foundIP {
		_ = input.ClickClient(sess.HWND, inProgPt.X, inProgPt.Y)
		n.step("已点击「In Progress」")
		if n.sleep(1200 * time.Millisecond) {
			return failResult(n, target, "已停止", false)
		}
	} else {
		n.step("未点「In Progress」（可能已选中）")
	}

	if !n.findAndClickBanner() {
		n.step("「In Progress」下未找到横幅，尝试「All」筛选")
		n.clickAllFilter(inProgPt, foundIP)
		if n.sleep(1200 * time.Millisecond) {
			return failResult(n, target, "已停止", false)
		}
		if !n.findAndClickBanner() {
			return failResult(n, target, "未找到新手套牌对决横幅", false)
		}
	}
	if n.sleep(1500 * time.Millisecond) {
		return failResult(n, target, "已停止", false)
	}

	n.swapDeck(target.Colors)
	if n.stopped() {
		return failResult(n, target, "已停止", false)
	}

	if n.pressEventPlay() {
		return okResult(n, target, false)
	}
	if n.clickTemplate("Buttons/play_btn.png", "Play", roiPlayConfirm, 0.80) {
		n.step("已用 play_btn 点击")
		return okResult(n, target, false)
	}
	return failResult(n, target, "未能点击事件 Play", false)
}

func okResult(n *navigator, t quest.Target, short bool) Result {
	return Result{
		OK: true, Message: "进入匹配",
		TargetColors: t.Colors, TargetReason: t.Reason,
		Steps: n.steps, UsedShortPath: short,
	}
}

func failResult(n *navigator, t quest.Target, msg string, short bool) Result {
	return Result{
		OK: false, Message: msg,
		TargetColors: t.Colors, TargetReason: t.Reason,
		Steps: n.steps, UsedShortPath: short,
	}
}

// DipHomeForQuests 领奖后点回 Home，让客户端写出 QuestGetQuests。失败不阻断后续排队。
func DipHomeForQuests(sess Session, hooks Hooks) bool {
	n := &navigator{sess: sess, hooks: hooks}
	if hooks.Log == nil {
		n.hooks.Log = func(msg string) { log.Print(msg) }
	}
	if sess.HWND == 0 {
		n.step("任务刷新: 未捕获窗口")
		return false
	}
	input.Focus(sess.HWND)
	n.dismissClaimPopup()
	if n.stopped() {
		return false
	}
	return n.goHome()
}

func (n *navigator) goHome() bool {
	n.step("========== 任务刷新：回到 Home ==========")
	for i := 1; i <= 3; i++ {
		if n.stopped() {
			return false
		}
		n.step(fmt.Sprintf("点击 Home 页签 @(%d,%d) 第 %d 次", ptHomeTab.X, ptHomeTab.Y, i))
		if err := input.ClickClient(n.sess.HWND, ptHomeTab.X, ptHomeTab.Y); err != nil {
			n.step("点击 Home 失败: " + err.Error())
			return false
		}
		_ = input.ParkCursor(n.sess.HWND)
		if n.sleep(1500 * time.Millisecond) {
			return false
		}
		if n.waitVisible("assert/home_anchor.png", roiHomeAnchor, 0.75, 2*time.Second) {
			n.step("已到达 Home")
			return true
		}
		n.step("Home 锚点未确认，重试")
	}
	return false
}
