package gamestate

import (
	"strings"
	"sync"
	"time"
)

const (
	gsmFull = "GameStateType_Full"
	gsmDiff = "GameStateType_Diff"
)

// Store 线程安全地合并 GRE 局面，供决策器取 Snapshot。
type Store struct {
	mu sync.RWMutex

	matchID         string
	systemSeatID    int
	pendingMsgCount int
	turn            TurnInfo
	zones           map[int]wireZone
	objects         map[int]Object
	players         map[int]wirePlayer
	actions         []Action
	suppressed      map[int]struct{}
	hasGSM          bool
	selectTarget    bool // GREMessageType_SelectTargetsReq 待处理
	selectTargetP   *SelectTargetPrompt
	selectN         *SelectNPrompt
	attackPW        bool // 宣告攻击时对方有鹏洛克，需给攻击者指定伤害接受者
	attackerIDs     []int

	latestGREStateID int
	payCostsAt       time.Time
	selectTargetAt   time.Time
	lastCTOAt        time.Time
	ctoUntil         time.Time
	ctoClickAt       time.Time
	ctoTurnKey       TurnInfo
	ctoStateID       int
	ctoJob           *CastingTimeJob
	lastGroupAt      time.Time
	groupUntil       time.Time
	groupJob         *GroupReqJob
	lastAssignAt     time.Time
	assignUntil      time.Time
	assignJob        *AssignDamageJob
	lastModalAt      time.Time
	modalUntil       time.Time
	modalJob         *ModalChoiceJob
	matchWon         *bool
}

// SelectNPrompt 弃牌 / 选 N 张等。
type SelectNPrompt struct {
	IDs    []int
	MinSel int
	MaxSel int
}

// NewStore 创建空局面仓。
func NewStore() *Store {
	return &Store{
		zones:      map[int]wireZone{},
		objects:    map[int]Object{},
		players:    map[int]wirePlayer{},
		suppressed: map[int]struct{}{},
	}
}

// Reset 清空（换局 / MatchCompleted）。
func (s *Store) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.matchID = ""
	s.systemSeatID = 0
	s.pendingMsgCount = 0
	s.turn = TurnInfo{}
	s.zones = map[int]wireZone{}
	s.objects = map[int]Object{}
	s.players = map[int]wirePlayer{}
	s.actions = nil
	s.suppressed = map[int]struct{}{}
	s.hasGSM = false
	s.selectTarget = false
	s.selectTargetP = nil
	s.selectN = nil
	s.attackPW = false
	s.attackerIDs = nil
	s.latestGREStateID = 0
	s.payCostsAt = time.Time{}
	s.selectTargetAt = time.Time{}
	s.lastCTOAt = time.Time{}
	s.ctoUntil = time.Time{}
	s.ctoClickAt = time.Time{}
	s.ctoTurnKey = TurnInfo{}
	s.ctoStateID = 0
	s.ctoJob = nil
	s.lastGroupAt = time.Time{}
	s.groupUntil = time.Time{}
	s.groupJob = nil
	s.lastAssignAt = time.Time{}
	s.assignUntil = time.Time{}
	s.assignJob = nil
	s.lastModalAt = time.Time{}
	s.modalUntil = time.Time{}
	s.modalJob = nil
	s.matchWon = nil
}

