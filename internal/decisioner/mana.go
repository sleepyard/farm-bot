package decisioner

import (
	"github.com/flourbrain/mtga-farm-bot/internal/carddb"
	"github.com/flourbrain/mtga-farm-bot/internal/gamestate"
)

var greColorToName = map[string]string{
	"ManaColor_White":   "white",
	"ManaColor_Blue":    "blue",
	"ManaColor_Black":   "black",
	"ManaColor_Red":     "red",
	"ManaColor_Green":   "green",
	"ManaColor_Generic": "generic",
}

var allColors = []string{"white", "blue", "black", "red", "green"}

// ManaPool 来自当前 AAR 里的 ActionType_Activate_Mana（对齐 Python DummyAI）。
type ManaPool struct {
	Colors  map[string]struct{} // 可用颜色并集
	Total   int                 // 来源数（= 可支付总法力）
	Sources []map[string]struct{}
}

// AvailableMana 统计本方可激活法力。
func AvailableMana(actions []gamestate.Action) ManaPool {
	byInst := map[int]map[string]struct{}{}
	for _, a := range actions {
		if a.ActionType != gamestate.ActionActivateMana || a.InstanceID == 0 {
			continue
		}
		src, ok := byInst[a.InstanceID]
		if !ok {
			src = map[string]struct{}{}
			byInst[a.InstanceID] = src
		}
		if col, ok := carddb.ManaAbilityColor[a.AbilityGrpID]; ok {
			src[col] = struct{}{}
			continue
		}
		for c := range carddb.LandProducedColors(a.GrpID) {
			src[c] = struct{}{}
		}
	}
	pool := ManaPool{Colors: map[string]struct{}{}}
	for _, src := range byInst {
		if len(src) == 0 {
			for _, c := range allColors {
				src[c] = struct{}{}
			}
		}
		cp := map[string]struct{}{}
		for c := range src {
			cp[c] = struct{}{}
			pool.Colors[c] = struct{}{}
		}
		pool.Sources = append(pool.Sources, cp)
	}
	pool.Total = len(pool.Sources)
	return pool
}

// ManaCostTotal 费用总点数。
func ManaCostTotal(cost []gamestate.ManaPip) int {
	n := 0
	for _, p := range cost {
		n += p.Count
	}
	return n
}

// CanAfford 是否付得起（色约束 + 总量），对齐 Python _can_cast_with_mana_costs。
func CanAfford(cost []gamestate.ManaPip, pool ManaPool) bool {
	if len(cost) == 0 {
		return true
	}
	totalNeeded := ManaCostTotal(cost)
	if pool.Total < totalNeeded {
		return false
	}

	var coloredReqs []map[string]struct{}
	for _, pip := range cost {
		opts := map[string]struct{}{}
		generic := false
		for _, c := range pip.Colors {
			name := greColorToName[c]
			if name == "" {
				name = c
			}
			if name == "generic" {
				generic = true
				break
			}
			opts[name] = struct{}{}
		}
		if generic || len(opts) == 0 {
			continue
		}
		for i := 0; i < pip.Count; i++ {
			cp := map[string]struct{}{}
			for k := range opts {
				cp[k] = struct{}{}
			}
			coloredReqs = append(coloredReqs, cp)
		}
	}
	if len(coloredReqs) == 0 {
		return true
	}
	for _, req := range coloredReqs {
		ok := false
		for c := range req {
			if _, has := pool.Colors[c]; has {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if len(coloredReqs) > len(pool.Sources) {
		return false
	}

	sources := make([]map[string]struct{}, len(pool.Sources))
	copy(sources, pool.Sources)
	used := make([]bool, len(sources))
	if !assignColored(coloredReqs, sources, used) {
		return false
	}
	genericNeeded := totalNeeded - len(coloredReqs)
	remaining := pool.Total - len(coloredReqs)
	return remaining >= genericNeeded
}

func assignColored(reqs []map[string]struct{}, sources []map[string]struct{}, used []bool) bool {
	if len(reqs) == 0 {
		return true
	}
	best := 0
	bestN := 1 << 30
	cands := make([][]int, len(reqs))
	for i, req := range reqs {
		for j, src := range sources {
			if used[j] {
				continue
			}
			for c := range req {
				if _, ok := src[c]; ok {
					cands[i] = append(cands[i], j)
					break
				}
			}
		}
		if n := len(cands[i]); n < bestN {
			bestN = n
			best = i
		}
	}
	if bestN == 0 {
		return false
	}
	for _, srcIdx := range cands[best] {
		used[srcIdx] = true
		rest := append(append([]map[string]struct{}{}, reqs[:best]...), reqs[best+1:]...)
		if assignColored(rest, sources, used) {
			return true
		}
		used[srcIdx] = false
	}
	return false
}

func withSimLand(pool ManaPool, produced map[string]struct{}) ManaPool {
	out := ManaPool{
		Colors:  map[string]struct{}{},
		Total:   pool.Total + 1,
		Sources: make([]map[string]struct{}, 0, len(pool.Sources)+1),
	}
	for c := range pool.Colors {
		out.Colors[c] = struct{}{}
	}
	for _, s := range pool.Sources {
		cp := map[string]struct{}{}
		for c := range s {
			cp[c] = struct{}{}
		}
		out.Sources = append(out.Sources, cp)
	}
	src := map[string]struct{}{}
	if len(produced) == 0 {
		for _, c := range allColors {
			src[c] = struct{}{}
			out.Colors[c] = struct{}{}
		}
	} else {
		for c := range produced {
			src[c] = struct{}{}
			out.Colors[c] = struct{}{}
		}
	}
	out.Sources = append(out.Sources, src)
	return out
}
