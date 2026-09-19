package playerlog

import (
	"regexp"
	"strconv"
	"strings"
)

var reWinningTeam = regexp.MustCompile(`"winningTeamId"\s*:\s*(\d+)`)

// InferMatchWon 从日志片段推断胜负。优先 GRE winningTeamId，否则用 victory/defeat 关键词。
func InferMatchWon(chunk string, mySeat, myTeam int) (won bool, ok bool) {
	if chunk == "" {
		return false, false
	}
	if m := reWinningTeam.FindStringSubmatch(chunk); len(m) == 2 {
		wt, err := strconv.Atoi(m[1])
		if err == nil && wt != 0 {
			ref := myTeam
			if ref == 0 {
				ref = mySeat
			}
			if ref != 0 {
				return wt == ref, true
			}
		}
	}
	return inferByKeywords(chunk)
}

func inferByKeywords(text string) (won bool, ok bool) {
	low := strings.ToLower(text)
	hasWin := containsWord(low, "victory") || containsWord(low, "won")
	hasLoss := containsWord(low, "defeat") || containsWord(low, "loss") || containsWord(low, "lost")
	if hasWin && !hasLoss {
		return true, true
	}
	if hasLoss && !hasWin {
		return false, true
	}
	return false, false
}

func containsWord(s, word string) bool {
	i := 0
	for {
		j := strings.Index(s[i:], word)
		if j < 0 {
			return false
		}
		j += i
		beforeOK := j == 0 || !isAlpha(s[j-1])
		after := j + len(word)
		afterOK := after >= len(s) || !isAlpha(s[after])
		if beforeOK && afterOK {
			return true
		}
		i = j + 1
	}
}

func isAlpha(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}