// Snapshot 返回只读副本。
func (s *Store) Snapshot(hasMulledKeep bool) Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	objs := make([]Object, 0, len(s.objects))
	for _, o := range s.objects {
		objs = append(objs, o)
	}
	acts := append([]Action(nil), s.actions...)
	complete := s.hasGSM && s.systemSeatID > 0 && s.turn.DecisionPlayer > 0

	var selN *SelectNPrompt
	if s.selectN != nil {
		cp := *s.selectN
		cp.IDs = append([]int(nil), s.selectN.IDs...)
		selN = &cp
	}
	var selT *SelectTargetPrompt
	if s.selectTargetP != nil {
		cp := *s.selectTargetP
		cp.OppCreatures = append([]int(nil), s.selectTargetP.OppCreatures...)
		cp.OwnCreatures = append([]int(nil), s.selectTargetP.OwnCreatures...)
		cp.ChooserIDs = append([]int(nil), s.selectTargetP.ChooserIDs...)
		cp.StackIDs = append([]int(nil), s.selectTargetP.StackIDs...)
		selT = &cp
	}
	return Snapshot{
		MatchID:                s.matchID,
		SystemSeatID:           s.systemSeatID,
		Turn:                   s.turn,
		Objects:                objs,
		Actions:                acts,
		PendingMsgCount:        s.pendingMsgCount,
		HasMulledKeep:          hasMulledKeep,
		GameStateComplete:      complete,
		NeedsTargetSelect:      s.selectTarget || actionsNeedSpellTarget(acts),
		NeedsCastingTimeOption: s.ctoArmed(),
		NeedsGroupReq:          s.groupArmed(),
		NeedsAssignDamage:      s.assignArmed(),
		NeedsModalChoice:       s.modalArmed(),
		SelectTarget:           selT,
		SelectN:                selN,
		AttackTargetRequired:   s.attackPW || (s.turn.Step == StepDeclareAttack && actionsNeedAttackTarget(acts)),
		AttackerIDs:            append([]int(nil), s.attackerIDs...),
	}
}

// ApplyEnvelope 消化一条 greToClientEvent。
func (s *Store) ApplyEnvelope(env *greEnvelope) (changed bool) {
	if env == nil || env.GreToClientEvent == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.noteEnvelopeStateIDs(env)

	for _, msg := range env.GreToClientEvent.GreToClientMessages {
		if seat := singletonSeat(msg.SystemSeatIDs); seat > 0 {
			// 座位只在尚未确定或 Mulligan 时写入，避免被其它提示消息改写
			if s.systemSeatID == 0 || msg.Type == "GREMessageType_MulliganReq" {
				s.systemSeatID = seat
			}
		}
		switch msg.Type {
		case "GREMessageType_GameStateMessage":
			if msg.GameStateMessage != nil {
				s.applyGSM(msg.GameStateMessage)
				changed = true
			}
		case "GREMessageType_ActionsAvailableReq":
			if msg.ActionsAvailableReq != nil {
				s.applyActions(msg.SystemSeatIDs, msg.ActionsAvailableReq.Actions)
				changed = true
			}
		case "GREMessageType_MulliganReq":
			if seat := singletonSeat(msg.SystemSeatIDs); seat > 0 {
				s.systemSeatID = seat
				changed = true
			}
		case "GREMessageType_SelectTargetsReq":
			seat := singletonSeat(msg.SystemSeatIDs)
			if seat == 0 {
				seat = s.systemSeatID
			}
			if s.systemSeatID > 0 && seat != s.systemSeatID {
				continue
			}
			s.selectTarget = true
			s.selectTargetP = s.classifySelectTargets(msg.SelectTargetsReq)
			s.selectTargetAt = time.Now()
			if s.systemSeatID > 0 {
				s.turn.DecisionPlayer = s.systemSeatID
			}
			changed = true
		case "GREMessageType_PayCostsReq":
			s.payCostsAt = time.Now()
			changed = true
		case "GREMessageType_CastingTimeOptionsReq":
			if s.applyCastingTimeOptions(msg) {
				changed = true
			}
		case "GREMessageType_GroupReq":
			if s.applyGroupReq(msg) {
				changed = true
			}
		case "GREMessageType_AssignDamageReq":
			if s.applyAssignDamageReq(msg) {
				changed = true
			}
		case "GREMessageType_DeclareAttackersReq":
			if s.applyDeclareAttackersReq(msg) {
				changed = true
			}
		case "GREMessageType_SelectNReq":
			if msg.SelectNReq == nil || len(msg.SelectNReq.IDs) == 0 {
				continue
			}
			seat := singletonSeat(msg.SystemSeatIDs)
			if seat == 0 {
				seat = s.systemSeatID
			}
			if s.systemSeatID > 0 && seat != s.systemSeatID {
				continue
			}
			// 结算异能 Choose One：PromptParameterIndex，不走手牌弃牌路径
			if msg.SelectNReq.IDType == "IdType_PromptParameterIndex" {
				if s.applyModalSelectN(msg) {
					changed = true
				}
				continue
			}
			minSel := msg.SelectNReq.MinSel
			if minSel < 1 {
				minSel = 1
			}
			maxSel := msg.SelectNReq.MaxSel
			if maxSel < minSel {
				maxSel = minSel
			}
			s.selectN = &SelectNPrompt{
				IDs:    append([]int(nil), msg.SelectNReq.IDs...),
				MinSel: minSel,
				MaxSel: maxSel,
			}
			if s.systemSeatID > 0 {
				s.turn.DecisionPlayer = s.systemSeatID
			}
			changed = true
		}
	}
	return changed
}

