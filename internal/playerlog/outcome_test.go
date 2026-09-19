package playerlog

import "testing"

func TestInferMatchWonByTeamID(t *testing.T) {
	chunk := `{"finalMatchResult":[{"result":"ResultType_WinLoss","winningTeamId":1}]}`
	won, ok := InferMatchWon(chunk, 1, 1)
	if !ok || !won {
		t.Fatalf("want win, got won=%v ok=%v", won, ok)
	}
	won, ok = InferMatchWon(chunk, 2, 2)
	if !ok || won {
		t.Fatalf("want loss, got won=%v ok=%v", won, ok)
	}
}

func TestInferMatchWonKeywords(t *testing.T) {
	won, ok := InferMatchWon("MatchCompleted victory screen", 0, 0)
	if !ok || !won {
		t.Fatalf("victory: won=%v ok=%v", won, ok)
	}
	won, ok = InferMatchWon("MatchCompleted defeat", 0, 0)
	if !ok || won {
		t.Fatalf("defeat: won=%v ok=%v", won, ok)
	}
	if _, ok := InferMatchWon("winningTeamId field present without number context", 0, 0); ok {
		t.Fatal("should not match winningTeamId as win keyword")
	}
}
