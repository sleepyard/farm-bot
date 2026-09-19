package match

import (
	"fmt"
	"strings"
	"time"

	"github.com/flourbrain/mtga-farm-bot/internal/gamestate"
)

const (
	ctoOverlayDelay = 1 * time.Second
	ctoSettle       = 400 * time.Millisecond
	ctoRetryEvery   = 1600 * time.Millisecond
	ctoMaxRetries   = 2
)

func (s *Session) answerCastingTime(job gamestate.CastingTimeJob) {
	opts := strings.Join(job.Options, "; ")
	if opts == "" {
		opts = "unparsed"
	}
	s.log(fmt.Sprintf("CASTING_TIME_OPTIONS detected: options=[%s] — choosing %s.", opts, job.Desc))
	t := time.NewTimer(ctoOverlayDelay)
	defer t.Stop()
	select {
	case <-t.C:
	case <-s.doneCh():
		s.store.ClearCastingTime()
		return
	}
	s.clickCastingTime(job, 0)
}

func (s *Session) clickCastingTime(job gamestate.CastingTimeJob, attempt int) {
	if s.stopped() || s.isDone() {
		s.store.ClearCastingTime()
		return
	}
	if attempt > 0 && !s.store.CastingTimeStillOpen() {
		s.logf("CASTING_TIME_OPTION: dialog resolved before retry %d; stopping.", attempt)
		return
	}
	pt, label, err := s.ctrl.AimCastingTime(job.Kind)
	if err != nil {
		s.logf("CASTING_TIME_OPTION click failed: %v", err)
		s.store.ClearCastingTime()
		return
	}
	s.logf("CASTING_TIME_OPTION click (attempt %d): target=(%d,%d) %s", attempt, pt.X, pt.Y, label)
	if !s.sleepInterruptible(ctoSettle, s.doneCh()) {
		s.store.ClearCastingTime()
		return
	}
	if attempt > 0 && !s.store.CastingTimeStillOpen() {
		s.logf("CASTING_TIME_OPTION: dialog resolved during retry %d; not clicking.", attempt)
		return
	}
	if err := s.ctrl.ClickCastingTime(); err != nil {
		s.logf("CASTING_TIME_OPTION click failed: %v", err)
		s.store.ClearCastingTime()
		return
	}
	if attempt < ctoMaxRetries {
		t := time.NewTimer(ctoRetryEvery)
		defer t.Stop()
		select {
		case <-t.C:
			s.clickCastingTime(job, attempt+1)
		case <-s.doneCh():
			s.store.ClearCastingTime()
		}
		return
	}
	t := time.NewTimer(ctoRetryEvery)
	defer t.Stop()
	select {
	case <-t.C:
		s.log("CASTING_TIME_OPTION wait cleared: retries exhausted.")
		s.store.ClearCastingTime()
	case <-s.doneCh():
		s.store.ClearCastingTime()
	}
}

func (s *Session) doneCh() <-chan struct{} {
	if s.ctx != nil {
		return s.ctx.Done()
	}
	return nil
}