func singletonSeat(ids []int) int {
	if len(ids) == 1 && ids[0] > 0 {
		return ids[0]
	}
	return 0
}

func (s *Store) applyGSM(gsm *wireGSM) {
	if gsm == nil {
		return
	}
	s.hasGSM = true

	if gsm.GameInfo != nil && gsm.GameInfo.MatchID != "" {
		if s.matchID != "" && s.matchID != gsm.GameInfo.MatchID {
			s.zones = map[int]wireZone{}
			s.objects = map[int]Object{}
			s.players = map[int]wirePlayer{}
			s.actions = nil
			s.suppressed = map[int]struct{}{}
			s.turn = TurnInfo{}
			s.attackPW = false
			s.attackerIDs = nil
		}
		s.matchID = gsm.GameInfo.MatchID
	}

	prevDecision := s.turn.DecisionPlayer
	prevTurn := s.turn.TurnNumber
	if gsm.Type == gsmFull {
		s.replaceFull(gsm)
	} else {
		s.mergeDiff(gsm)
	}
	if s.turn.TurnNumber != prevTurn && s.turn.TurnNumber != 0 {
		s.suppressed = map[int]struct{}{}
	}
	// 优先权离开本方时清空可用动作，避免对手 AAR 粘在摘要里
	if s.systemSeatID > 0 && s.turn.DecisionPlayer != s.systemSeatID && prevDecision != s.turn.DecisionPlayer {
		s.actions = nil
	}
	s.maybeClearAssignAfterStep()
	s.maybeClearAttackTarget()
	s.noteMatchResultLocked(gsm)
}

func (s *Store) replaceFull(gsm *wireGSM) {
	s.pendingMsgCount = gsm.PendingMessageCount
	if gsm.TurnInfo != nil {
		s.turn = turnFromWire(gsm.TurnInfo)
	}
	s.zones = map[int]wireZone{}
	for _, z := range gsm.Zones {
		s.zones[z.ZoneID] = z
	}
	s.objects = map[int]Object{}
	for _, o := range gsm.GameObjects {
		s.objects[o.InstanceID] = objectFromWire(o)
	}
	s.players = map[int]wirePlayer{}
	for _, p := range gsm.Players {
		s.players[p.SystemSeatNumber] = p
	}
	for _, id := range gsm.DiffDeletedInstanceIDs {
		delete(s.objects, id)
	}
}

func (s *Store) mergeDiff(gsm *wireGSM) {
	s.pendingMsgCount = gsm.PendingMessageCount
	if gsm.TurnInfo != nil {
		s.turn = mergeTurn(s.turn, gsm.TurnInfo)
	}
	for _, z := range gsm.Zones {
		prev := s.zones[z.ZoneID]
		s.zones[z.ZoneID] = mergeZone(prev, z)
	}
	for _, o := range gsm.GameObjects {
		prev := s.objects[o.InstanceID]
		s.objects[o.InstanceID] = mergeObject(prev, o)
	}
	for _, id := range gsm.DiffDeletedInstanceIDs {
		delete(s.objects, id)
	}
	for _, p := range gsm.Players {
		prev := s.players[p.SystemSeatNumber]
		if p.TeamID == 0 {
			p.TeamID = prev.TeamID
		}
		s.players[p.SystemSeatNumber] = p
	}
}

