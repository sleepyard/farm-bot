package carddb

import "strings"

// 基础地 titleId / 英文名 → 法力色（与 Python CardInfo 对齐）。
var basicLandTitleColor = map[int]string{
	647:  "green", // Forest
	648:  "white", // Plains
	652:  "blue",  // Island
	653:  "black", // Swamp
	1250: "red",   // Mountain
}

var basicLandNameColor = map[string]string{
	"Plains":   "white",
	"Island":   "blue",
	"Swamp":    "black",
	"Mountain": "red",
	"Forest":   "green",
}

// ManaAbilityColor abilityGrpId → 色（Activate_Mana）。
var ManaAbilityColor = map[int]string{
	1001: "white",
	1002: "blue",
	1003: "black",
	1004: "red",
	1005: "green",
}

// LandProducedColors 推断地牌可产生的颜色（基础地 + card.Colors；未知则空）。
func LandProducedColors(grpID int) map[string]struct{} {
	out := map[string]struct{}{}
	c, ok := Global().Get(grpID)
	if !ok {
		return out
	}
	if col, ok := basicLandTitleColor[c.TitleID]; ok {
		out[col] = struct{}{}
		return out
	}
	if col, ok := basicLandNameColor[c.Name]; ok {
		out[col] = struct{}{}
		return out
	}
	for _, col := range c.Colors {
		if n := normalizeCardColor(col); n != "" {
			out[n] = struct{}{}
		}
	}
	return out
}

func normalizeCardColor(c string) string {
	switch strings.ToUpper(strings.TrimSpace(c)) {
	case "W", "WHITE":
		return "white"
	case "U", "BLUE":
		return "blue"
	case "B", "BLACK":
		return "black"
	case "R", "RED":
		return "red"
	case "G", "GREEN":
		return "green"
	default:
		return ""
	}
}

// IsCreature grpId 是否生物。
func IsCreature(grpID int) bool {
	c, ok := Global().Get(grpID)
	if !ok {
		return false
	}
	for _, t := range c.Types {
		if strings.EqualFold(t, "Creature") {
			return true
		}
	}
	return false
}

// CMCFromCostString 解析 "{2}{W}" 形式 CMC（无 X）。
func CMCFromCostString(cost string) int {
	cost = strings.TrimSpace(cost)
	if cost == "" {
		return 0
	}
	total := 0
	for _, part := range strings.Split(cost, "}") {
		part = strings.TrimPrefix(strings.TrimSpace(part), "{")
		if part == "" || part == "X" {
			continue
		}
		switch part {
		case "W", "U", "B", "R", "G", "C", "S":
			total++
		default:
			n := 0
			for _, ch := range part {
				if ch >= '0' && ch <= '9' {
					n = n*10 + int(ch-'0')
				} else {
					n = -1
					break
				}
			}
			if n >= 0 {
				total += n
			} else {
				total++ // hybrid etc.
			}
		}
	}
	return total
}

// CardCMC 查库 CMC；未知返回 99。
func CardCMC(grpID int) int {
	db := Global()
	if db == nil {
		return 99
	}
	c, ok := db.Get(grpID)
	if !ok || c.ManaCost == "" {
		return 99
	}
	return CMCFromCostString(c.ManaCost)
}
