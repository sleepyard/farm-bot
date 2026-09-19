// Package gamestate 保存对局内 GRE 合并后的只读快照。
// 决策器只读本包；控制器不解析日志。
package gamestate

// Phase / Step 与 GRE turnInfo 字符串对齐（节选常用值）。
const (
	PhaseMain1  = "Phase_Main1"
	PhaseCombat = "Phase_Combat"
	PhaseMain2  = "Phase_Main2"

	StepDeclareAttack = "Step_DeclareAttack"
	StepDeclareBlock  = "Step_DeclareBlock"
	StepCombatDamage  = "Step_CombatDamage"
)

// ActionType GRE 可用动作类型。
const (
	ActionPlay         = "ActionType_Play"
	ActionCast         = "ActionType_Cast"
	ActionPass         = "ActionType_Pass"
	ActionActivate     = "ActionType_Activate"
	ActionActivateMana = "ActionType_Activate_Mana"
)

// ZoneType 区域。
const (
	ZoneHand        = "ZoneType_Hand"
	ZoneBattlefield = "ZoneType_Battlefield"
	ZoneStack       = "ZoneType_Stack"
	ZoneLibrary     = "ZoneType_Library"
	ZoneGraveyard   = "ZoneType_Graveyard"
)

// Object 是场上 / 手牌等区域中的一个游戏物件。
type Object struct {
	InstanceID        int
	GrpID             int
	ZoneID            int
	ControllerSeatID  int
	CardTypes         []string
	Power             int
	Toughness         int
	AttackState       string
	ObjectSourceGrpID int // 堆叠异能上溯原牌 grpId
}

// ManaPip 是 GRE action.manaCost 里的一段费用。
type ManaPip struct {
	Colors []string // e.g. ManaColor_White / ManaColor_Generic
	Count  int
}

// Action 是 ActionsAvailableReq / 合并后的可执行动作。
type Action struct {
	SeatID       int
	ActionType   string
	InstanceID   int
	GrpID        int
	AbilityGrpID int
	ManaCost     []ManaPip
}

// TurnInfo 回合信息。
type TurnInfo struct {
	TurnNumber     int
	Phase          string
	Step           string
	ActivePlayer   int
	PriorityPlayer int
	DecisionPlayer int
}

// SelectTargetPrompt 解析自 SelectTargetsReq 的合法目标分类。
type SelectTargetPrompt struct {
	SourceID     int
	OppCreatures []int // 对方战场生物
	OwnCreatures []int // 己方战场生物
	ChooserIDs   []int // 墓地/放逐等中屏选牌（Zombify）
	StackIDs     []int // 堆叠上的法术（反击目标）
	FaceLegal    bool  // 可点对手玩家
}

// CastingTimeKind 出牌时中屏 Choose One 的点选策略（对齐 Python）。
type CastingTimeKind string

const (
	CastingTimePlain       CastingTimeKind = "plain"        // kicker：左侧不加费版本
	CastingTimeModalSecond CastingTimeKind = "modal_second" // 右侧模式（如 Valorous Stance 消灭）
	CastingTimeSacrifice   CastingTimeKind = "sacrifice"    // 额外费用：牺牲生物
)

// CastingTimeJob 新到的 CastingTimeOptionsReq，由对局循环去点。
type CastingTimeJob struct {
	Kind    CastingTimeKind
	Desc    string
	Options []string
}

// GroupReqJob 占卜 / 探查等分组弹窗，由对局循环去点 Done。
type GroupReqJob struct {
	Context string
}

// AssignDamageJob 战斗伤害分配，由对局循环去点 Done。
type AssignDamageJob struct{}

// ModalChoiceKind 结算时竖排 / Wardens 横排 Choose One。
type ModalChoiceKind string

const (
	ModalChoiceLast            ModalChoiceKind = "last"              // 竖排最底一项（通常是失去生命）
	ModalChoiceWardensGainLife ModalChoiceKind = "wardens_gain_life" // 左：获得 2 生命
	ModalChoiceWardensDraw     ModalChoiceKind = "wardens_draw"      // 右：抓牌并失去 1 生命
)

// ModalChoiceJob 结算异能 Choose One（SelectN PromptParameterIndex）。
type ModalChoiceJob struct {
	Kind     ModalChoiceKind
	NOptions int
	SourceID int
	Life     int
	Reason   string
}

// Snapshot 是决策器看到的当前局面（不可变视图约定）。
type Snapshot struct {
	MatchID                string
	SystemSeatID           int
	Turn                   TurnInfo
	Objects                []Object
	Actions                []Action
	PendingMsgCount        int
	HasMulledKeep          bool
	GameStateComplete      bool
	NeedsTargetSelect      bool // SelectTargetsReq 或 AAR 含 Target 动作
	NeedsCastingTimeOption bool // 中屏 Choose One（kicker / 模式 / 额外费用）仍挡住点击
	NeedsGroupReq          bool // Scry / Surveil 等 GroupReq 仍挡住点击
	NeedsAssignDamage      bool // AssignDamageReq 伤害分配 Done
	NeedsModalChoice       bool // 结算异能竖排/横排 Choose One
	SelectTarget           *SelectTargetPrompt
	SelectN                *SelectNPrompt
	AttackTargetRequired   bool // 对方鹏洛克：攻击者需指定打脸/鹏洛克
	AttackerIDs            []int
}

// NeedsSelectN 是否有弃牌/选 N 张挂起。
func (s Snapshot) NeedsSelectN() bool {
	return s.SelectN != nil && len(s.SelectN.IDs) > 0
}

// NeedsSpellTarget 当前动作列表是否要求选非法术/异能目标（非攻击）。
func (s Snapshot) NeedsSpellTarget() bool {
	if s.NeedsTargetSelect {
		return true
	}
	return actionsNeedSpellTarget(s.Actions)
}

// OurHandInstanceIDs 返回本方手牌 instanceId。
func (s Snapshot) OurHandInstanceIDs(handZoneIDs map[int]struct{}) []int {
	out := make([]int, 0)
	for _, o := range s.Objects {
		if o.ControllerSeatID != s.SystemSeatID {
			continue
		}
		if _, ok := handZoneIDs[o.ZoneID]; ok {
			out = append(out, o.InstanceID)
		}
	}
	return out
}

// IsOurDecision 是否轮到本方决策。
func (s Snapshot) IsOurDecision() bool {
	if s.SystemSeatID <= 0 {
		return false
	}
	if s.NeedsTargetSelect || s.NeedsSelectN() || s.NeedsCastingTimeOption || s.NeedsGroupReq || s.NeedsAssignDamage || s.NeedsModalChoice {
		return true
	}
	if s.Turn.DecisionPlayer == s.SystemSeatID {
		return true
	}
	// turnInfo 滞后时：本方 AAR 已在 Actions 里也视为可决策
	for _, a := range s.Actions {
		if a.SeatID == s.SystemSeatID || a.SeatID == 0 {
			return true
		}
	}
	return false
}