func (s *Store) applyActions(seatIDs []int, wire []wireAction) {
	seat := singletonSeat(seatIDs)
	if seat == 0 {
		seat = s.systemSeatID
	}
	// 只保留本方座位的可用动作
	if s.systemSeatID > 0 && seat != s.systemSeatID {
		return
	}
	if seat > 0 {
		// AAR 到达即表示本方需要决策（弥补 Diff 省略 decisionPlayer）
		s.turn.DecisionPlayer = seat
		if s.turn.PriorityPlayer == 0 {
			s.turn.PriorityPlayer = seat
		}
	}
	out := make([]Action, 0, len(wire))
	for _, a := range wire {
		if _, skip := s.suppressed[a.InstanceID]; skip {
			continue
		}
		out = append(out, Action{
			SeatID:       seat,
			ActionType:   a.ActionType,
			InstanceID:   a.InstanceID,
			GrpID:        a.GrpID,
			AbilityGrpID: a.AbilityGrpID,
			ManaCost:     manaPipsFromWire(a.ManaCost),
		})
	}
	s.actions = out
}

// ClearTargetSelect 选目标点击后清除挂起标记。
func (s *Store) ClearTargetSelect() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.selectTarget = false
	s.selectTargetP = nil
}

// classifySelectTargets 把 SelectTargetsReq 合法目标分成己方/对方生物与打脸。
func (s *Store) classifySelectTargets(req *selectTargetsReq) *SelectTargetPrompt {
	if req == nil {
		return &SelectTargetPrompt{}
	}
	out := &SelectTargetPrompt{SourceID: req.SourceID}
	playerSeats := map[int]struct{}{}
	for seat := range s.players {
		if seat > 0 {
			playerSeats[seat] = struct{}{}
		}
	}
	seenOwn := map[int]struct{}{}
	seenOpp := map[int]struct{}{}
	seenChooser := map[int]struct{}{}
	seenStack := map[int]struct{}{}
	for _, group := range req.Targets {
		for _, tgt := range group.Targets {
			if tgt.LegalAction != "" && tgt.LegalAction != "SelectAction_Select" {
				continue
			}
			tid := tgt.TargetInstanceID
			if tid <= 0 {
				continue
			}
			obj, ok := s.objects[tid]
			if ok {
				if zt := s.zoneTypeOf(obj.ZoneID); zt != "" && zt != ZoneBattlefield {
					if zt == ZoneStack {
						if _, dup := seenStack[tid]; !dup {
							seenStack[tid] = struct{}{}
							out.StackIDs = append(out.StackIDs, tid)
						}
					} else if _, dup := seenChooser[tid]; !dup {
						seenChooser[tid] = struct{}{}
						out.ChooserIDs = append(out.ChooserIDs, tid)
					}
					continue
				}
			}
			if ok && isCreatureObject(obj) && s.objectOnBattlefield(obj) {
				if obj.ControllerSeatID == s.systemSeatID {
					if _, dup := seenOwn[tid]; !dup {
						seenOwn[tid] = struct{}{}
						out.OwnCreatures = append(out.OwnCreatures, tid)
					}
				} else if _, dup := seenOpp[tid]; !dup {
					seenOpp[tid] = struct{}{}
					out.OppCreatures = append(out.OppCreatures, tid)
				}
				continue
			}
			if _, isPlayer := playerSeats[tid]; isPlayer {
				if tid != s.systemSeatID {
					out.FaceLegal = true
				}
				continue
			}
			// 物件表里没有：可能是玩家座位号，或墓地/堆叠目标；保守标 face 可选
			if !ok && tid != s.systemSeatID {
				out.FaceLegal = true
			}
		}
	}
	return out
}

