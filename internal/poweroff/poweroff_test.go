package poweroff

import (
	"strings"
	"testing"
)

func TestScheduleArgsClampAndComment(t *testing.T) {
	args := scheduleArgs(0, "  15 wins done  ")
	if len(args) < 4 || args[0] != "shutdown" || args[1] != "/s" || args[2] != "/t" {
		t.Fatalf("args=%v", args)
	}
	if args[3] != "30" {
		t.Fatalf("min delay want 30 got %s", args[3])
	}
	if args[4] != "/c" || args[5] != "15 wins done" {
		t.Fatalf("comment args=%v", args)
	}
	long := strings.Repeat("x", 600)
	args = scheduleArgs(DefaultDelaySec, long)
	if len(args[5]) != 500 {
		t.Fatalf("comment cap got %d", len(args[5]))
	}
}

func TestScheduleUsesRunnerOnWindowsOnly(t *testing.T) {
	var got []string
	old := runFn
	runFn = func(args []string) bool {
		got = append([]string(nil), args...)
		return true
	}
	t.Cleanup(func() { runFn = old })

	ok := Schedule(DefaultDelaySec, "MTGA Farm Bot")
	if Supported() {
		if !ok {
			t.Fatal("expected schedule ok")
		}
		if len(got) < 4 || got[3] != "120" {
			t.Fatalf("got=%v", got)
		}
	} else if ok || got != nil {
		t.Fatal("non-windows must be a no-op")
	}
}

func TestCancelHint(t *testing.T) {
	if CancelHint() != "shutdown /a" {
		t.Fatal(CancelHint())
	}
}
