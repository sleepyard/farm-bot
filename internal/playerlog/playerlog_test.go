package playerlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLatestQuests(t *testing.T) {
	line := `<== QuestGetQuests {"quests":[{"questId":"old","locKey":"Quests/Quest_Simic_Manipulator","goal":20,"endingProgress":3,"chestDescription":{"locParams":{"number1":500}}}],"canSwap":true}`
	qs, err := ParseLatestQuests(line)
	if err != nil || qs == nil {
		t.Fatalf("err=%v qs=%v", err, qs)
	}
	if qs.CanSwap == nil || !*qs.CanSwap {
		t.Fatal("canSwap should be true")
	}
	view := BuildQuestView(qs.Quests)
	if len(view) != 1 {
		t.Fatalf("len=%d", len(view))
	}
	q := view[0]
	if q.ID != "old" || q.Colors != "蓝绿" || q.RewardGold != 500 || q.Progress != 3 || q.Goal != 20 {
		t.Fatalf("unexpected quest: %+v", q)
	}
	if q.Name != "Simic Manipulator" || q.LocKey != "Quests/Quest_Simic_Manipulator" {
		t.Fatalf("name=%q loc=%q", q.Name, q.LocKey)
	}
}

func TestParseLatestQuestsCanSwapOmitted(t *testing.T) {
	line := `{"quests":[]}`
	qs, err := ParseLatestQuests(line)
	if err != nil || qs == nil {
		t.Fatalf("err=%v", err)
	}
	if qs.CanSwap != nil {
		t.Fatal("omitted canSwap should be nil")
	}
	if len(BuildQuestView(qs.Quests)) != 0 {
		t.Fatal("empty quests")
	}
}

func TestReaderTailAndSince(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Player.log")
	content := strings.Repeat("A", 100) +
		`{"quests":[{"questId":"q","locKey":"Quests/Quest_Gruul_Scrapper","goal":20,"endingProgress":0,"chestDescription":{"locParams":{"number1":500}}}],"canSwap":true}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &Reader{Path: path}
	text, size, err := r.Tail(DefaultTailBytes)
	if err != nil {
		t.Fatal(err)
	}
	if size != int64(len(content)) {
		t.Fatalf("size=%d", size)
	}
	if !strings.Contains(text, `"quests"`) {
		t.Fatalf("tail missing quests: %q", text)
	}
	since, err := r.Since(0, DefaultTailBytes, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(since, "canSwap") {
		t.Fatalf("since missing canSwap: %q", since)
	}
}

func TestSnapshotterCapture(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Player.log")
	body := `<== QuestGetQuests {"quests":[{"questId":"q1","locKey":"Quests/Quest_Orzhov_Something","goal":10,"endingProgress":1,"chestDescription":{"locParams":{"number1":750}}}],"canSwap":true}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	snap := (&Snapshotter{Reader: &Reader{Path: path}}).Capture()
	if !snap.OK {
		t.Fatalf("warning=%s", snap.Warning)
	}
	if snap.CanSwap == nil || !*snap.CanSwap {
		t.Fatal("canSwap should be true")
	}
	if len(snap.Quests) != 1 || snap.Quests[0].Colors != "黑白" {
		t.Fatalf("quests=%+v", snap.Quests)
	}
}

func TestDetectShrinkClearsFloor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Player.log")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 200)), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &Snapshotter{Reader: &Reader{Path: path}, Floor: 150}
	if err := os.WriteFile(path, []byte("small"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = s.Capture()
	if s.Floor != 0 {
		t.Fatalf("floor should clear on shrink, got %d", s.Floor)
	}
}

func TestQuestDisplayName(t *testing.T) {
	if got := questDisplayName("Quests/Quest_Play_Lands"); got != "Play Lands" {
		t.Fatalf("got %q", got)
	}
	if got := questDisplayName("Quests/Quest_Simic_Manipulator"); got != "Simic Manipulator" {
		t.Fatalf("got %q", got)
	}
}
