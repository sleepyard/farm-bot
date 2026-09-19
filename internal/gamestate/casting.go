package gamestate

import (
	"fmt"
	"time"
)

// Python _MODAL_PICK_SECOND_GRPIDS：Valorous Stance 各印记，选右侧「消灭」。
// kicker 及其它模式牌默认点左边；只有名单内 grpId 点右边。
var modalPickSecondGrpIDs = map[int]struct{}{
	72198: {},
	78825: {},
	93566: {},
	94011: {},
	98299: {},
}

const (
	ctoDedupeWindow = 2 * time.Second
	ctoWaitWindow   = 12 * time.Second
)

func (s *Store) noteEnvelopeStateIDs(env *greEnvelope) {
	if env == nil || env.GreToClientEvent == nil {
		return
	}
	newest := 0
	for _, msg := range env.GreToClientEvent.GreToClientMessages {
		if msg.GameStateID > newest {
			newest = msg.GameStateID
		}
		if msg.GameStateMessage != nil && msg.GameStateMessage.GameStateID > newest {
			newest = msg.GameStateMessage.GameStateID
		}
	}
	if newest != 0 {
		s.latestGREStateID = newest
	}
}

func (s *Store) readGameStateID() int {
	if s.latestGREStateID != 0 {
		return s.latestGREStateID
	}
	return 0
}

func (s *Store) ctoOurs(seatIDs []int) bool {
	if s.systemSeatID <= 0 || len(seatIDs) == 0 {
		return true
	}
	for _, id := range seatIDs {
		if id == s.systemSeatID {
			return true
		}
	}
	return false
}

func (s *Store) ctoArmed() bool {
	return !s.ctoUntil.IsZero() && time.Now().Before(s.ctoUntil)
}

func (s *Store) applyCastingTimeOptions(msg greMessage) bool {
	now := time.Now()
	if !s.lastCTOAt.IsZero() && now.Sub(s.lastCTOAt) < ctoDedupeWindow {
		return false
	}
	if !s.ctoOurs(msg.SystemSeatIDs) {
		return false
	}

	chooseOrCost := false
	preferSecond := false
	summaries := make([]string, 0)
	if msg.CastingTimeOptionsReq != nil {
		for _, opt := range msg.CastingTimeOptionsReq.Options {
			if opt.Type == "CastingTimeOptionType_ChooseOrCost" {
				chooseOrCost = true
			}
			if opt.Type == "CastingTimeOptionType_Modal" {
				if _, ok := modalPickSecondGrpIDs[opt.GrpID]; ok {
					preferSecond = true
				}
			}
			summaries = append(summaries, fmt.Sprintf(
				"type=%s ctoId=%d grpId=%d required=%v",
				opt.Type, opt.CtoID, opt.GrpID, opt.IsRequired,
			))
		}
	}

	kind := CastingTimePlain
	desc := "plain (non-kicked) version"
	if chooseOrCost {
		kind = CastingTimeSacrifice
		desc = "sacrifice-a-creature (bottom button)"
	} else if preferSecond {
		kind = CastingTimeModalSecond
		desc = "second/right modal option (e.g. Valorous Stance destroy)"
	}

	s.lastCTOAt = now
	s.ctoClickAt = now
	s.ctoUntil = now.Add(ctoWaitWindow)
	s.ctoTurnKey = s.turn
	s.ctoStateID = s.readGameStateID()
	s.ctoJob = &CastingTimeJob{Kind: kind, Desc: desc, Options: summaries}
	return true
}

func (s *Store) clearCastingTimeLocked() {
	s.ctoUntil = time.Time{}
	s.ctoTurnKey = TurnInfo{}
	s.ctoStateID = 0
	s.ctoJob = nil
}

func (s *Store) castingTimeStillOpenLocked() bool {
	if s.ctoUntil.IsZero() {
		return false
	}
	if time.Now().After(s.ctoUntil) {
		s.clearCastingTimeLocked()
		return false
	}
	if !s.payCostsAt.IsZero() && !s.payCostsAt.Before(s.ctoClickAt) {
		s.clearCastingTimeLocked()
		return false
	}
	if !s.selectTargetAt.IsZero() && !s.selectTargetAt.Before(s.ctoClickAt) {
		s.clearCastingTimeLocked()
		return false
	}
	stateID := s.readGameStateID()
	if stateID != 0 && s.ctoStateID != 0 && stateID != s.ctoStateID {
		s.clearCastingTimeLocked()
		return false
	}
	key := s.turn
	rec := s.ctoTurnKey
	if rec != (TurnInfo{}) &&
		(key.TurnNumber != rec.TurnNumber || key.Phase != rec.Phase ||
			key.Step != rec.Step || key.DecisionPlayer != rec.DecisionPlayer) {
		s.clearCastingTimeLocked()
		return false
	}
	return true
}

// TakeCastingTimeJob 取出待点击的 Choose One（只取一次）。
func (s *Store) TakeCastingTimeJob() (CastingTimeJob, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctoJob == nil {
		return CastingTimeJob{}, false
	}
	job := *s.ctoJob
	s.ctoJob = nil
	return job, true
}

// CastingTimeStillOpen 对话框是否仍挡住点击；过期 / 局面推进时清掉暂停。
func (s *Store) CastingTimeStillOpen() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.castingTimeStillOpenLocked()
}

// ClearCastingTime 结束暂停窗口。
func (s *Store) ClearCastingTime() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clearCastingTimeLocked()
}
