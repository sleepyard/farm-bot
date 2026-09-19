package gamestate

import "testing"

func TestParseAndFullReplace(t *testing.T) {
	line := `[UnityCrossThreadLogger]==> {"greToClientEvent":{"greToClientMessages":[{
		"type":"GREMessageType_GameStateMessage",
		"systemSeatIds":[2],
		"gameStateMessage":{
			"type":"GameStateType_Full",
			"pendingMessageCount":0,
			"gameInfo":{"matchID":"abc-123","stage":"GameStage_Play"},
			"turnInfo":{"phase":"Phase_Main1","step":"Step_Main","turnNumber":3,"activePlayer":2,"priorityPlayer":2,"decisionPlayer":2},
			"zones":[{"zoneId":31,"type":"ZoneType_Hand","ownerSeatId":2,"objectInstanceIds":[10,20]}],
			"gameObjects":[
				{"instanceId":10,"grpId":100,"zoneId":31,"ownerSeatId":2,"controllerSeatId":2,"cardTypes":["CardType_Land"]},
				{"instanceId":20,"grpId":200,"zoneId":31,"ownerSeatId":2,"controllerSeatId":2,"cardTypes":["CardType_Creature"]}
			],
			"players":[{"systemSeatNumber":2,"lifeTotal":20}]
		}
	},{
		"type":"GREMessageType_ActionsAvailableReq",
		"systemSeatIds":[2],
		"actionsAvailableReq":{"actions":[
			{"actionType":"ActionType_Play","instanceId":10},
			{"actionType":"ActionType_Cast","instanceId":20,"grpId":200},
			{"actionType":"ActionType_Pass"}
		]}
	}]}}`

	env, ok := ParseLogLine(line)
	if !ok {
		t.Fatal("parse failed")
	}
	st := NewStore()
	if !st.ApplyEnvelope(env) {
		t.Fatal("apply failed")
	}
	snap := st.Snapshot(true)
	if snap.SystemSeatID != 2 {
		t.Fatalf("seat=%d", snap.SystemSeatID)
	}
	if !snap.IsOurDecision() {
		t.Fatal("expected our decision")
	}
	pc := snap.PlayOrCastActions()
	if len(pc) != 2 {
		t.Fatalf("play/cast=%d %#v", len(pc), pc)
	}
	if snap.MatchID != "abc-123" {
		t.Fatalf("match=%s", snap.MatchID)
	}
	if len(snap.Objects) != 2 {
		t.Fatalf("objs=%d", len(snap.Objects))
	}
}

func TestDiffDeletesObject(t *testing.T) {
	st := NewStore()
	full := `{"greToClientEvent":{"greToClientMessages":[{"type":"GREMessageType_GameStateMessage","systemSeatIds":[1],"gameStateMessage":{
		"type":"GameStateType_Full",
		"turnInfo":{"phase":"Phase_Main1","turnNumber":1,"decisionPlayer":1,"activePlayer":1,"priorityPlayer":1},
		"gameObjects":[{"instanceId":7,"grpId":1,"zoneId":1,"controllerSeatId":1}]
	}}]}}`
	env, ok := ParseLogLine(full)
	if !ok {
		t.Fatal("full parse")
	}
	st.ApplyEnvelope(env)

	diff := `{"greToClientEvent":{"greToClientMessages":[{"type":"GREMessageType_GameStateMessage","gameStateMessage":{
		"type":"GameStateType_Diff",
		"turnInfo":{"turnNumber":2,"decisionPlayer":1},
		"diffDeletedInstanceIds":[7]
	}}]}}`
	env2, ok := ParseLogLine(diff)
	if !ok {
		t.Fatal("diff parse")
	}
	st.ApplyEnvelope(env2)
	snap := st.Snapshot(false)
	if len(snap.Objects) != 0 {
		t.Fatalf("expected delete, objs=%d", len(snap.Objects))
	}
	if snap.Turn.TurnNumber != 2 {
		t.Fatalf("turn=%d", snap.Turn.TurnNumber)
	}
}

