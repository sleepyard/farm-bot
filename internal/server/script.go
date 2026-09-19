package server

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/flourbrain/mtga-farm-bot/internal/appstate"
	"github.com/flourbrain/mtga-farm-bot/internal/assets"
	"github.com/flourbrain/mtga-farm-bot/internal/historic"
	"github.com/flourbrain/mtga-farm-bot/internal/input"
	"github.com/flourbrain/mtga-farm-bot/internal/match"
	"github.com/flourbrain/mtga-farm-bot/internal/playerlog"
	"github.com/flourbrain/mtga-farm-bot/internal/poweroff"
	"github.com/flourbrain/mtga-farm-bot/internal/quest"
	"github.com/flourbrain/mtga-farm-bot/internal/starter"
	"github.com/flourbrain/mtga-farm-bot/internal/vision"
	"github.com/flourbrain/mtga-farm-bot/internal/window"
)

type scriptState struct {
	mu            sync.Mutex
	running       bool
	stop          atomic.Bool
	templateRel   string
	templatePNG   []byte
	framePNG      []byte
	previewSeq    int64
	lastMessage   string
	lastOK        bool
	steps         []string
	targetColors  string
	targetReason  string
	logLines      []string
	sessionWins   int
	sessionLosses int
	quests        []playerlog.Quest
}

var script = &scriptState{}

const sessionWinGoal = 15

func (s *scriptState) setPreview(rel string, frame *image.RGBA) {
	var frameBuf bytes.Buffer
	if frame != nil {
		_ = png.Encode(&frameBuf, frame)
	}
	abs := filepath.Join(assets.RootDir(), filepath.FromSlash(rel))
	tplBytes, _ := os.ReadFile(abs)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.templateRel = rel
	s.templatePNG = tplBytes
	s.framePNG = frameBuf.Bytes()
	s.previewSeq++
}

func (s *scriptState) setGameFrame(frame *image.RGBA) {
	var frameBuf bytes.Buffer
	if frame != nil {
		_ = png.Encode(&frameBuf, frame)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.framePNG = frameBuf.Bytes()
	s.previewSeq++
}

func (s *scriptState) appendLog(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logLines = append(s.logLines, msg)
	if len(s.logLines) > 500 {
		s.logLines = s.logLines[len(s.logLines)-400:]
	}
}

func (s *scriptState) snapshotStatus(since int) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	if since < 0 {
		since = 0
	}
	if since > len(s.logLines) {
		since = len(s.logLines)
	}
	newLogs := append([]string(nil), s.logLines[since:]...)
	mode := appStore.Mode()
	return map[string]any{
		"ok":                  true,
		"running":             s.running,
		"message":             s.lastMessage,
		"last_ok":             s.lastOK,
		"game_mode":           mode,
		"mode_label":          appstate.ModeLabel(mode),
		"auto_switch":         appStore.AutoSwitch(),
		"shutdown_after_wins": appStore.ShutdownAfterWins(),
		"target_colors":       s.targetColors,
		"target_reason":       s.targetReason,
		"template_rel":        s.templateRel,
		"preview_seq":         s.previewSeq,
		"log_offset":          len(s.logLines),
		"logs":                newLogs,
		"steps":               append([]string(nil), s.steps...),
		"session_wins":        s.sessionWins,
		"session_losses":      s.sessionLosses,
		"win_goal":            sessionWinGoal,
		"quests":              append([]playerlog.Quest(nil), s.quests...),
	}
}

func handleScriptStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ok, msg, mode := startScript()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": ok, "message": msg, "running": ok, "mode": mode,
	})
}

