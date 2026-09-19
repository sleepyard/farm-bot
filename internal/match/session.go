// Package match 把「进入匹配」之后的对局循环粘合起来：
// Player.log → 局面标记 → 决策器 → 控制器。
package match

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/flourbrain/mtga-farm-bot/internal/controller"
	"github.com/flourbrain/mtga-farm-bot/internal/decisioner"
	"github.com/flourbrain/mtga-farm-bot/internal/gamestate"
	"github.com/flourbrain/mtga-farm-bot/internal/playerlog"
)

// Hooks 日志与停止。
type Hooks struct {
	Log        func(string)
	ShouldStop func() bool
}

// Session 一次对局会话（从排队成功到 MatchCompleted）。
type Session struct {
	HWND    uintptr
	LogPath string
	Mode    string // historic | starter；传给决策器
	Hooks   Hooks

	ctrl  *controller.Controller
	ai    *decisioner.Engine
	store *gamestate.Store
	ctx   context.Context

	mu            sync.Mutex
	hasMulledKeep bool
	inGame        bool
	matchDone     bool
	keepPolling   bool
	sawMulligan   bool
	lastSummary   string
	decideRunning bool
	matchStart    time.Time
	concedeOnce   bool
	won           bool
	wonKnown      bool
}

// Result 对局循环结果。
type Result struct {
	OK       bool
	Message  string
	Won      bool
	WonKnown bool
}

func (s *Session) log(msg string) {
	if s.Hooks.Log != nil {
		s.Hooks.Log(msg)
	}
}

func (s *Session) logf(format string, args ...any) {
	s.log(fmt.Sprintf(format, args...))
}

func (s *Session) stopped() bool {
	return s.Hooks.ShouldStop != nil && s.Hooks.ShouldStop()
}

func (s *Session) isDone() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.matchDone
}

func (s *Session) kept() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hasMulledKeep
}

// Run 在 starter 已点击 Play 之后调用。
func Run(ctx context.Context, sess *Session) Result {
	if sess == nil {
		return Result{OK: false, Message: "session 为空"}
	}
	if sess.HWND == 0 {
		return Result{OK: false, Message: "无 MTGA 窗口"}
	}
	if sess.LogPath == "" {
		p, err := playerlog.ResolvePath()
		if err != nil {
			return Result{OK: false, Message: "找不到 Player.log: " + err.Error()}
		}
		sess.LogPath = p
	}
	sess.ctrl = &controller.Controller{
		HWND:    sess.HWND,
		LogPath: sess.LogPath,
		Log:     sess.Hooks.Log,
	}
	sess.ai = decisioner.NewWithMode(sess.Mode)
	sess.store = gamestate.NewStore()
	if sess.Mode == "historic" {
		sess.log("对局策略: 史迹模式 — 不阻挡，选目标一律对手头像")
	}

	reader := &playerlog.Reader{Path: sess.LogPath}
	startOff, err := reader.Size()
	if err != nil {
		return Result{OK: false, Message: "读取 Player.log 失败: " + err.Error()}
	}

	sess.log("==========对局：等待匹配 / GameState==========")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	sess.ctx = ctx

	_, followErr := reader.Follow(ctx, startOff, 350*time.Millisecond, playerlog.FollowHooks{
		Stop: sess.stopped,
		OnChunk: func(chunk string) {
			sess.onLogChunk(chunk, cancel)
		},
	})
	if sess.stopped() {
		return sess.finishResult(false, "已停止")
	}
	if sess.isDone() {
		return sess.finishResult(true, "对局结束")
	}
	if followErr != nil && ctx.Err() == nil {
		return sess.finishResult(false, followErr.Error())
	}
	if sess.isDone() {
		return sess.finishResult(true, "对局结束")
	}
	return sess.finishResult(false, "对局跟随结束")
}

func (s *Session) finishResult(ok bool, base string) Result {
	r := Result{OK: ok, Message: base, Won: s.won, WonKnown: s.wonKnown}
	if s.wonKnown {
		if s.won {
			r.Message = base + "（胜利）"
		} else {
			r.Message = base + "（失败）"
		}
	}
	return r
}