func TestFlexStatAndPerMessageParse(t *testing.T) {
	line := `{"greToClientEvent":{"greToClientMessages":[{"type":"GREMessageType_GameStateMessage","systemSeatIds":[2],"gameStateMessage":{"type":"GameStateType_Diff","turnInfo":{"turnNumber":3,"phase":"Phase_Main1","activePlayer":2,"priorityPlayer":2},"gameObjects":[{"instanceId":9,"grpId":1,"power":{},"toughness":{"value":2}}]}},{"type":"GREMessageType_ActionsAvailableReq","systemSeatIds":[2],"actionsAvailableReq":{"actions":[{"actionType":"ActionType_Play","instanceId":9,"grpId":1}]}}]}}`
	st := NewStore()
	st.systemSeatID = 2
	st.turn = TurnInfo{TurnNumber: 2, DecisionPlayer: 1, Phase: PhaseMain1, Step: "Step_Draw"}
	env, ok := ParseLogLine(line)
	if !ok {
		t.Fatal("parse")
	}
	if !st.ApplyEnvelope(env) {
		t.Fatal("apply")
	}
	snap := st.Snapshot(true)
	if snap.Turn.TurnNumber != 3 {
		t.Fatalf("turn=%d", snap.Turn.TurnNumber)
	}
	if !snap.IsOurDecision() {
		t.Fatalf("expected ours, dp=%d actions=%d", snap.Turn.DecisionPlayer, len(snap.Actions))
	}
	if len(snap.PlayOrCastActions()) != 1 {
		t.Fatalf("actions=%#v", snap.Actions)
	}
	if snap.Turn.Step != "" {
		t.Fatalf("stale step=%q", snap.Turn.Step)
	}
}

func TestSelectTargetsReqClassify(t *testing.T) {
	line := `{"greToClientEvent":{"greToClientMessages":[
		{"type":"GREMessageType_GameStateMessage","systemSeatIds":[1],"gameStateMessage":{
			"type":"GameStateType_Full","gameStateId":1,
			"turnInfo":{"turnNumber":2,"phase":"Phase_Main1","decisionPlayer":1,"priorityPlayer":1,"activePlayer":1},
			"zones":[{"zoneId":31,"type":"ZoneType_Battlefield","objectInstanceIds":[10,20]}],
			"gameObjects":[
				{"instanceId":10,"grpId":2,"zoneId":31,"controllerSeatId":1,"cardTypes":["CardType_Creature"],"power":{"value":2},"toughness":{"value":2}},
				{"instanceId":20,"grpId":3,"zoneId":31,"controllerSeatId":2,"cardTypes":["CardType_Creature"],"power":{"value":3},"toughness":{"value":3}},
				{"instanceId":99,"grpId":9,"zoneId":31,"controllerSeatId":1,"cardTypes":["CardType_Instant"]}
			],
			"players":[{"systemSeatNumber":1,"lifeTotal":20},{"systemSeatNumber":2,"lifeTotal":20}]
		}},
		{"type":"GREMessageType_SelectTargetsReq","systemSeatIds":[1],"selectTargetsReq":{
			"sourceId":99,
			"targets":[{"targets":[
				{"targetInstanceId":10,"legalAction":"SelectAction_Select"},
				{"targetInstanceId":20,"legalAction":"SelectAction_Select"},
				{"targetInstanceId":2,"legalAction":"SelectAction_Select"}
			]}]
		}}
	]}}`
	st := NewStore()
	env, ok := ParseLogLine(line)
	if !ok {
		t.Fatal("parse")
	}
	if !st.ApplyEnvelope(env) {
		t.Fatal("apply")
	}
	snap := st.Snapshot(true)
	if !snap.NeedsTargetSelect || snap.SelectTarget == nil {
		t.Fatalf("expected select target: %#v", snap.SelectTarget)
	}
	p := snap.SelectTarget
	if len(p.OwnCreatures) != 1 || p.OwnCreatures[0] != 10 {
		t.Fatalf("own=%v", p.OwnCreatures)
	}
	if len(p.OppCreatures) != 1 || p.OppCreatures[0] != 20 {
		t.Fatalf("opp=%v", p.OppCreatures)
	}
	if !p.FaceLegal {
		t.Fatal("face should be legal (seat 2)")
	}
	if p.SourceID != 99 {
		t.Fatalf("source=%d", p.SourceID)
	}
}

