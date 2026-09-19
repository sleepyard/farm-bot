package gamestate

import "time"

const (
	assignDedupeWindow = 2 * time.Second
	assignWaitWindow   = 12 * time.Second
)

func (s *Store) assignArmed() bool {
	return !s.assignUntil.IsZero() && time.Now().Before(s.assignUntil)
}

func (s *Store) applyAssignDamageReq(msg greMessage) bool {
	now := time.Now()
	if !s.lastAssignAt.IsZero() && now.Sub(s.lastAssignAt) < assignDedupeWindow {
		return false
	}
	if !s.ctoOurs(msg.SystemSeatIDs) {
		return false
	}
	s.lastAssignAt = now
	s.assignUntil = now.Add(assignWaitWindow)
	s.assignJob = &AssignDamageJob{}
	return true
}

func (s *Store) clearAssignLocked() {
	s.assignUntil = time.Time{}
	s.assignJob = nil
}

func (s *Store) maybeClearAssignAfterStep() {
	if s.turn.Step == "" || s.turn.Step == StepCombatDamage {
		return
	}
	s.clearAssignLocked()
}

// TakeAssignDamageJob 取出待点击的伤害分配 Done（只取一次）。
func (s *Store) TakeAssignDamageJob() (AssignDamageJob, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.assignJob == nil {
		return AssignDamageJob{}, false
	}
	job := *s.assignJob
	s.assignJob = nil
	return job, true
}

// AssignDamageStillOpen 伤害分配弹窗暂停是否仍有效。
func (s *Store) AssignDamageStillOpen() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.assignUntil.IsZero() {
		return false
	}
	if time.Now().After(s.assignUntil) {
		s.clearAssignLocked()
		return false
	}
	if s.turn.Step != "" && s.turn.Step != StepCombatDamage {
		s.clearAssignLocked()
		return false
	}
	return true
}

// ClearAssignDamage 结束伤害分配暂停。
func (s *Store) ClearAssignDamage() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clearAssignLocked()
}
