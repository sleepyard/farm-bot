package match

import (
	"time"

	"github.com/flourbrain/mtga-farm-bot/internal/gamestate"
)

const assignDamageDelay = time.Second

func (s *Session) answerAssignDamage(_ gamestate.AssignDamageJob) {
	s.log("ASSIGN_DAMAGE: 搜图点击 assign_damage_done.png")
	if !s.sleepInterruptible(assignDamageDelay, s.doneCh()) {
		s.store.ClearAssignDamage()
		return
	}
	for i := 0; i < 3; i++ {
		if s.stopped() || s.isDone() {
			s.store.ClearAssignDamage()
			return
		}
		if !s.store.AssignDamageStillOpen() {
			return
		}
		if err := s.ctrl.ClickAssignDamageDone(); err != nil {
			s.logf("ASSIGN_DAMAGE Done 失败: %v", err)
			if !s.sleepInterruptible(500*time.Millisecond, s.doneCh()) {
				s.store.ClearAssignDamage()
				return
			}
			continue
		}
		s.log("ASSIGN_DAMAGE: 已点击 assign_damage_done.png")
		s.store.ClearAssignDamage()
		return
	}
}
