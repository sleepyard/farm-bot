package gamestate

// noteMatchResultLocked 从 GSM results 推断本局胜负（调用方已持锁）。
func (s *Store) noteMatchResultLocked(gsm *wireGSM) {
	if gsm == nil || gsm.GameInfo == nil {
		return
	}
	winTeam := 0
	found := false
	for _, r := range gsm.GameInfo.Results {
		if r.Result == "ResultType_WinLoss" && r.WinningTeamID != 0 {
			winTeam = r.WinningTeamID
			found = true
			break
		}
	}
	if !found {
		return
	}
	myTeam := 0
	if s.systemSeatID > 0 {
		if p, ok := s.players[s.systemSeatID]; ok {
			myTeam = p.TeamID
		}
	}
	if myTeam == 0 {
		myTeam = s.systemSeatID
	}
	if myTeam == 0 {
		return
	}
	won := winTeam == myTeam
	s.matchWon = &won
}

// MatchOutcome 返回本局是否胜利；ok=false 表示尚未从 GRE 读到胜负。
func (s *Store) MatchOutcome() (won bool, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.matchWon == nil {
		return false, false
	}
	return *s.matchWon, true
}

// SeatAndTeam 供 MatchCompleted 行兜底解析。
func (s *Store) SeatAndTeam() (seat, team int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	seat = s.systemSeatID
	if p, ok := s.players[seat]; ok {
		team = p.TeamID
	}
	if team == 0 {
		team = seat
	}
	return seat, team
}