// startScript 开始脚本（HTTP / F9 共用）。返回 ok、消息、当前模式。
func startScript() (ok bool, msg string, mode string) {
	mode = appStore.Mode()
	if mode != appstate.ModeStarter && mode != appstate.ModeHistoric {
		return false, "未知模式: " + mode, mode
	}

	script.mu.Lock()
	if script.running {
		script.mu.Unlock()
		return false, "脚本已在运行", mode
	}
	script.running = true
	script.stop.Store(false)
	script.logLines = nil
	script.steps = nil
	script.lastMessage = "运行中"
	script.lastOK = false
	script.templateRel = ""
	script.templatePNG = nil
	script.framePNG = nil
	script.previewSeq = 0
	script.sessionWins = 0
	script.sessionLosses = 0
	script.quests = nil
	script.mu.Unlock()

	if lastHWND == 0 {
		cap := window.CaptureMTGA()
		if !cap.OK || cap.Candidate == nil {
			fail := "请先捕获 MTGA 窗口: " + cap.Message
			script.mu.Lock()
			script.running = false
			script.lastMessage = fail
			script.mu.Unlock()
			return false, fail, mode
		}
		rememberCapture(cap)
	}

	snap := capturePlayerLogSnapshot()
	quests := snap.Quests
	if len(quests) == 0 {
		quests = lastQuests
	} else {
		lastQuests = quests
	}
	setScriptQuests(quests)

	mode = applyQueueMode(quests, nil)

	go runScript(quests)
	return true, "脚本已开始（" + appstate.ModeLabel(mode) + "）", mode
}

func applyQueueMode(quests []playerlog.Quest, logFn func(string)) string {
	cur := appStore.Mode()
	target := quest.ResolveTarget(quests)
	allDone := target.AllDone || len(quests) == 0
	next := appstate.AutoSwitchTarget(cur, appStore.AutoSwitch(), allDone)
	if next == cur {
		if appStore.AutoSwitch() && cur == appstate.ModeStarter {
			reason := target.Reason
			if reason == "" {
				reason = "每日任务尚未完成"
			}
			msg := "自动切换未触发: " + reason
			if logFn != nil {
				logFn(msg)
			} else {
				log.Print(msg)
				script.appendLog(msg)
			}
		}
		return cur
	}
	appStore.SetMode(next)
	msg := "每日任务已全部完成，自动切换到史迹模式"
	if len(quests) == 0 {
		msg = "未发现未完成的每日任务，自动切换到史迹模式"
	}
	if logFn != nil {
		logFn(msg)
	} else {
		log.Print(msg)
		script.appendLog(msg)
	}
	return next
}

// stopScript 请求停止脚本（HTTP / F9 共用）。
func stopScript() (ok bool, msg string) {
	script.mu.Lock()
	running := script.running
	script.mu.Unlock()
	if !running {
		return false, "脚本未在运行"
	}
	script.stop.Store(true)
	script.appendLog("收到停止请求")
	return true, "正在停止…"
}

// toggleScript F9：运行中则停，否则开。
func toggleScript() {
	script.mu.Lock()
	running := script.running
	script.mu.Unlock()
	if running {
		ok, msg := stopScript()
		log.Printf("热键 F9: 停止脚本 ok=%v %s", ok, msg)
		return
	}
	ok, msg, _ := startScript()
	log.Printf("热键 F9: 开始脚本 ok=%v %s", ok, msg)
	if ok {
		script.appendLog("热键 F9: " + msg)
	} else {
		script.appendLog("热键 F9 启动失败: " + msg)
	}
}

