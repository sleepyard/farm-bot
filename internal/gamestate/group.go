package gamestate

import "time"

const (
	groupDedupeWindow = 2 * time.Second
	groupWaitWindow   = 6 * time.Second
)

func (s *Store) groupArmed() bool {
	return !s.groupUntil.IsZero() && time.Now().Before(s.groupUntil)
}

func (s *Store) applyGroupReq(msg greMessage) bool {
	now := time.Now()
	if !s.lastGroupAt.IsZero() && now.Sub(s.lastGroupAt) < groupDedupeWindow {
		return false
	}
	if !s.ctoOurs(msg.SystemSeatIDs) {
		return false
	}
	ctx := ""
	if msg.GroupReq != nil {
		ctx = msg.GroupReq.Context
	}
	s.lastGroupAt = now
	s.groupUntil = now.Add(groupWaitWindow)
	s.groupJob = &GroupReqJob{Context: ctx}
	return true
}

func (s *Store) clearGroupLocked() {
	s.groupUntil = time.Time{}
	s.groupJob = nil
}

// TakeGroupReqJob 取出待点击的 Scry/Surveil Done（只取一次）。
func (s *Store) TakeGroupReqJob() (GroupReqJob, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.groupJob == nil {
		return GroupReqJob{}, false
	}
	job := *s.groupJob
	s.groupJob = nil
	return job, true
}

// GroupReqStillOpen 分组弹窗暂停是否仍有效。
func (s *Store) GroupReqStillOpen() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.groupUntil.IsZero() {
		return false
	}
	if time.Now().After(s.groupUntil) {
		s.clearGroupLocked()
		return false
	}
	return true
}

// ClearGroupReq 结束分组弹窗暂停。
func (s *Store) ClearGroupReq() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clearGroupLocked()
}