func (s *Store) zoneTypeOf(zoneID int) string {
	if z, ok := s.zones[zoneID]; ok {
		return z.Type
	}
	return ""
}

func isCreatureObject(o Object) bool {
	for _, t := range o.CardTypes {
		if t == "CardType_Creature" || strings.EqualFold(t, "Creature") {
			return true
		}
	}
	return false
}

func (s *Store) objectOnBattlefield(o Object) bool {
	if z, ok := s.zones[o.ZoneID]; ok {
		return z.Type == ZoneBattlefield
	}
	// Diff 丢 zone 时：若某战场 zone 的 objectInstanceIds 含它也算
	for _, z := range s.zones {
		if z.Type != ZoneBattlefield {
			continue
		}
		for _, id := range z.ObjectInstanceIDs {
			if id == o.InstanceID {
				return true
			}
		}
	}
	// 无 zone 信息时仍允许按生物类型选（避免漏点）
	return o.ZoneID == 0
}

// ClearSelectN 选 N / 弃牌完成后清除。
func (s *Store) ClearSelectN() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.selectN = nil
}

// DropSelectNID 某张候选扫描失败后从列表拿掉，避免死循环。
func (s *Store) DropSelectNID(id int) {
	if id <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.selectN == nil {
		return
	}
	kept := s.selectN.IDs[:0]
	for _, x := range s.selectN.IDs {
		if x != id {
			kept = append(kept, x)
		}
	}
	s.selectN.IDs = kept
	if len(s.selectN.IDs) == 0 {
		s.selectN = nil
	}
}

func actionsNeedSpellTarget(acts []Action) bool {
	for _, a := range acts {
		t := a.ActionType
		if t == "" {
			continue
		}
		if strings.Contains(t, "Target") && !strings.Contains(t, "Attack") && !strings.Contains(t, "Combat") {
			return true
		}
	}
	return false
}

// SuppressAction 暂时从可用列表去掉某 instance（双击未改变局面时防死循环）。
func (s *Store) SuppressAction(instanceID int) {
	if instanceID <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.suppressed == nil {
		s.suppressed = map[int]struct{}{}
	}
	s.suppressed[instanceID] = struct{}{}
	filtered := s.actions[:0]
	for _, a := range s.actions {
		if a.InstanceID != instanceID {
			filtered = append(filtered, a)
		}
	}
	s.actions = filtered
}

func turnFromWire(t *wireTurnInfo) TurnInfo {
	if t == nil {
		return TurnInfo{}
	}
	return TurnInfo{
		TurnNumber:     t.TurnNumber,
		Phase:          t.Phase,
		Step:           t.Step,
		ActivePlayer:   t.ActivePlayer,
		PriorityPlayer: t.PriorityPlayer,
		DecisionPlayer: t.DecisionPlayer,
	}
}

func mergeTurn(prev TurnInfo, t *wireTurnInfo) TurnInfo {
	out := prev
	turnChanged := t.TurnNumber != 0 && t.TurnNumber != prev.TurnNumber
	phaseChanged := t.Phase != "" && t.Phase != prev.Phase

	if t.Phase != "" {
		out.Phase = t.Phase
	}
	if t.Step != "" {
		out.Step = t.Step
	} else if phaseChanged || turnChanged {
		// Diff 换回合/阶段时常省略 step，避免残留 Step_Draw 挂在 Phase_Main1 上
		out.Step = ""
	}
	if t.TurnNumber != 0 {
		out.TurnNumber = t.TurnNumber
	}
	if t.ActivePlayer != 0 {
		out.ActivePlayer = t.ActivePlayer
	}
	if t.PriorityPlayer != 0 {
		out.PriorityPlayer = t.PriorityPlayer
	}
	if t.DecisionPlayer != 0 {
		out.DecisionPlayer = t.DecisionPlayer
	} else if turnChanged {
		// Diff 在换回合时常省略 decisionPlayer；沿用上回合座位会卡死「非本方决策」
		if t.PriorityPlayer != 0 {
			out.DecisionPlayer = t.PriorityPlayer
		} else if t.ActivePlayer != 0 {
			out.DecisionPlayer = t.ActivePlayer
		} else {
			out.DecisionPlayer = 0
		}
	}
	return out
}