func applyLine(t *testing.T, st *Store, line string) {
	t.Helper()
	env, ok := ParseLogLine(line)
	if !ok {
		t.Fatal("parse failed: " + line[:min(80, len(line))])
	}
	st.ApplyEnvelope(env)
}

func TestCastingTimeKickerPlain(t *testing.T) {
	st := NewStore()
	st.systemSeatID = 1
	st.turn = TurnInfo{TurnNumber: 3, Phase: PhaseMain1, DecisionPlayer: 1}
	applyLine(t, st, `{"greToClientEvent":{"greToClientMessages":[{
		"type":"GREMessageType_CastingTimeOptionsReq","systemSeatIds":[1],"gameStateId":217,
		"castingTimeOptionsReq":{"castingTimeOptionReq":[{
			"castingTimeOptionType":"CastingTimeOptionType_Kicker","ctoId":1,"grpId":14410,"isRequired":false
		}]}
	}]}}`)
	job, ok := st.TakeCastingTimeJob()
	if !ok || job.Kind != CastingTimePlain {
		t.Fatalf("job=%#v ok=%v", job, ok)
	}
	if !st.CastingTimeStillOpen() {
		t.Fatal("expected pause")
	}
	if !st.Snapshot(true).NeedsCastingTimeOption {
		t.Fatal("snapshot should flag overlay")
	}
}

func TestCastingTimeChooseOrCostSacrifice(t *testing.T) {
	st := NewStore()
	st.systemSeatID = 1
	applyLine(t, st, `{"greToClientEvent":{"greToClientMessages":[{
		"type":"GREMessageType_CastingTimeOptionsReq","systemSeatIds":[1],
		"castingTimeOptionsReq":{"castingTimeOptionReq":[{
			"castingTimeOptionType":"CastingTimeOptionType_ChooseOrCost","ctoId":2,"grpId":9,"isRequired":true
		}]}
	}]}}`)
	job, ok := st.TakeCastingTimeJob()
	if !ok || job.Kind != CastingTimeSacrifice {
		t.Fatalf("job=%#v", job)
	}
}

func TestCastingTimeValorousStancePicksSecond(t *testing.T) {
	st := NewStore()
	st.systemSeatID = 1
	applyLine(t, st, `{"greToClientEvent":{"greToClientMessages":[{
		"type":"GREMessageType_CastingTimeOptionsReq","systemSeatIds":[1],
		"castingTimeOptionsReq":{"castingTimeOptionReq":[{
			"castingTimeOptionType":"CastingTimeOptionType_Modal","ctoId":2,"grpId":72198,"isRequired":true
		}]}
	}]}}`)
	job, ok := st.TakeCastingTimeJob()
	if !ok || job.Kind != CastingTimeModalSecond {
		t.Fatalf("job=%#v", job)
	}
}

func TestCastingTimeOtherModalStaysLeft(t *testing.T) {
	st := NewStore()
	st.systemSeatID = 1
	applyLine(t, st, `{"greToClientEvent":{"greToClientMessages":[{
		"type":"GREMessageType_CastingTimeOptionsReq","systemSeatIds":[1],
		"castingTimeOptionsReq":{"castingTimeOptionReq":[{
			"castingTimeOptionType":"CastingTimeOptionType_Modal","ctoId":1,"grpId":11111,"isRequired":true
		}]}
	}]}}`)
	job, ok := st.TakeCastingTimeJob()
	if !ok || job.Kind != CastingTimePlain {
		t.Fatalf("other modal should click left/plain, job=%#v", job)
	}
}

func TestCastingTimeOtherSeatIgnored(t *testing.T) {
	st := NewStore()
	st.systemSeatID = 1
	applyLine(t, st, `{"greToClientEvent":{"greToClientMessages":[{
		"type":"GREMessageType_CastingTimeOptionsReq","systemSeatIds":[2],
		"castingTimeOptionsReq":{"castingTimeOptionReq":[{
			"castingTimeOptionType":"CastingTimeOptionType_Modal","ctoId":2,"grpId":1
		}]}
	}]}}`)
	if _, ok := st.TakeCastingTimeJob(); ok {
		t.Fatal("other seat should not arm")
	}
	if st.CastingTimeStillOpen() {
		t.Fatal("should not pause")
	}
}

