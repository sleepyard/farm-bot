package decisioner

import (
	"fmt"
	"strings"

	"github.com/flourbrain/mtga-farm-bot/internal/carddb"
	"github.com/flourbrain/mtga-farm-bot/internal/gamestate"
)

// trySelectN 弃牌 / 选 N：对齐 Python SelectNReq — 先选手牌再 submit。
func (e *Engine) trySelectN(snap gamestate.Snapshot) *Move {
	if !snap.NeedsSelectN() {
		return nil
	}
	prompt := snap.SelectN
	n := prompt.MinSel
	if n < 1 {
		n = 1
	}
	if n > len(prompt.IDs) {
		n = len(prompt.IDs)
	}
	picked := pickSelectNIDs(snap, prompt.IDs, n)
	if len(picked) == 0 {
		return nil
	}
	id := picked[0]
	grp := grpOf(snap, id)
	return &Move{
		Kind:       KindSelectN,
		InstanceID: id,
		GrpID:      grp,
		CardName:   carddb.NameByGrp(grp),
		Targets:    picked, // 若 MinSel>1，控制器可依次点
		Reason:     fmt.Sprintf("SelectN 选择 %d 张（弃牌/提示）", n),
	}
}

func grpOf(snap gamestate.Snapshot, instanceID int) int {
	for _, o := range snap.Objects {
		if o.InstanceID == instanceID {
			return o.GrpID
		}
	}
	return 0
}

// pickSelectNIDs 优先弃高费非地；地最后弃（农场向）。
func pickSelectNIDs(snap gamestate.Snapshot, ids []int, n int) []int {
	type scored struct {
		id    int
		score int
	}
	var list []scored
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		grp := grpOf(snap, id)
		score := carddb.CardCMC(grp)
		if score >= 99 {
			score = 1
		}
		if isLandGrp(grp) {
			score -= 50
		}
		list = append(list, scored{id: id, score: score})
	}
	if len(list) == 0 {
		return nil
	}
	// 降序：高分先弃
	for i := 0; i < len(list); i++ {
		for j := i + 1; j < len(list); j++ {
			if list[j].score > list[i].score {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	out := make([]int, 0, n)
	for _, s := range list {
		out = append(out, s.id)
		if len(out) >= n {
			break
		}
	}
	return out
}

func isLandGrp(grpID int) bool {
	c, ok := carddb.Global().Get(grpID)
	if !ok {
		return false
	}
	for _, t := range c.Types {
		if strings.EqualFold(t, "Land") {
			return true
		}
	}
	return false
}