func (s *Session) onLogChunk(chunk string, cancel context.CancelFunc) {
	if gamestate.IngestChunk(s.store, chunk) {
		snap := s.store.Snapshot(s.kept())
		sum := snap.Summary()
		s.mu.Lock()
		prev := s.lastSummary
		s.lastSummary = sum
		s.mu.Unlock()
		if sum != prev {
			s.log("局面: " + sum)
		}
	}
	if job, ok := s.store.TakeCastingTimeJob(); ok {
		go s.answerCastingTime(job)
	}
	if job, ok := s.store.TakeGroupReqJob(); ok {
		go s.answerGroupReq(job)
	}
	if job, ok := s.store.TakeAssignDamageJob(); ok {
		go s.answerAssignDamage(job)
	}
	if job, ok := s.store.TakeModalChoiceJob(); ok {
		go s.answerModalChoice(job)
	}

	m := playerlog.ScanMatchMarkers(chunk)

	s.mu.Lock()
	if m.SawGameStateMessage && !s.inGame {
		s.inGame = true
		s.matchStart = time.Now()
		s.mu.Unlock()
		s.log("对局：已进入 IN_GAME（检测到 GameStateMessage）")
		go s.watchConcedeLimits()
		s.mu.Lock()
	}
	if m.SawMulliganReq && !s.sawMulligan {
		s.sawMulligan = true
		startPoll := !s.keepPolling && !s.hasMulledKeep
		if startPoll {
			s.keepPolling = true
		}
		s.mu.Unlock()
		s.log("对局：收到 MulliganReq，开始搜寻 keep_hand.png")
		if startPoll {
			go s.pollKeepHand()
		}
		s.mu.Lock()
	}
	if m.SawMatchCompleted {
		s.recordMatchOutcome(chunk)
		s.matchDone = true
		s.mu.Unlock()
		s.store.Reset()
		if s.wonKnown {
			if s.won {
				s.log("对局：MatchCompleted（胜利）")
			} else {
				s.log("对局：MatchCompleted（失败）")
			}
		} else {
			s.log("对局：MatchCompleted")
		}
		cancel()
		return
	}
	s.mu.Unlock()
}

func (s *Session) recordMatchOutcome(chunk string) {
	if won, ok := s.store.MatchOutcome(); ok {
		s.won, s.wonKnown = won, true
		return
	}
	seat, team := s.store.SeatAndTeam()
	if won, ok := playerlog.InferMatchWon(chunk, seat, team); ok {
		s.won, s.wonKnown = won, true
	}
}

func (s *Session) pollKeepHand() {
	deadline := time.Now().Add(120 * time.Second)
	attempts := 0
	for time.Now().Before(deadline) {
		if s.stopped() || s.isDone() || s.kept() {
			return
		}
		attempts++
		pt, err := s.ctrl.FindKeepHand()
		if err != nil {
			if attempts == 1 || attempts%5 == 0 {
				s.log("对局：尚未找到 keep_hand.png，继续等待…")
			}
			time.Sleep(700 * time.Millisecond)
			continue
		}
		if err := s.ctrl.ClickKeepHand(pt); err != nil {
			s.log("对局：点击 keep_hand 失败: " + err.Error())
			time.Sleep(700 * time.Millisecond)
			continue
		}
		s.mu.Lock()
		s.hasMulledKeep = true
		startDecide := !s.decideRunning
		if startDecide {
			s.decideRunning = true
		}
		s.mu.Unlock()
		s.log("日志确认已保留手牌")
		s.log("局面(保留后): " + s.store.Snapshot(true).Summary())
		if startDecide {
			go s.decisionLoop()
		}
		return
	}
	s.log("对局：超时未找到 keep_hand.png（未确认保留）")
}

