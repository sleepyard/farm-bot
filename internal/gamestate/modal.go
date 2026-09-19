package gamestate

import (
	"fmt"
	"time"
)

const (
	wardensOfTheCycleGrpID  = 93838
	wardensLowLifeThreshold = 5
	modalDedupeWindow       = 2 * time.Second
	modalWaitWindow         = 8 * time.Second
)

func (s *Store) modalArmed() bool {
	return !s.modalUntil.IsZero() && time.Now().Before(s.modalUntil)
}

func (s *Store) applyModalSelectN(msg greMessage) bool {
	req := msg.SelectNReq
	if req == nil {
		return false
	}
	if req.Context != "SelectionContext_Resolution" {
		return false
	}
	now := time.Now()
	if !s.lastModalAt.IsZero() && now.Sub(s.lastModalAt) < modalDedupeWindow {
		return false
	}

	n := len(req.IDs)
	if n < 1 {
		n = 1
	}
	source := req.SourceID
	job := ModalChoiceJob{
		Kind:     ModalChoiceLast,
		NOptions: n,
		SourceID: source,
		Reason:   fmt.Sprintf("%d option(s) (source=%d) -> clicking bottom (lose life)", n, source),
	}
	if n == 2 && s.isWardensSource(source) {
		life := s.myLifeTotal()
		job.Life = life
		if life > 0 && life < wardensLowLifeThreshold {
			job.Kind = ModalChoiceWardensGainLife
			job.Reason = fmt.Sprintf("Wardens of the Cycle (source=%d) life=%d -> gain_2_life", source, life)
		} else {
			job.Kind = ModalChoiceWardensDraw
			lifeStr := "unknown"
			if life > 0 {
				lifeStr = fmt.Sprintf("%d", life)
			}
			job.Reason = fmt.Sprintf("Wardens of the Cycle (source=%d) life=%s -> draw_lose_1_life", source, lifeStr)
		}
	}

	s.lastModalAt = now
	s.modalUntil = now.Add(modalWaitWindow)
	s.modalJob = &job
	if s.systemSeatID > 0 {
		s.turn.DecisionPlayer = s.systemSeatID
	}
	return true
}

func (s *Store) isWardensSource(instanceID int) bool {
	if instanceID <= 0 {
		return false
	}
	obj, ok := s.objects[instanceID]
	if !ok {
		return false
	}
	return obj.GrpID == wardensOfTheCycleGrpID || obj.ObjectSourceGrpID == wardensOfTheCycleGrpID
}

func (s *Store) myLifeTotal() int {
	if s.systemSeatID <= 0 {
		return 0
	}
	p, ok := s.players[s.systemSeatID]
	if !ok {
		return 0
	}
	return p.LifeTotal
}

func (s *Store) clearModalLocked() {
	s.modalUntil = time.Time{}
	s.modalJob = nil
}

// TakeModalChoiceJob 取出待点击的结算 Choose One（只取一次）。
func (s *Store) TakeModalChoiceJob() (ModalChoiceJob, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.modalJob == nil {
		return ModalChoiceJob{}, false
	}
	job := *s.modalJob
	s.modalJob = nil
	return job, true
}

// ModalChoiceStillOpen 结算 Choose One 暂停是否仍有效。
func (s *Store) ModalChoiceStillOpen() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.modalUntil.IsZero() {
		return false
	}
	if time.Now().After(s.modalUntil) {
		s.clearModalLocked()
		return false
	}
	return true
}

// ClearModalChoice 结束结算 Choose One 暂停。
func (s *Store) ClearModalChoice() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clearModalLocked()
}
