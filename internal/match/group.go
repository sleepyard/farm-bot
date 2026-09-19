package match

import (
	"time"

	"github.com/flourbrain/mtga-farm-bot/internal/gamestate"
)

const groupOverlayDelay = 800 * time.Millisecond

func (s *Session) answerGroupReq(job gamestate.GroupReqJob) {
	ctx := job.Context
	if ctx == "" {
		ctx = "unparsed"
	}
	s.logf("GROUP_REQ (%s): clicking Done via scry_done.png (no reordering).", ctx)
	if !s.sleepInterruptible(groupOverlayDelay, s.doneCh()) {
		s.store.ClearGroupReq()
		return
	}
	if s.stopped() || s.isDone() {
		s.store.ClearGroupReq()
		return
	}
	if err := s.ctrl.ClickScryDone(); err != nil {
		s.logf("GROUP_REQ Done click failed: %v", err)
		return
	}
	s.log("GROUP_REQ Done click: matched scry_done.png template.")
}
