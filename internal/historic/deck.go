package historic

import (
	"fmt"
	"time"

	"github.com/flourbrain/mtga-farm-bot/internal/input"
)

// ensureMyDecksOpen 展开 My Decks（已开则 skip，避免二次点击收起）。
func (n *navigator) ensureMyDecksOpen() bool {
	n.step("========== POST_LOGIN_MY_DECKS ==========")
	if n.myDecksGridOpen() {
		n.step("My Decks 网格已开，跳过点击标题")
		return true
	}
	// 优先 assert/nav，其次 Buttons/my_decks.png
	if n.clickTemplateOpts("assert/nav/nav_my_decks.png", "My Decks", roiDecksHeader, 0.80, locateOpts{noFallback: false}) ||
		n.clickTemplateOpts("Buttons/my_decks.png", "My Decks", roiDecksHeader, 0.80, locateOpts{}) {
		if n.sleep(1000 * time.Millisecond) {
			return false
		}
	} else {
		n.step("未找到 My Decks 标题，仍尝试确认网格")
	}
	if n.myDecksGridOpen() {
		return true
	}
	n.step("看不见我的套牌网格；不盲点第一套")
	return false
}

func (n *navigator) myDecksGridOpen() bool {
	return n.visibleQuiet("Buttons/my_decks_grid_open.png", roiDecksGrid, 0.80)
}

// clickFirstDeck 对齐 Python _click_first_deck_slot：固定点选列表第一套。
func (n *navigator) clickFirstDeck() bool {
	n.step("========== 选择默认第一套卡组 ==========")
	if !n.ensureMyDecksOpen() {
		return false
	}
	n.step(fmt.Sprintf("点击第一套卡组 @(%d,%d)", ptFirstDeck.X, ptFirstDeck.Y))
	if err := input.ClickClient(n.sess.HWND, ptFirstDeck.X, ptFirstDeck.Y); err != nil {
		n.step("点击第一套失败: " + err.Error())
		return false
	}
	_ = n.sleep(800 * time.Millisecond)
	return true
}

// pressQueuePlay 点右下角 Play 进入匹配。
func (n *navigator) pressQueuePlay() bool {
	n.step("========== 排队 Play ==========")
	if n.clickTemplate("Buttons/play_btn.png", "排队 Play", roiHomePlay, 0.75) {
		_ = n.sleep(1000 * time.Millisecond)
		return true
	}
	n.step(fmt.Sprintf("play_btn 未命中，固定点 @(%d,%d)", ptQueuePlay.X, ptQueuePlay.Y))
	if err := input.ClickClient(n.sess.HWND, ptQueuePlay.X, ptQueuePlay.Y); err != nil {
		n.step("排队点击失败: " + err.Error())
		return false
	}
	_ = n.sleep(1000 * time.Millisecond)
	return true
}
