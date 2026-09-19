package playerlog_test

import (
	"testing"

	"github.com/flourbrain/mtga-farm-bot/internal/playerlog"
)

func TestDetectBotStateInGame(t *testing.T) {
	tail := `GREMessageType_GameStateMessage {"gameStateId":1}`
	if got := playerlog.DetectBotState(tail); got != playerlog.StateInGame {
		t.Fatalf("got %s want IN_GAME", got)
	}
}

func TestDetectBotStateLeftMatch(t *testing.T) {
	tail := `GREMessageType_GameStateMessage
MatchGameRoomStateType_MatchCompleted
"toSceneName": "Home"`
	if got := playerlog.DetectBotState(tail); got != playerlog.StateHome {
		t.Fatalf("got %s want HOME", got)
	}
}

func TestScanMatchMarkers(t *testing.T) {
	m := playerlog.ScanMatchMarkers("GREMessageType_MulliganReq then GREMessageType_GameStateMessage")
	if !m.SawMulliganReq || !m.SawGameStateMessage {
		t.Fatalf("markers=%+v", m)
	}
}
