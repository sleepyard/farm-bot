package starter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/flourbrain/mtga-farm-bot/internal/assets"
	"github.com/flourbrain/mtga-farm-bot/internal/input"
	"github.com/flourbrain/mtga-farm-bot/internal/quest"
	"github.com/flourbrain/mtga-farm-bot/internal/vision"
)

const (
	screenChooser = "chooser"
	screenPlay    = "play"
	screenUnknown = "unknown"
)

func (n *navigator) detectScreen() string {
	// 先判活动落地页（event_title + event_play），避免每次先扫 view_deck 浪费十几秒。
	if n.visibleQuiet("Buttons/event_title.png", roiEventTitle, 0.90) &&
		n.visibleQuiet("Buttons/event_play.png", roiEventPlay, 0.90) {
		return screenPlay
	}
	if n.visibleQuiet("Buttons/view_deck.png", roiViewDeck, 0.90) ||
		n.visibleQuiet("Buttons/view_deck_active.png", roiViewDeck, 0.90) {
		return screenChooser
	}
	return screenUnknown
}

func (n *navigator) openChooser() bool {
	for step := 0; step < 6; step++ {
		switch n.detectScreen() {
		case screenChooser:
			n.step("选牌器已在屏幕上")
			return true
		case screenPlay:
			// 刚进新手套牌对决落地页：直接点套牌盒，不再绕 view_deck。
			n.step(fmt.Sprintf("落地页，点击套牌盒打开选牌器 @(%d,%d)", ptDeckBox.X, ptDeckBox.Y))
			_ = input.ClickClient(n.sess.HWND, ptDeckBox.X, ptDeckBox.Y)
			if n.sleep(1500 * time.Millisecond) {
				return false
			}
			n.step("已点击套牌盒，开始选牌")
			return true
		default:
			// 横幅刚点进来时 title/play 模板可能尚未稳定：首步仍直接点套牌盒。
			if step == 0 {
				n.step(fmt.Sprintf("进入活动后直接点击套牌盒 @(%d,%d)", ptDeckBox.X, ptDeckBox.Y))
				_ = input.ClickClient(n.sess.HWND, ptDeckBox.X, ptDeckBox.Y)
				if n.sleep(1500 * time.Millisecond) {
					return false
				}
				n.step("已点击套牌盒，开始选牌")
				return true
			}
			n.step("无法识别活动屏，点返回箭头")
			_ = input.ClickClient(n.sess.HWND, ptBackArrow.X, ptBackArrow.Y)
			if n.sleep(1200 * time.Millisecond) {
				return false
			}
		}
	}
	n.step("无法到达选牌器，保留当前套牌")
	return false
}

func (n *navigator) chooseDeckTemplate(colors string) (absPath, stem string) {
	if colors == "" {
		return "", ""
	}
	dir := filepath.Join(assets.RootDir(), "assert", "starter_decks")
	entries, err := os.ReadDir(dir)
	if err != nil {
		n.step("未找到 starter_decks 目录")
		return "", ""
	}
	target := map[rune]struct{}{}
	for _, c := range strings.ToUpper(colors) {
		if strings.ContainsRune("WUBRGC", c) {
			target[c] = struct{}{}
		}
	}
	bestName := ""
	bestScore, bestExtra := -1, 999
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		low := strings.ToLower(name)
		if !strings.HasSuffix(low, ".png") && !strings.HasSuffix(low, ".jpg") && !strings.HasSuffix(low, ".jpeg") {
			continue
		}
		stemLetters := quest.NormalizeDeckStem(strings.TrimSuffix(name, filepath.Ext(name)))
		letters := map[rune]struct{}{}
		for _, c := range stemLetters {
			letters[c] = struct{}{}
		}
		score := 0
		for c := range target {
			if _, ok := letters[c]; ok {
				score++
			}
		}
		extra := len(letters) - score
		if score > bestScore || (score == bestScore && extra < bestExtra) {
			bestScore, bestExtra = score, extra
			bestName = name
		}
	}
	if bestScore <= 0 || bestName == "" {
		n.step("无匹配新手套牌模板，保留当前套牌")
		return "", ""
	}
	stem = strings.ToUpper(quest.NormalizeDeckStem(strings.TrimSuffix(bestName, filepath.Ext(bestName))))
	return filepath.Join(dir, bestName), stem
}

func (n *navigator) swapDeck(colors string) {
	tplPath, name := n.chooseDeckTemplate(colors)
	if tplPath == "" {
		return
	}
	n.step("换牌 → " + name)
	if !n.openChooser() {
		return
	}

	_ = input.ParkCursor(n.sess.HWND)
	img, err := vision.CaptureClient(n.sess.HWND)
	if err != nil {
		n.step("换牌截图失败: " + err.Error())
		return
	}
	scales := make([]float64, 0, 7)
	for i := 0; i <= 6; i++ {
		scales = append(scales, 0.7+0.1*float64(i))
	}
	m, err := vision.FindTemplateMultiScale(img, tplPath, vision.ROI{X: 0, Y: 120, W: 1280, H: 520}, 0.80, scales)
	clicked := false
	if err == nil && m != nil {
		clickY := m.Y + 40
		_ = input.ClickClient(n.sess.HWND, m.X, clickY)
		n.step(fmt.Sprintf("图像选中套牌 %s @(%d,%d)", name, m.X, clickY))
		clicked = true
	}
	if !clicked {
		gp, ok := deckGridPoint(name)
		if !ok {
			n.step("无网格坐标，放弃换牌")
			return
		}
		_ = input.ClickClient(n.sess.HWND, gp.X, gp.Y)
		n.step(fmt.Sprintf("网格坐标选中 %s @(%d,%d)", name, gp.X, gp.Y))
	}
	if n.sleep(1200 * time.Millisecond) {
		return
	}

	if !n.clickTemplate("Buttons/submit_deck.png", "Submit Deck", roiEventPlay, 0.85) {
		_ = input.ClickClient(n.sess.HWND, ptSubmitDeck.X, ptSubmitDeck.Y)
		n.step(fmt.Sprintf("Submit Deck 固定点 @(%d,%d)", ptSubmitDeck.X, ptSubmitDeck.Y))
	}
	if n.sleep(1500 * time.Millisecond) {
		return
	}

	// 提交后若仍停在选牌器，点返回；落地页有 event_play 时不会再扫 view_deck。
	if n.detectScreen() == screenChooser {
		n.step("提交后仍在选牌器，点返回")
		_ = input.ClickClient(n.sess.HWND, ptBackArrow.X, ptBackArrow.Y)
		_ = n.sleep(1000 * time.Millisecond)
	}
}