func runScript(quests []playerlog.Quest) {
	logFn := func(msg string) {
		log.Print(msg)
		script.appendLog(msg)
	}
	preview := func(rel string, frame *image.RGBA) {
		script.setPreview(rel, frame)
	}
	shouldStop := func() bool { return script.stop.Load() }

	log.Printf("开始…（循环直到停止）")
	round := 0
	var (
		lastOK           bool
		lastMsg          string
		lastSteps        []string
		lastTargetColors string
		lastTargetReason string
	)

	for !shouldStop() {
		round++
		logFn(fmt.Sprintf("========== 第 %d 轮：从头导航 ==========", round))

		snap := capturePlayerLogSnapshot()
		if len(snap.Quests) > 0 {
			quests = snap.Quests
			lastQuests = quests
		} else if len(quests) == 0 {
			quests = lastQuests
		}
		setScriptQuests(quests)

		mode := applyQueueMode(quests, logFn)

		var (
			ok           bool
			msg          string
			steps        []string
			targetColors string
			targetReason string
		)

		switch mode {
		case appstate.ModeHistoric:
			logFn("模式: 史迹（15胜）— 领奖 → Home → 右下Play → Find Match → 面板锚点 → Historic → My Decks → 第一套 → Play")
			res := historic.Navigate(historic.Session{
				HWND: lastHWND, Width: lastWidth, Height: lastHeight,
			}, historic.Hooks{Log: logFn, Preview: preview, ShouldStop: shouldStop})
			ok, msg, steps = res.OK, res.Message, res.Steps
			targetReason = "史迹默认第一套"
		default:
			logFn("模式: 新手套牌对决")
			res := starter.Navigate(starter.Session{
				HWND: lastHWND, Width: lastWidth, Height: lastHeight,
			}, quests, starter.Hooks{Log: logFn, Preview: preview, ShouldStop: shouldStop})
			ok, msg, steps = res.OK, res.Message, res.Steps
			targetColors, targetReason = res.TargetColors, res.TargetReason
		}

		if shouldStop() {
			lastOK, lastMsg, lastSteps = ok, msg+"（已停止）", steps
			lastTargetColors, lastTargetReason = targetColors, targetReason
			break
		}

		if !ok {
			logFn("本轮导航失败: " + msg + " — 2 秒后从头重试")
			script.mu.Lock()
			script.lastOK = false
			script.lastMessage = msg
			script.steps = append([]string(nil), steps...)
			script.targetColors = targetColors
			script.targetReason = targetReason
			script.mu.Unlock()
			lastOK, lastMsg, lastSteps = false, msg, steps
			lastTargetColors, lastTargetReason = targetColors, targetReason
			if !sleepUntilStop(2*time.Second, shouldStop) {
				break
			}
			continue
		}

		logFn("导航成功: " + msg + " — 启动对局决策循环")
		logPath, _ := playerlog.ResolvePath()
		matchRes := match.Run(context.Background(), &match.Session{
			HWND:    lastHWND,
			LogPath: logPath,
			Mode:    mode,
			Hooks: match.Hooks{
				Log:        logFn,
				ShouldStop: shouldStop,
			},
		})
		ok = matchRes.OK
		msg = msg + " → " + matchRes.Message
		lastOK, lastMsg, lastSteps = ok, msg, steps
		lastTargetColors, lastTargetReason = targetColors, targetReason

		script.mu.Lock()
		script.lastOK = ok
		script.lastMessage = msg
		script.steps = append([]string(nil), steps...)
		script.targetColors = targetColors
		script.targetReason = targetReason
		script.mu.Unlock()
		script.appendLog(msg)

		if matchRes.WonKnown && matchRes.Won {
			script.mu.Lock()
			script.sessionWins++
			wins := script.sessionWins
			script.mu.Unlock()
			logFn(fmt.Sprintf("本会话胜场: %d / %d", wins, sessionWinGoal))
			if wins >= sessionWinGoal {
				if shouldStop() {
					break
				}
				logFn(fmt.Sprintf("本轮结束（%s）— 等待 15s 结算动画…", matchRes.Message))
				if !sleepUntilStop(15*time.Second, shouldStop) {
					break
				}
				tryClickSkip(logFn, preview)
				cx, cy := lastWidth/2, lastHeight/2
				if cx <= 0 {
					cx = 640
				}
				if cy <= 0 {
					cy = 360
				}
				logFn(fmt.Sprintf("结算界面：点击屏幕中心 @(%d,%d)", cx, cy))
				if err := input.ClickClient(lastHWND, cx, cy); err != nil {
					logFn("结算点击失败: " + err.Error())
				}
				logFn(fmt.Sprintf("已达到 %d 胜，停止脚本", sessionWinGoal))
				lastOK, lastMsg = true, fmt.Sprintf("已达到 %d 胜，脚本停止", sessionWinGoal)
				if appStore.ShutdownAfterWins() {
					if poweroff.Schedule(poweroff.DefaultDelaySec, "MTGA Farm Bot: 已达到15胜") {
						mins := poweroff.DefaultDelaySec / 60
						logFn(fmt.Sprintf("已安排 %d 分钟后关机。取消请在终端执行：%s", mins, poweroff.CancelHint()))
						lastMsg = fmt.Sprintf("已达到 %d 胜，电脑将在 %d 分钟后关机", sessionWinGoal, mins)
					} else {
						logFn("无法安排自动关机，请自行关闭电脑")
					}
				}
				break
			}
		} else if matchRes.WonKnown {
			script.mu.Lock()
			script.sessionLosses++
			losses := script.sessionLosses
			script.mu.Unlock()
			logFn(fmt.Sprintf("本局失败，本会话负场: %d", losses))
		} else if matchRes.OK {
			logFn("本局胜负未知，不计入胜场")
		}

		if shouldStop() {
			break
		}
		// 结算界面：等动画 → 点屏幕中心关闭 → 再等一会 → 下一轮导航
		logFn("本轮结束（" + matchRes.Message + "）— 等待 15s 结算动画…")
		if !sleepUntilStop(15*time.Second, shouldStop) {
			break
		}
		tryClickSkip(logFn, preview)
		cx, cy := lastWidth/2, lastHeight/2
		if cx <= 0 {
			cx = 640
		}
		if cy <= 0 {
			cy = 360
		}
		logFn(fmt.Sprintf("结算界面：点击屏幕中心 @(%d,%d)", cx, cy))
		if err := input.ClickClient(lastHWND, cx, cy); err != nil {
			logFn("结算点击失败: " + err.Error())
		}
		logFn("结算界面：再等待 15s 后开始下一轮…")
		if !sleepUntilStop(15*time.Second, shouldStop) {
			break
		}
		if mode == appstate.ModeStarter {
			quests = refreshStarterQuests(quests, logFn, shouldStop, preview)
			if shouldStop() {
				break
			}
		}
	}

	endMsg := lastMsg
	if endMsg == "" {
		endMsg = "已停止"
	}
	script.mu.Lock()
	script.running = false
	script.lastOK = lastOK
	script.lastMessage = endMsg
	script.targetColors = lastTargetColors
	script.targetReason = lastTargetReason
	script.steps = append([]string(nil), lastSteps...)
	script.mu.Unlock()
	script.appendLog("脚本循环结束: " + endMsg)
	log.Printf("脚本循环结束 ok=%v msg=%s rounds=%d", lastOK, endMsg, round)
}