func (s *Session) decisionLoop() {
	s.log("对局：BOT 决策循环已启动")
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	var done <-chan struct{}
	if s.ctx != nil {
		done = s.ctx.Done()
	}
	waitStreak := 0
	prevDecisionPlayer := 0 // 用于检测「对方 → 本方」决策切换
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if s.stopped() || s.isDone() {
				return
			}
			if s.tryConcedeIfNeeded() {
				continue
			}
			if !s.kept() {
				continue
			}
			if s.store.CastingTimeStillOpen() {
				waitStreak++
				if waitStreak == 1 || waitStreak%5 == 0 {
					s.log("Deferring decision; casting-time Choose One dialog still open")
				}
				continue
			}
			if s.store.GroupReqStillOpen() {
				waitStreak++
				if waitStreak == 1 || waitStreak%5 == 0 {
					s.log("Deferring decision; scry/group prompt still active")
				}
				continue
			}
			if s.store.AssignDamageStillOpen() {
				waitStreak++
				if waitStreak == 1 || waitStreak%5 == 0 {
					s.log("Deferring decision; assign damage prompt still active")
				}
				continue
			}
			if s.store.ModalChoiceStillOpen() {
				waitStreak++
				if waitStreak == 1 || waitStreak%5 == 0 {
					s.log("Deferring decision; resolution Choose One still open")
				}
				continue
			}
			snap := s.store.Snapshot(true)

			// 对方回合结束、轮到本方决策：先等动画，再开始扫手牌等操作
			dp := snap.Turn.DecisionPlayer
			seat := snap.SystemSeatID
			if seat > 0 && dp == seat && prevDecisionPlayer > 0 && prevDecisionPlayer != seat {
				s.log("轮到本方决策，等待 10s 让动画结束后再决策…")
				if !s.sleepInterruptible(10*time.Second, done) {
					return
				}
				snap = s.store.Snapshot(true) // 动画期间局面可能已变
			}
			if dp > 0 {
				prevDecisionPlayer = dp
			}

			move := s.ai.NextMove(snap)
			if move.Kind == decisioner.KindWait {
				waitStreak++
				if waitStreak == 1 || waitStreak%5 == 0 {
					s.logf("BOT等待: %s（局面 %s）", move.Reason, snap.Summary())
				}
				continue
			}
			waitStreak = 0
			if tag := move.InstanceTag(); tag != "" {
				s.logf("BOT决策: %s %s — %s", move.Kind, tag, move.Reason)
			} else {
				s.logf("BOT决策: %s — %s", move.Kind, move.Reason)
			}

			before := snap
			if err := s.ctrl.Execute(move); err != nil {
				s.logf("BOT操作失败: %s — %v", move.Kind, err)
				if errors.Is(err, controller.ErrHandScanMissed) {
					s.log("BOT操作: 已按 Esc 关闭界面，恢复决策出牌")
					continue
				}
				if move.Kind == decisioner.KindCast || move.Kind == decisioner.KindPlayLand {
					s.logf("BOT决策: %s 出牌失败，本回合不再尝试该牌", move.InstanceTag())
					s.store.SuppressAction(move.InstanceID)
				}
				if move.Kind == decisioner.KindSelectN {
					s.store.DropSelectNID(move.InstanceID)
					if s.store.Snapshot(true).NeedsSelectN() {
						s.log("BOT操作: SelectN 跳过该牌，尝试下一张")
					} else {
						s.log("BOT操作: SelectN 候选均未扫到，点击确认以免卡死")
						_ = s.ctrl.SubmitPrompt("SelectN 失败后确认")
						s.store.ClearSelectN()
					}
					time.Sleep(400 * time.Millisecond)
				}
				continue
			}
			if move.Kind == decisioner.KindSelectTarget {
				s.store.ClearTargetSelect()
				s.log("对局：选目标完成，等待 5s 让动画结束后再扫手牌…")
				if !s.sleepInterruptible(5*time.Second, done) {
					return
				}
				continue
			}
			if move.Kind == decisioner.KindSelectN {
				s.store.ClearSelectN()
				time.Sleep(500 * time.Millisecond)
				continue
			}

			if move.Kind == decisioner.KindCast || move.Kind == decisioner.KindPlayLand {
				if !s.waitBoardChange(before, move.InstanceID, 5*time.Second) {
					s.logf("BOT操作: %s 双击后该动作仍可用，本回合不再尝试", move.InstanceTag())
					s.store.SuppressAction(move.InstanceID)
					continue
				}
				// 再确认一次：物件数抖动不能算成功
				cur := s.store.Snapshot(true)
				if actionStillAvailable(cur, move.InstanceID) {
					s.logf("BOT操作: %s 局面抖动但动作仍在，本回合不再尝试", move.InstanceTag())
					s.store.SuppressAction(move.InstanceID)
					continue
				}
				s.logf("BOT操作: %s 已离开可用动作，等待 5s 让动画结束后再扫手牌", move.InstanceTag())
				if move.Kind == decisioner.KindPlayLand {
					s.ai.MarkLandPlayed()
				}
				if s.tryConcedeIfNeeded() {
					continue
				}
				if !s.sleepInterruptible(5*time.Second, done) {
					return
				}
				continue
			}
			time.Sleep(600 * time.Millisecond)
		}
	}
}

