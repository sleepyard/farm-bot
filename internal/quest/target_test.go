package quest

import (
	"testing"

	"github.com/flourbrain/mtga-farm-bot/internal/playerlog"
)

func TestResolveTargetGuild(t *testing.T) {
	quests := []playerlog.Quest{
		{LocKey: "Quest_Orzhov_Play", Progress: 0, Goal: 4, RewardGold: 500},
		{LocKey: "Quest_Azorius_Play", Progress: 1, Goal: 4, RewardGold: 750},
	}
	got := ResolveTarget(quests)
	if got.Colors != "WU" {
		t.Fatalf("want WU (higher gold azorius), got %q reason=%s", got.Colors, got.Reason)
	}
}

func TestResolveTargetAllDone(t *testing.T) {
	quests := []playerlog.Quest{
		{LocKey: "Quest_Orzhov_Play", Progress: 4, Goal: 4, RewardGold: 500},
	}
	got := ResolveTarget(quests)
	if !got.AllDone || got.Colors != "WB" {
		t.Fatalf("want AllDone WB, got %+v", got)
	}
}

func TestResolveTargetEmptyNotAllDone(t *testing.T) {
	got := ResolveTarget(nil)
	if got.AllDone {
		t.Fatal("empty list is unknown data, not AllDone; auto-switch treats empty separately")
	}
}

func TestResolveTargetNoGuild(t *testing.T) {
	quests := []playerlog.Quest{
		{LocKey: "Quest_Win_Games", Progress: 0, Goal: 5, RewardGold: 500},
	}
	got := ResolveTarget(quests)
	if got.Colors != "" {
		t.Fatalf("want empty keep deck, got %q", got.Colors)
	}
}
