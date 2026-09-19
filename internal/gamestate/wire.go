package gamestate

import (
	"encoding/json"
	"strconv"
	"strings"
)

// GRE 线缆结构（Player.log JSON）。字段名与日志一致。

type greEnvelope struct {
	GreToClientEvent *greToClientEvent `json:"greToClientEvent"`
}

type greToClientEvent struct {
	GreToClientMessages []greMessage `json:"greToClientMessages"`
}

type greMessage struct {
	Type                  string                 `json:"type"`
	SystemSeatIDs         []int                  `json:"systemSeatIds"`
	GameStateID           int                    `json:"gameStateId"`
	GameStateMessage      *wireGSM               `json:"gameStateMessage"`
	ActionsAvailableReq   *actionsAvailableReq   `json:"actionsAvailableReq"`
	SelectTargetsReq      *selectTargetsReq      `json:"selectTargetsReq"`
	SelectNReq            *selectNReq            `json:"selectNReq"`
	CastingTimeOptionsReq *castingTimeOptionsReq `json:"castingTimeOptionsReq"`
	GroupReq              *groupReq              `json:"groupReq"`
	DeclareAttackersReq   *declareAttackersReq   `json:"declareAttackersReq"`
}

type declareAttackersReq struct {
	Attackers          []declareAttacker `json:"attackers"`
	QualifiedAttackers []declareAttacker `json:"qualifiedAttackers"`
	CanSubmitAttackers bool              `json:"canSubmitAttackers"`
}

type declareAttacker struct {
	AttackerInstanceID    int               `json:"attackerInstanceId"`
	LegalDamageRecipients []damageRecipient `json:"legalDamageRecipients"`
}

type damageRecipient struct {
	Type string `json:"type"` // DamageRecType_PlanesWalker / DamageRecType_Player
}

type groupReq struct {
	Context string `json:"context"`
}

type castingTimeOptionsReq struct {
	Options []castingTimeOption `json:"castingTimeOptionReq"`
}

type castingTimeOption struct {
	Type       string `json:"castingTimeOptionType"`
	CtoID      int    `json:"ctoId"`
	GrpID      int    `json:"grpId"`
	IsRequired bool   `json:"isRequired"`
}

type selectTargetsReq struct {
	SourceID int                 `json:"sourceId"`
	Targets  []selectTargetGroup `json:"targets"`
}

type selectTargetGroup struct {
	Targets []selectTargetOpt `json:"targets"`
}

type selectTargetOpt struct {
	TargetInstanceID int    `json:"targetInstanceId"`
	LegalAction      string `json:"legalAction"`
}

type selectNReq struct {
	MinSel        int    `json:"minSel"`
	MaxSel        int    `json:"maxSel"`
	IDs           []int  `json:"ids"`
	Context       string `json:"context"`
	OptionContext string `json:"optionContext"`
	SourceID      int    `json:"sourceId"`
	IDType        string `json:"idType"`
}

type actionsAvailableReq struct {
	Actions []wireAction `json:"actions"`
}

type wireAction struct {
	ActionType   string         `json:"actionType"`
	InstanceID   int            `json:"instanceId"`
	GrpID        int            `json:"grpId"`
	AbilityGrpID int            `json:"abilityGrpId"`
	ManaCost     []wireManaCost `json:"manaCost"`
}

type wireManaCost struct {
	Color []string `json:"color"`
	Count int      `json:"count"`
}

type wireGSM struct {
	Type                   string        `json:"type"` // GameStateType_Full | GameStateType_Diff
	GameStateID            int           `json:"gameStateId"`
	PendingMessageCount    int           `json:"pendingMessageCount"`
	GameInfo               *wireGameInfo `json:"gameInfo"`
	TurnInfo               *wireTurnInfo `json:"turnInfo"`
	Zones                  []wireZone    `json:"zones"`
	GameObjects            []wireObject  `json:"gameObjects"`
	Players                []wirePlayer  `json:"players"`
	DiffDeletedInstanceIDs []int         `json:"diffDeletedInstanceIds"`
}

type wireGameInfo struct {
	MatchID string           `json:"matchID"`
	Stage   string           `json:"stage"`
	Results []wireGameResult `json:"results"`
}

type wireGameResult struct {
	Result        string `json:"result"`
	WinningTeamID int    `json:"winningTeamId"`
	Reason        string `json:"reason"`
}

type wireTurnInfo struct {
	Phase          string `json:"phase"`
	Step           string `json:"step"`
	TurnNumber     int    `json:"turnNumber"`
	ActivePlayer   int    `json:"activePlayer"`
	PriorityPlayer int    `json:"priorityPlayer"`
	DecisionPlayer int    `json:"decisionPlayer"`
}

type wireZone struct {
	ZoneID            int    `json:"zoneId"`
	Type              string `json:"type"`
	OwnerSeatID       int    `json:"ownerSeatId"`
	ObjectInstanceIDs []int  `json:"objectInstanceIds"`
}

type wireObject struct {
	InstanceID        int      `json:"instanceId"`
	GrpID             int      `json:"grpId"`
	ObjectSourceGrpID int      `json:"objectSourceGrpId"`
	ZoneID            int      `json:"zoneId"`
	OwnerSeatID       int      `json:"ownerSeatId"`
	ControllerSeatID  int      `json:"controllerSeatId"`
	CardTypes         []string `json:"cardTypes"`
	AttackState       string   `json:"attackState"`
	Power             *int     `json:"-"`
	Toughness         *int     `json:"-"`
	RawPower          flexStat `json:"power"`
	RawToughness      flexStat `json:"toughness"`
}

func (o *wireObject) normalizeStats() {
	if v, ok := o.RawPower.Int(); ok {
		o.Power = &v
	}
	if v, ok := o.RawToughness.Int(); ok {
		o.Toughness = &v
	}
}

type wirePlayer struct {
	SystemSeatNumber int `json:"systemSeatNumber"`
	LifeTotal        int `json:"lifeTotal"`
	TeamID           int `json:"teamId"`
}

// flexStat 兼容 GRE 的 power/toughness：纯数字、{"value":N}、或空对象 {}。
type flexStat struct {
	ok  bool
	val int
}

func (f flexStat) Int() (int, bool) { return f.val, f.ok }

func (f *flexStat) UnmarshalJSON(b []byte) error {
	b = bytesTrim(b)
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	if b[0] == '{' {
		var obj struct {
			Value *int `json:"value"`
		}
		if err := json.Unmarshal(b, &obj); err != nil {
			return nil
		}
		if obj.Value != nil {
			f.ok = true
			f.val = *obj.Value
		}
		return nil
	}
	var n int
	if err := json.Unmarshal(b, &n); err != nil {
		var s string
		if err2 := json.Unmarshal(b, &s); err2 == nil {
			if v, err3 := strconv.Atoi(strings.TrimSpace(s)); err3 == nil {
				f.ok = true
				f.val = v
			}
		}
		return nil
	}
	f.ok = true
	f.val = n
	return nil
}

func bytesTrim(b []byte) []byte {
	i, j := 0, len(b)
	for i < j && (b[i] == ' ' || b[i] == '\t' || b[i] == '\n' || b[i] == '\r') {
		i++
	}
	for j > i && (b[j-1] == ' ' || b[j-1] == '\t' || b[j-1] == '\n' || b[j-1] == '\r') {
		j--
	}
	return b[i:j]
}