// sleepInterruptible 可被停止 / ctx 取消打断的 sleep；返回 false 表示应退出循环。
func (s *Session) sleepInterruptible(d time.Duration, done <-chan struct{}) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-done:
		return false
	case <-t.C:
		return !s.stopped() && !s.isDone()
	}
}

// waitBoardChange 等到该 instance 不再出现在可用动作里，或优先权/回合已离开。
// 不用 objs 数量：施法弹窗/堆叠抖动会导致误判「已打出」从而死循环同一张牌。
func (s *Session) waitBoardChange(before gamestate.Snapshot, instanceID int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if s.stopped() || s.isDone() {
			return false
		}
		time.Sleep(200 * time.Millisecond)
		cur := s.store.Snapshot(true)
		if !actionStillAvailable(cur, instanceID) {
			// AAR 刷新瞬间可能空一拍，短暂确认仍不在列表里
			time.Sleep(350 * time.Millisecond)
			cur = s.store.Snapshot(true)
			if !actionStillAvailable(cur, instanceID) {
				return true
			}
			continue
		}
		if cur.Turn.TurnNumber != before.Turn.TurnNumber {
			return true
		}
		if cur.Turn.DecisionPlayer != before.Turn.DecisionPlayer &&
			cur.SystemSeatID > 0 && cur.Turn.DecisionPlayer != cur.SystemSeatID {
			return true
		}
	}
	return false
}

func actionStillAvailable(snap gamestate.Snapshot, instanceID int) bool {
	for _, a := range snap.Actions {
		if a.InstanceID == instanceID {
			return true
		}
	}
	return false
}

func (s *Session) watchConcedeLimits() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var done <-chan struct{}
	if s.ctx != nil {
		done = s.ctx.Done()
	}
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if s.stopped() || s.isDone() {
				return
			}
			s.tryConcedeIfNeeded()
		}
	}
}

// tryConcedeIfNeeded 对局超过 10 分钟则投降。已投降或正在投降时返回 true，决策循环应跳过。
func (s *Session) tryConcedeIfNeeded() bool {
	s.mu.Lock()
	if s.concedeOnce {
		s.mu.Unlock()
		return true
	}
	reason := ""
	if !s.matchStart.IsZero() && time.Since(s.matchStart) >= 10*time.Minute {
		reason = "对局超过 10 分钟"
	}
	if reason == "" {
		s.mu.Unlock()
		return false
	}
	s.concedeOnce = true
	s.mu.Unlock()

	s.log("对局：触发投降 — " + reason)
	if err := s.ctrl.Concede(); err != nil {
		s.log("对局：投降失败: " + err.Error() + "，将重试")
		s.mu.Lock()
		s.concedeOnce = false
		s.mu.Unlock()
		return false
	}
	s.log("对局：已点击 concede，等待 MatchCompleted 后结算")
	return true
}
