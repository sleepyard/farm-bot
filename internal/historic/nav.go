// Package historic 实现史迹（15胜）开打前导航。
//
//	搜图 claim 领奖（防奖励遮罩）
//	→ Home 重新定位
//	→ 右下 Play
//	→ 搜图点击 find_match_btn.png
//	→ 搜图点击 find_match_anchor.png
//	→ 搜图点击 nav_historic_play.png
//	→ My Decks → 第一套卡组 → 右下 Play
//
// 坐标与模板均按 1280×720，无运行时缩放。
package historic

import (
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/flourbrain/mtga-farm-bot/internal/assets"
	"github.com/flourbrain/mtga-farm-bot/internal/input"
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
	OK      bool     `json:"ok"`
	Message string   `json:"message"`
	Steps   []string `json:"steps"`
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

// Navigate 走到史迹选牌界面，点默认第一套后排队。
func Navigate(sess Session, hooks Hooks) Result {
	n := &navigator{sess: sess, hooks: hooks}
	if hooks.Log == nil {
		n.hooks.Log = func(msg string) { log.Print(msg) }
	}
	if sess.HWND == 0 {
		return Result{OK: false, Message: "未捕获 MTGA 窗口", Steps: n.steps}
	}
	if abs(sess.Width-vision.LiveWidth) > 2 || abs(sess.Height-vision.LiveHeight) > 2 {
		return Result{
			OK:      false,
			Message: fmt.Sprintf("客户区须为 %dx%d，当前 %dx%d", vision.LiveWidth, vision.LiveHeight, sess.Width, sess.Height),
			Steps:   n.steps,
		}
	}

	input.Focus(sess.HWND)
	if n.stopped() {
		return fail(n, "已停止")
	}

	n.dismissClaimPopup()
	if n.stopped() {
		return fail(n, "已停止")
	}

	n.step("====================")
	n.step("史迹导航: 领奖 → Home → 右下Play → Find Match → 面板锚点 → Historic Play → My Decks → 第一套 → Play")

	if !n.goHome() {
		return fail(n, "未能回到 Home")
	}
	if n.stopped() {
		return fail(n, "已停止")
	}
	if !n.clickHomePlay() {
		return fail(n, "未能点击 Home 右下 Play")
	}
	if n.stopped() {
		return fail(n, "已停止")
	}
	if !n.clickTemplate("Buttons/find_match_btn.png", "Find Match", roiBladeTabs, 0.80) {
		return fail(n, "未找到 find_match_btn.png")
	}
	if n.sleep(600 * time.Millisecond) {
		return fail(n, "已停止")
	}
	if !n.clickTemplate("assert/find_match_anchor.png", "Find Match 面板", roiBladeContent, 0.80) {
		return fail(n, "未找到 find_match_anchor.png")
	}
	if n.sleep(600 * time.Millisecond) {
		return fail(n, "已停止")
	}
	if !n.clickTemplate("assert/nav/nav_historic_play.png", "Historic Play", roiFormatList, 0.80) {
		return fail(n, "未找到 nav_historic_play.png")
	}
	if n.sleep(800 * time.Millisecond) {
		return fail(n, "已停止")
	}

	if !n.clickFirstDeck() {
		return fail(n, "未能点击第一套卡组")
	}
	if n.stopped() {
		return fail(n, "已停止")
	}
	if !n.pressQueuePlay() {
		return fail(n, "未能点击排队 Play")
	}
	return ok(n, "进入匹配")
}

// goHome 点左上 Home 页签并确认 home_anchor（模板在未选中时对不上，故用固定点）。
func (n *navigator) goHome() bool {
	n.step("========== HOME 重新定位 ==========")
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

// clickHomePlay 点 Home 右下 Play，打开对战刀刃。
func (n *navigator) clickHomePlay() bool {
	n.step("========== Home 右下 Play ==========")
	if n.clickTemplate("Buttons/play_btn.png", "Home Play", roiHomePlay, 0.75) {
		return !n.sleep(800 * time.Millisecond)
	}
	n.step(fmt.Sprintf("play_btn 未命中，固定点 @(%d,%d)", ptQueuePlay.X, ptQueuePlay.Y))
	if err := input.ClickClient(n.sess.HWND, ptQueuePlay.X, ptQueuePlay.Y); err != nil {
		n.step("点击 Home Play 失败: " + err.Error())
		return false
	}
	n.step(fmt.Sprintf("已点击 Home Play @(%d,%d)", ptQueuePlay.X, ptQueuePlay.Y))
	return !n.sleep(800 * time.Millisecond)
}

func ok(n *navigator, msg string) Result {
	return Result{OK: true, Message: msg, Steps: n.steps}
}

func fail(n *navigator, msg string) Result {
	return Result{OK: false, Message: msg, Steps: n.steps}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
