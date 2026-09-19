package gamestate

import "strings"

func (s *Store) applyDeclareAttackersReq(msg greMessage) bool {
	if msg.DeclareAttackersReq == nil {
		return false
	}
	seat := singletonSeat(msg.SystemSeatIDs)
	if seat == 0 {
		seat = s.systemSeatID
	}
	if s.systemSeatID > 0 && seat != s.systemSeatID {
		return false
	}
	if s.systemSeatID > 0 && s.turn.ActivePlayer > 0 && s.turn.ActivePlayer != s.systemSeatID {
		return false
	}

	req := msg.DeclareAttackersReq
	list := req.Attackers
	if len(list) == 0 {
		list = req.QualifiedAttackers
	}
	ids := make([]int, 0, len(list))
	pw := false
	for _, a := range list {
		if a.AttackerInstanceID > 0 {
			ids = append(ids, a.AttackerInstanceID)
		}
		for _, rec := range a.LegalDamageRecipients {
			if strings.Contains(strings.ToLower(rec.Type), "planeswalker") {
				pw = true
			}
		}
	}
	s.attackerIDs = ids
	s.attackPW = pw
	return true
}

func (s *Store) maybeClearAttackTarget() {
	if s.turn.Step != "" && s.turn.Step != StepDeclareAttack {
		s.attackPW = false
		s.attackerIDs = nil
	}
}

func actionsNeedAttackTarget(acts []Action) bool {
	for _, a := range acts {
		t := a.ActionType
		if t == "" {
			continue
		}
		if strings.Contains(t, "AttackTarget") || strings.Contains(t, "SelectAttackTarget") {
			return true
		}
		if strings.Contains(t, "Target") && (strings.Contains(t, "Attack") || strings.Contains(t, "Combat")) {
			return true
		}
	}
	return false
}