func TestCastingTimeTimerStateIdClosesWindow(t *testing.T) {
	st := NewStore()
	st.systemSeatID = 1
	applyLine(t, st, `{"greToClientEvent":{"greToClientMessages":[{
		"type":"GREMessageType_CastingTimeOptionsReq","systemSeatIds":[1],"gameStateId":224,
		"castingTimeOptionsReq":{"castingTimeOptionReq":[{
			"castingTimeOptionType":"CastingTimeOptionType_Modal","ctoId":2,"grpId":175864
		}]}
	}]}}`)
	if !st.CastingTimeStillOpen() {
		t.Fatal("armed")
	}
	applyLine(t, st, `{"greToClientEvent":{"greToClientMessages":[{
		"type":"GREMessageType_TimerStateMessage","systemSeatIds":[1,2],"gameStateId":226,
		"timerStateMessage":{"seatId":1,"timers":[]}
	}]}}`)
	if st.CastingTimeStillOpen() {
		t.Fatal("timer gameStateId should close overlay pause")
	}
}

func TestCastingTimeFollowUpTargetClosesWindow(t *testing.T) {
	st := NewStore()
	st.systemSeatID = 1
	applyLine(t, st, `{"greToClientEvent":{"greToClientMessages":[{
		"type":"GREMessageType_CastingTimeOptionsReq","systemSeatIds":[1],"gameStateId":10,
		"castingTimeOptionsReq":{"castingTimeOptionReq":[{"castingTimeOptionType":"CastingTimeOptionType_Modal","grpId":1}]}
	}]}}`)
	applyLine(t, st, `{"greToClientEvent":{"greToClientMessages":[{
		"type":"GREMessageType_SelectTargetsReq","systemSeatIds":[1],"selectTargetsReq":{"sourceId":1,"targets":[]}
	}]}}`)
	if st.CastingTimeStillOpen() {
		t.Fatal("SelectTargetsReq should close overlay pause")
	}
}

func TestCastingTimeFollowUpPayCostsClosesWindow(t *testing.T) {
	st := NewStore()
	st.systemSeatID = 1
	applyLine(t, st, `{"greToClientEvent":{"greToClientMessages":[{
		"type":"GREMessageType_CastingTimeOptionsReq","systemSeatIds":[1],"gameStateId":10,
		"castingTimeOptionsReq":{"castingTimeOptionReq":[{"castingTimeOptionType":"CastingTimeOptionType_Kicker"}]}
	}]}}`)
	applyLine(t, st, `{"greToClientEvent":{"greToClientMessages":[{
		"type":"GREMessageType_PayCostsReq","systemSeatIds":[1]
	}]}}`)
	if st.CastingTimeStillOpen() {
		t.Fatal("PayCostsReq should close overlay pause")
	}
}

func TestCastingTimeDedupeWithin2s(t *testing.T) {
	st := NewStore()
	st.systemSeatID = 1
	line := `{"greToClientEvent":{"greToClientMessages":[{
		"type":"GREMessageType_CastingTimeOptionsReq","systemSeatIds":[1],
		"castingTimeOptionsReq":{"castingTimeOptionReq":[{"castingTimeOptionType":"CastingTimeOptionType_Kicker"}]}
	}]}}`
	applyLine(t, st, line)
	job1, ok1 := st.TakeCastingTimeJob()
	applyLine(t, st, line)
	job2, ok2 := st.TakeCastingTimeJob()
	if !ok1 || job1.Kind != CastingTimePlain {
		t.Fatalf("first job=%#v", job1)
	}
	if ok2 {
		t.Fatalf("duplicate should be ignored, got %#v", job2)
	}
}

func TestGroupReqArmsPause(t *testing.T) {
	st := NewStore()
	st.systemSeatID = 1
	applyLine(t, st, `{"greToClientEvent":{"greToClientMessages":[{
		"type":"GREMessageType_GroupReq","systemSeatIds":[1],
		"groupReq":{"context":"AbilityContext_Scry"}
	}]}}`)
	job, ok := st.TakeGroupReqJob()
	if !ok || job.Context != "AbilityContext_Scry" {
		t.Fatalf("job=%#v ok=%v", job, ok)
	}
	if !st.GroupReqStillOpen() {
		t.Fatal("expected group pause")
	}
	if !st.Snapshot(true).NeedsGroupReq {
		t.Fatal("snapshot should flag group overlay")
	}
}