func mergeZone(prev, upd wireZone) wireZone {
	out := prev
	if upd.ZoneID != 0 {
		out.ZoneID = upd.ZoneID
	}
	if upd.Type != "" {
		out.Type = upd.Type
	}
	if upd.OwnerSeatID != 0 {
		out.OwnerSeatID = upd.OwnerSeatID
	}
	// objectInstanceIds：Diff 里若带了就替换；未带则保留旧值
	if upd.ObjectInstanceIDs != nil {
		out.ObjectInstanceIDs = append([]int(nil), upd.ObjectInstanceIDs...)
	}
	return out
}

func objectFromWire(o wireObject) Object {
	out := Object{
		InstanceID:        o.InstanceID,
		GrpID:             o.GrpID,
		ObjectSourceGrpID: o.ObjectSourceGrpID,
		ZoneID:            o.ZoneID,
		ControllerSeatID:  o.ControllerSeatID,
		CardTypes:         append([]string(nil), o.CardTypes...),
		AttackState:       o.AttackState,
	}
	if out.ControllerSeatID == 0 {
		out.ControllerSeatID = o.OwnerSeatID
	}
	if o.Power != nil {
		out.Power = *o.Power
	}
	if o.Toughness != nil {
		out.Toughness = *o.Toughness
	}
	return out
}

func mergeObject(prev Object, upd wireObject) Object {
	out := prev
	if upd.InstanceID != 0 {
		out.InstanceID = upd.InstanceID
	}
	if upd.GrpID != 0 {
		out.GrpID = upd.GrpID
	}
	if upd.ObjectSourceGrpID != 0 {
		out.ObjectSourceGrpID = upd.ObjectSourceGrpID
	}
	if upd.ZoneID != 0 {
		out.ZoneID = upd.ZoneID
	}
	seat := upd.ControllerSeatID
	if seat == 0 {
		seat = upd.OwnerSeatID
	}
	if seat != 0 {
		out.ControllerSeatID = seat
	}
	if len(upd.CardTypes) > 0 {
		out.CardTypes = append([]string(nil), upd.CardTypes...)
	}
	if upd.AttackState != "" {
		out.AttackState = upd.AttackState
	}
	if upd.Power != nil {
		out.Power = *upd.Power
	}
	if upd.Toughness != nil {
		out.Toughness = *upd.Toughness
	}
	return out
}

// PlayOrCastActions 过滤可出牌/施放动作。
func (s Snapshot) PlayOrCastActions() []Action {
	out := make([]Action, 0)
	for _, a := range s.Actions {
		if a.ActionType == ActionPlay || a.ActionType == ActionCast {
			out = append(out, a)
		}
	}
	return out
}

func manaPipsFromWire(in []wireManaCost) []ManaPip {
	if len(in) == 0 {
		return nil
	}
	out := make([]ManaPip, 0, len(in))
	for _, p := range in {
		out = append(out, ManaPip{
			Colors: append([]string(nil), p.Color...),
			Count:  p.Count,
		})
	}
	return out
}

// Summary 一行调试摘要。
func (s Snapshot) Summary() string {
	plays, casts := 0, 0
	for _, a := range s.Actions {
		switch a.ActionType {
		case ActionPlay:
			plays++
		case ActionCast:
			casts++
		}
	}
	return formatSummary(s.MatchID, s.SystemSeatID, s.Turn, s.IsOurDecision(), plays, casts, len(s.Objects), s.PendingMsgCount)
}
