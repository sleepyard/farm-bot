// Package decisioner 根据局面给出下一步动作（不碰鼠标）。
//
// 按卡牌关注点拆分文件：
//
//	decide.go   — NextMove 总调度
//	keep.go     — 起手保留 / 再调度
//	land.go     — 用地
//	cast.go     — 施放结界 / 生物 / 法术
//	removal.go  — 清除类
//	counter.go  — 反击
//	fight.go    — 互斗
//	combat.go   — 攻击 / 阻挡
//	activate.go — 启动式异能
//	policy.go   — 牌表策略 / 图例规则等
package decisioner

import (
	"fmt"

	"github.com/flourbrain/mtga-farm-bot/internal/carddb"
	"github.com/flourbrain/mtga-farm-bot/internal/gamestate"
)

// Kind 动作种类。
type Kind string

const (
	KindKeep         Kind = "keep"
	KindMulligan     Kind = "mulligan"
	KindCast         Kind = "cast"
	KindPlayLand     Kind = "play_land"
	KindActivate     Kind = "activate"
	KindAllAttack    Kind = "all_attack"
	KindNoAttacks    Kind = "no_attacks"
	KindNoBlocks     Kind = "no_blocks"
	KindSelectTarget Kind = "select_target"
	KindSelectN      Kind = "select_n" // 弃牌 / 选 N 张手牌
	KindResolve      Kind = "resolve"  // 过优先权 / 点右下角
	KindWait         Kind = "wait"     // 暂不行动
)

// Move 是决策器输出、控制器执行的指令。
type Move struct {
	Kind           Kind
	InstanceID     int    // cast / play_land / activate
	GrpID          int    // 对应 grpId，日志查牌名
	CardName       string // 英文牌名（可空）
	Targets        []int  // select_target 等；-1=对手头像
	FriendlyTarget bool   // select_target：true=扫己方战场
	Chooser        bool   // 中屏墓地/放逐选牌（Zombify）
	Stack          bool   // 堆叠法术（反击目标）
	Reason         string
}

// InstanceTag 日志用：instance=219||name=Plains；无 instance 时返回空串。
func (m Move) InstanceTag() string {
	if m.InstanceID <= 0 {
		return ""
	}
	name := m.CardName
	if name == "" {
		name = carddb.NameByGrp(m.GrpID)
	}
	if name == "" {
		name = "?"
	}
	return fmt.Sprintf("instance=%d||name=%s", m.InstanceID, name)
}

// Engine 持有跨回合的决策状态（本回合是否已用地等）。
type Engine struct {
	landPlayedThisTurn bool
	currentTurn        int
	attackAllTries     int    // 本段宣告攻击里已点全攻次数
	mode               string // historic | starter；影响阻挡/选目标/出牌优先级
}

// New 创建决策引擎（默认无模式特判，行为同 starter 完整逻辑）。
func New() *Engine {
	return &Engine{}
}

// NewWithMode 按排队模式创建引擎（史迹：不阻挡、选目标一律打脸）。
func NewWithMode(mode string) *Engine {
	return &Engine{mode: mode}
}

func (e *Engine) isHistoric() bool {
	return e != nil && e.mode == "historic"
}

// NextMove 根据快照给出下一步。
func (e *Engine) NextMove(snap gamestate.Snapshot) Move {
	if !snap.HasMulledKeep {
		return Move{Kind: KindWait, Reason: "尚未保留起手"}
	}
	if !snap.IsOurDecision() {
		return Move{Kind: KindWait, Reason: "非本方决策"}
	}
	e.noteTurn(snap.Turn.TurnNumber)
	if snap.Turn.Step != gamestate.StepDeclareAttack {
		e.attackAllTries = 0
	}

	if snap.NeedsCastingTimeOption {
		return Move{Kind: KindWait, Reason: "Choose One 对话框仍打开"}
	}
	if snap.NeedsGroupReq {
		return Move{Kind: KindWait, Reason: "Scry/Surveil 对话框仍打开"}
	}
	if snap.NeedsAssignDamage {
		return Move{Kind: KindWait, Reason: "伤害分配对话框仍打开"}
	}

	// pending 常会粘在 >0；已有 Play/Cast 或战斗步骤时照常行动。
	hasPlayCast := len(snap.PlayOrCastActions()) > 0
	inCombatPrompt := snap.Turn.Step == gamestate.StepDeclareAttack ||
		snap.Turn.Step == gamestate.StepDeclareBlock

	if m := e.tryCounter(snap); m != nil {
		return *m
	}
	if m := e.trySelectN(snap); m != nil {
		return *m
	}
	if m := e.trySelectTarget(snap); m != nil {
		return *m
	}
	if m := e.tryCombat(snap); m != nil {
		return *m
	}
	if m := e.tryPlayLand(snap); m != nil {
		return *m
	}
	if m := e.tryCast(snap); m != nil {
		return *m
	}
	if m := e.tryActivate(snap); m != nil {
		return *m
	}
	if snap.PendingMsgCount > 0 && !hasPlayCast && !inCombatPrompt && !snap.NeedsTargetSelect && !snap.NeedsSelectN() {
		return Move{Kind: KindWait, Reason: "pending>0 且无 play/cast"}
	}
	return Move{Kind: KindResolve, Reason: "无更优动作，过优先权"}
}

func (e *Engine) noteTurn(turn int) {
	if turn > e.currentTurn {
		e.currentTurn = turn
		e.landPlayedThisTurn = false
		e.attackAllTries = 0
	}
}
