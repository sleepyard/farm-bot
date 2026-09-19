package match

import (
	"time"

	"github.com/flourbrain/mtga-farm-bot/internal/gamestate"
)

const (
	modalOverlayDelay = 800 * time.Millisecond
	modalRetryEvery   = 1400 * time.Millisecond
	modalMaxRetries   = 2
)

func (s *Session) answerModalChoice(job gamestate.ModalChoiceJob) {
	s.logf("MODAL_CHOICE: %s", job.Reason)
	if !s.sleepInterruptible(modalOverlayDelay, s.doneCh()) {
		s.store.ClearModalChoice()
		return
	}
	s.clickModalChoice(job, 0)
}

func (s *Session) clickModalChoice(job gamestate.ModalChoiceJob, attempt int) {
	if s.stopped() || s.isDone() {
		s.store.ClearModalChoice()
		return
	}
	if attempt > 0 && !s.store.ModalChoiceStillOpen() {
		s.logf("MODAL_CHOICE: modal resolved before retry %d; stopping.", attempt)
		return
	}
	if err := s.ctrl.ClickModalChoice(job); err != nil {
		s.logf("MODAL_CHOICE click failed: %v", err)
		s.store.ClearModalChoice()
		return
	}
	if attempt < modalMaxRetries {
		if !s.sleepInterruptible(modalRetryEvery, s.doneCh()) {
			s.store.ClearModalChoice()
			return
		}
		s.clickModalChoice(job, attempt+1)
		return
	}
	if !s.sleepInterruptible(modalRetryEvery, s.doneCh()) {
		s.store.ClearModalChoice()
		return
	}
	s.log("MODAL_CHOICE wait cleared: retries exhausted.")
	s.store.ClearModalChoice()
}