// sleepUntilStop 可被停止打断的休眠；返回 false 表示应退出循环。
func sleepUntilStop(d time.Duration, shouldStop func() bool) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if shouldStop() {
			return false
		}
		left := time.Until(deadline)
		if left > 100*time.Millisecond {
			left = 100 * time.Millisecond
		}
		time.Sleep(left)
	}
	return !shouldStop()
}

func tryClickSkip(logFn func(string), preview func(string, *image.RGBA)) {
	_ = input.ParkCursor(lastHWND)
	img, err := vision.CaptureClient(lastHWND)
	if err != nil {
		logFn("结算界面：Skip 截图失败，继续")
		return
	}
	if preview != nil {
		preview("assert/Skip.png", img)
	}
	path := filepath.Join(assets.RootDir(), "assert", "Skip.png")
	m, err := vision.FindTemplate(img, path, vision.FullROI(), 0.80)
	if err != nil || m == nil {
		logFn("结算界面：未发现 Skip，继续")
		return
	}
	logFn(fmt.Sprintf("结算界面：点击 Skip @(%d,%d) score=%.3f", m.X, m.Y, m.Score))
	if err := input.ClickClient(lastHWND, m.X, m.Y); err != nil {
		logFn("点击 Skip 失败: " + err.Error())
	}
}