func TestGroupReqSurveilAlsoArms(t *testing.T) {
	st := NewStore()
	st.systemSeatID = 1
	applyLine(t, st, `{"greToClientEvent":{"greToClientMessages":[{
		"type":"GREMessageType_GroupReq","systemSeatIds":[1],
		"groupReq":{"context":"AbilityContext_Surveil"}
	}]}}`)
	job, ok := st.TakeGroupReqJob()
	if !ok || job.Context != "AbilityContext_Surveil" {
		t.Fatalf("job=%#v", job)
	}
}

func TestGroupReqOtherSeatIgnored(t *testing.T) {
	st := NewStore()
	st.systemSeatID = 1
	applyLine(t, st, `{"greToClientEvent":{"greToClientMessages":[{
		"type":"GREMessageType_GroupReq","systemSeatIds":[2],
		"groupReq":{"context":"AbilityContext_Scry"}
	}]}}`)
	if _, ok := st.TakeGroupReqJob(); ok {
		t.Fatal("other seat should not arm")
	}
	if st.GroupReqStillOpen() {
		t.Fatal("should not pause")
	}
}

func TestGroupReqDedupeWithin2s(t *testing.T) {
	st := NewStore()
	st.systemSeatID = 1
	line := `{"greToClientEvent":{"greToClientMessages":[{
		"type":"GREMessageType_GroupReq","systemSeatIds":[1],
		"groupReq":{"context":"AbilityContext_Scry"}
	}]}}`
	applyLine(t, st, line)
	_, ok1 := st.TakeGroupReqJob()
	applyLine(t, st, line)
	_, ok2 := st.TakeGroupReqJob()
	if !ok1 {
		t.Fatal("first group job missing")
	}
	if ok2 {
		t.Fatal("duplicate group req should be ignored")
	}
}

func TestAssignDamageReqArmsPause(t *testing.T) {
	st := NewStore()
	st.systemSeatID = 1
	applyLine(t, st, `{"greToClientEvent":{"greToClientMessages":[{
		"type":"GREMessageType_AssignDamageReq","systemSeatIds":[1]
	}]}}`)
	if _, ok := st.TakeAssignDamageJob(); !ok {
		t.Fatal("expected assign damage job")
	}
	if !st.AssignDamageStillOpen() {
		t.Fatal("expected assign pause")
	}
	if !st.Snapshot(true).NeedsAssignDamage {
		t.Fatal("snapshot should flag assign overlay")
	}
}

func TestAssignDamageOtherSeatIgnored(t *testing.T) {
	st := NewStore()
	st.systemSeatID = 1
	applyLine(t, st, `{"greToClientEvent":{"greToClientMessages":[{
		"type":"GREMessageType_AssignDamageReq","systemSeatIds":[2]
	}]}}`)
	if _, ok := st.TakeAssignDamageJob(); ok {
		t.Fatal("other seat should not arm")
	}
}

func TestAssignDamageClearsAfterLeavingCombatDamage(t *testing.T) {
	st := NewStore()
	st.systemSeatID = 1
	applyLine(t, st, `{"greToClientEvent":{"greToClientMessages":[{
		"type":"GREMessageType_AssignDamageReq","systemSeatIds":[1]
	}]}}`)
	applyLine(t, st, `{"greToClientEvent":{"greToClientMessages":[{
		"type":"GREMessageType_GameStateMessage",
		"gameStateMessage":{"type":"GameStateType_Diff","turnInfo":{"step":"Step_Main2"}}
	}]}}`)
	if st.AssignDamageStillOpen() {
		t.Fatal("should clear after leaving combat damage")
	}
}

func TestDropSelectNIDStopsPrompt(t *testing.T) {
	st := NewStore()
	st.systemSeatID = 1
	applyLine(t, st, `{"greToClientEvent":{"greToClientMessages":[{
		"type":"GREMessageType_SelectNReq","systemSeatIds":[1],
		"selectNReq":{"minSel":1,"maxSel":1,"ids":[11,22],"idType":"IdType_InstanceId"}
	}]}}`)
	if !st.Snapshot(true).NeedsSelectN() {
		t.Fatal("expected selectN")
	}
	st.DropSelectNID(11)
	snap := st.Snapshot(true)
	if !snap.NeedsSelectN() || len(snap.SelectN.IDs) != 1 || snap.SelectN.IDs[0] != 22 {
		t.Fatalf("after drop 11: %#v", snap.SelectN)
	}
	st.DropSelectNID(22)
	if st.Snapshot(true).NeedsSelectN() {
		t.Fatal("expected prompt cleared")
	}
}

