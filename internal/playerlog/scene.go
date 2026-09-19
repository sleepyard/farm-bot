package playerlog

import (
	"regexp"
	"strings"
)

// BotState 导航 / 对局粗状态（对齐 Python state_machine.BotState）。
type BotState string

const (
	StateHome      BotState = "HOME"
	StatePlayMenu  BotState = "PLAY_MENU"
	StateFindMatch BotState = "FIND_MATCH"
	StateHistoric  BotState = "HISTORIC"
	StateMyDecks   BotState = "MY_DECKS"
	StateInGame    BotState = "IN_GAME"
	StateOptions   BotState = "OPTIONS"
	StateStore     BotState = "STORE"
	StateUnknown   BotState = "UNKNOWN"
)

var sceneMap = []struct {
	key   string
	state BotState
}{
	{"home", StateHome},
	{"frontdoor", StateHome},
	{"mainmenu", StateHome},
	{"playblade", StatePlayMenu},
	{"play", StatePlayMenu},
	{"matchmaking", StateFindMatch},
	{"decks", StateMyDecks},
	{"collection", StateMyDecks},
	{"store", StateStore},
	{"options", StateOptions},
}

var reSceneName = regexp.MustCompile(`"toSceneName"\s*:\s*"([^"]+)"`)

// DetectBotState 根据 Player.log 尾部推断粗状态。
func DetectBotState(logTail string) BotState {
	text := logTail
	if text == "" {
		return StateUnknown
	}
	lowered := strings.ToLower(text)

	gsmPos := strings.LastIndex(lowered, "gremessagetype_gamestatemessage")
	sceneMatches := reSceneName.FindAllStringSubmatchIndex(text, -1)
	leftPos := maxInt(
		strings.LastIndex(lowered, "mainnav load in"),
		strings.LastIndex(lowered, "matchgameroomstatetype_matchcompleted"),
	)
	if len(sceneMatches) > 0 {
		last := sceneMatches[len(sceneMatches)-1]
		if last[0] > leftPos {
			leftPos = last[0]
		}
	}
	if gsmPos != -1 && gsmPos > leftPos {
		return StateInGame
	}

	cutoff := -1
	if len(sceneMatches) > 0 {
		last := sceneMatches[len(sceneMatches)-1]
		cutoff = last[0]
		scene := strings.ToLower(text[last[2]:last[3]])
		for _, m := range sceneMap {
			if strings.Contains(scene, m.key) {
				return m.state
			}
		}
	}

	type cand struct {
		pos   int
		state BotState
	}
	var cands []cand
	for _, pair := range []struct {
		needle string
		state  BotState
	}{
		{"mainnav load in", StateHome},
		{"my decks", StateMyDecks},
		{"find match", StateFindMatch},
		{"historic", StateHistoric},
	} {
		pos := strings.LastIndex(lowered, pair.needle)
		if pos > cutoff {
			cands = append(cands, cand{pos, pair.state})
		}
	}
	if len(cands) == 0 {
		return StateUnknown
	}
	best := cands[0]
	for _, c := range cands[1:] {
		if c.pos > best.pos {
			best = c
		}
	}
	return best.state
}

// MatchMarkers 从增量日志中提取对局生命周期事件。
type MatchMarkers struct {
	SawGameStateMessage bool
	SawMulliganReq      bool
	SawMatchCompleted   bool
	SawMulliganAccept   bool
}

// ScanMatchMarkers 扫描一段日志增量。
func ScanMatchMarkers(chunk string) MatchMarkers {
	low := strings.ToLower(chunk)
	return MatchMarkers{
		SawGameStateMessage: strings.Contains(low, "gremessagetype_gamestatemessage"),
		SawMulliganReq:      strings.Contains(low, "gremessagetype_mulliganreq"),
		SawMatchCompleted:   strings.Contains(low, "matchgameroomstatetype_matchcompleted"),
		SawMulliganAccept: strings.Contains(low, "mulliganoption_accepthand") ||
			strings.Contains(low, "clientmessagetype_mulliganresp"),
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