func setScriptQuests(qs []playerlog.Quest) {
	script.mu.Lock()
	script.quests = append([]playerlog.Quest(nil), qs...)
	script.mu.Unlock()
}

func questsFingerprint(qs []playerlog.Quest) string {
	out := ""
	for _, q := range qs {
		out += fmt.Sprintf("%s:%d/%d;", q.ID, q.Progress, q.Goal)
	}
	return out
}

func logQuestProgress(logFn func(string), qs []playerlog.Quest) {
	if len(qs) == 0 {
		logFn("每日任务：暂无数据")
		return
	}
	logFn(fmt.Sprintf("每日任务进度（%d 项）：", len(qs)))
	for _, q := range qs {
		colors := q.Colors
		if colors == "" {
			colors = "?"
		}
		name := q.Name
		if name == "" {
			name = "-"
		}
		logFn(fmt.Sprintf("  [%s] %s %d/%d (+%d)", colors, name, q.Progress, q.Goal, q.RewardGold))
	}
}

func refreshStarterQuests(prev []playerlog.Quest, logFn func(string), shouldStop func() bool, preview func(string, *image.RGBA)) []playerlog.Quest {
	logFn("任务刷新: 切到 Home 以重新获取任务进度")
	var floor int64
	if path, err := playerlog.ResolvePath(); err == nil {
		r := &playerlog.Reader{Path: path}
		floor, _ = r.Size()
	}
	reached := starter.DipHomeForQuests(starter.Session{
		HWND: lastHWND, Width: lastWidth, Height: lastHeight,
	}, starter.Hooks{Log: logFn, Preview: preview, ShouldStop: shouldStop})
	if shouldStop() {
		return prev
	}

	prevFP := questsFingerprint(prev)
	latest := prev
	gotFresh := false
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if shouldStop() {
			break
		}
		snap := capturePlayerLogSnapshotAt(floor)
		if len(snap.Quests) > 0 {
			latest = snap.Quests
			gotFresh = true
			if questsFingerprint(latest) != prevFP {
				break
			}
			break
		}
		if !sleepUntilStop(time.Second, shouldStop) {
			break
		}
	}
	if !gotFresh {
		snap := capturePlayerLogSnapshot()
		if len(snap.Quests) > 0 {
			latest = snap.Quests
		}
		if !reached {
			logFn("任务刷新: 未确认 Home 且无新任务数据，沿用上次进度")
		} else {
			logFn("任务刷新: Home 已到达，但未读到新的 QuestGetQuests，沿用日志中的最近任务")
		}
	}
	lastQuests = latest
	setScriptQuests(latest)
	logQuestProgress(logFn, latest)
	return latest
}

func handleScriptStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ok, msg := stopScript()
	writeJSON(w, http.StatusOK, map[string]any{"ok": ok, "message": msg})
}

func handleScriptStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	since := 0
	if v := r.URL.Query().Get("since"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			since = n
		}
	}
	writeJSON(w, http.StatusOK, script.snapshotStatus(since))
}

func handleScriptPreviewTemplate(w http.ResponseWriter, r *http.Request) {
	script.mu.Lock()
	b := append([]byte(nil), script.templatePNG...)
	rel := script.templateRel
	script.mu.Unlock()
	if len(b) == 0 {
		http.Error(w, "no template", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Template-Rel", rel)
	_, _ = w.Write(b)
}

func handleScriptPreviewFrame(w http.ResponseWriter, r *http.Request) {
	script.mu.Lock()
	b := append([]byte(nil), script.framePNG...)
	script.mu.Unlock()
	if len(b) == 0 {
		http.Error(w, "no frame", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(b)
}