func TestMatchOutcomeWinLoss(t *testing.T) {
	st := NewStore()
	applyLine(t, st, `{"greToClientEvent":{"greToClientMessages":[{
		"type":"GREMessageType_GameStateMessage","systemSeatIds":[1],
		"gameStateMessage":{
			"type":"GameStateType_Full",
			"gameInfo":{"matchID":"m1","results":[{"result":"ResultType_WinLoss","winningTeamId":1}]},
			"players":[{"systemSeatNumber":1,"teamId":1},{"systemSeatNumber":2,"teamId":2}]
		}
	}]}}`)
	won, ok := st.MatchOutcome()
	if !ok || !won {
		t.Fatalf("want win, got won=%v ok=%v", won, ok)
	}

	st2 := NewStore()
	applyLine(t, st2, `{"greToClientEvent":{"greToClientMessages":[{
		"type":"GREMessageType_GameStateMessage","systemSeatIds":[1],
		"gameStateMessage":{
			"type":"GameStateType_Full",
			"gameInfo":{"matchID":"m2","results":[{"result":"ResultType_WinLoss","winningTeamId":2}]},
			"players":[{"systemSeatNumber":1,"teamId":1},{"systemSeatNumber":2,"teamId":2}]
		}
	}]}}`)
	won, ok = st2.MatchOutcome()
	if !ok || won {
		t.Fatalf("want loss, got won=%v ok=%v", won, ok)
	}
}

func TestSelectNPromptParameterIndexClicksLast(t *testing.T) {
	st := NewStore()
	st.systemSeatID = 1
	applyLine(t, st, `{"greToClientEvent":{"greToClientMessages":[{
		"type":"GREMessageType_SelectNReq","systemSeatIds":[1],
		"selectNReq":{"ids":[1,2,3],"idType":"IdType_PromptParameterIndex",
			"context":"SelectionContext_Resolution","sourceId":77,"minSel":1,"maxSel":1}
	}]}}`)
	if st.Snapshot(true).NeedsSelectN() {
		t.Fatal("resolution modal must not use hand SelectN")
	}
	job, ok := st.TakeModalChoiceJob()
	if !ok || job.Kind != ModalChoiceLast || job.NOptions != 3 {
		t.Fatalf("job=%#v ok=%v", job, ok)
	}
	if !st.ModalChoiceStillOpen() {
		t.Fatal("expected modal pause")
	}
}

func TestSelectNPromptParameterIndexWardensDraw(t *testing.T) {
	st := NewStore()
	st.systemSeatID = 1
	st.objects[42] = Object{InstanceID: 42, GrpID: 93838}
	st.players[1] = wirePlayer{SystemSeatNumber: 1, LifeTotal: 17}
	applyLine(t, st, `{"greToClientEvent":{"greToClientMessages":[{
		"type":"GREMessageType_SelectNReq","systemSeatIds":[1],
		"selectNReq":{"ids":[1,2],"idType":"IdType_PromptParameterIndex",
			"context":"SelectionContext_Resolution","sourceId":42,"minSel":1,"maxSel":1}
	}]}}`)
	job, ok := st.TakeModalChoiceJob()
	if !ok || job.Kind != ModalChoiceWardensDraw {
		t.Fatalf("job=%#v ok=%v", job, ok)
	}
}

func TestSelectNPromptParameterIndexWardensLowLife(t *testing.T) {
	st := NewStore()
	st.systemSeatID = 1
	st.objects[42] = Object{InstanceID: 42, ObjectSourceGrpID: 93838}
	st.players[1] = wirePlayer{SystemSeatNumber: 1, LifeTotal: 4}
	applyLine(t, st, `{"greToClientEvent":{"greToClientMessages":[{
		"type":"GREMessageType_SelectNReq","systemSeatIds":[1],
		"selectNReq":{"ids":[1,2],"idType":"IdType_PromptParameterIndex",
			"context":"SelectionContext_Resolution","sourceId":42,"minSel":1,"maxSel":1}
	}]}}`)
	job, ok := st.TakeModalChoiceJob()
	if !ok || job.Kind != ModalChoiceWardensGainLife {
		t.Fatalf("job=%#v ok=%v", job, ok)
	}
}

func TestZombifyChooserFromGraveyard(t *testing.T) {
	line := `{"greToClientEvent":{"greToClientMessages":[
		{"type":"GREMessageType_GameStateMessage","systemSeatIds":[1],"gameStateMessage":{
			"type":"GameStateType_Full","gameStateId":1,
			"zones":[
				{"zoneId":31,"type":"ZoneType_Battlefield","objectInstanceIds":[]},
				{"zoneId":40,"type":"ZoneType_Graveyard","objectInstanceIds":[11,12]}
			],
			"gameObjects":[
				{"instanceId":11,"grpId":100,"zoneId":40,"controllerSeatId":1,"cardTypes":["CardType_Creature"],"power":{"value":2},"toughness":{"value":2}},
				{"instanceId":12,"grpId":200,"zoneId":40,"controllerSeatId":1,"cardTypes":["CardType_Creature"],"power":{"value":5},"toughness":{"value":5}},
				{"instanceId":99,"grpId":9,"zoneId":31,"controllerSeatId":1,"cardTypes":["CardType_Sorcery"]}
			],
			"players":[{"systemSeatNumber":1,"lifeTotal":20}]
		}},
		{"type":"GREMessageType_SelectTargetsReq","systemSeatIds":[1],"selectTargetsReq":{
			"sourceId":99,
			"targets":[{"targets":[
				{"targetInstanceId":11,"legalAction":"SelectAction_Select"},
				{"targetInstanceId":12,"legalAction":"SelectAction_Select"}
			]}]
		}}
	]}}`
	st := NewStore()
	env, ok := ParseLogLine(line)
	if !ok {
		t.Fatal("parse")
	}
	st.ApplyEnvelope(env)
	p := st.Snapshot(true).SelectTarget
	if p == nil || len(p.ChooserIDs) != 2 {
		t.Fatalf("chooser=%#v", p)
	}
	if len(p.OwnCreatures) != 0 || len(p.OppCreatures) != 0 {
		t.Fatalf("gy cards should not be battlefield targets: %#v", p)
	}
}

func TestDeclareAttackersReqPlaneswalker(t *testing.T) {
	st := NewStore()
	st.systemSeatID = 1
	st.turn = TurnInfo{TurnNumber: 4, Phase: PhaseCombat, Step: StepDeclareAttack, ActivePlayer: 1, DecisionPlayer: 1}
	applyLine(t, st, `{"greToClientEvent":{"greToClientMessages":[{
		"type":"GREMessageType_DeclareAttackersReq","systemSeatIds":[1],
		"declareAttackersReq":{"canSubmitAttackers":false,"attackers":[
			{"attackerInstanceId":11,"legalDamageRecipients":[
				{"type":"DamageRecType_Player"},
				{"type":"DamageRecType_PlanesWalker"}
			]},
			{"attackerInstanceId":12,"legalDamageRecipients":[
				{"type":"DamageRecType_Player"},
				{"type":"DamageRecType_PlanesWalker"}
			]}
		]}
	}]}}`)
	snap := st.Snapshot(true)
	if !snap.AttackTargetRequired {
		t.Fatal("expected planeswalker attack-target")
	}
	if len(snap.AttackerIDs) != 2 || snap.AttackerIDs[0] != 11 || snap.AttackerIDs[1] != 12 {
		t.Fatalf("attackers=%v", snap.AttackerIDs)
	}

	applyLine(t, st, `{"greToClientEvent":{"greToClientMessages":[{
		"type":"GREMessageType_GameStateMessage","systemSeatIds":[1],
		"gameStateMessage":{"type":"GameStateType_Diff","turnInfo":{
			"turnNumber":4,"phase":"Phase_Main2","step":"Step_BeginCombat","activePlayer":1,"decisionPlayer":1
		}}
	}]}}`)
	if st.Snapshot(true).AttackTargetRequired {
		t.Fatal("should clear after leaving declare attack")
	}
}
