//go:build live

package historic_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/flourbrain/mtga-farm-bot/internal/historic"
	"github.com/flourbrain/mtga-farm-bot/internal/window"
)

func chdirModuleRoot(t *testing.T) {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	start := dir
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if err := os.Chdir(dir); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chdir(start) })
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

// 实机验证：go test ./internal/historic -tags=live -count=1 -v -run Live
// 要求：MTGA 可见、客户区 1280×720，最好停在 Home。
func TestLiveNavigateHistoricPlay(t *testing.T) {
	chdirModuleRoot(t)
	cap := window.CaptureMTGA()
	if !cap.OK || cap.Candidate == nil {
		t.Skip("MTGA 未就绪: ", cap.Message)
	}
	c := cap.Candidate
	wd, _ := os.Getwd()
	fmt.Fprintf(os.Stderr, "Live nav HWND=%d %dx%d cwd=%s\n", c.HWND, c.ClientRect.W, c.ClientRect.H, wd)
	res := historic.Navigate(historic.Session{
		HWND:   c.HWND,
		Width:  c.ClientRect.W,
		Height: c.ClientRect.H,
	}, historic.Hooks{
		Log: func(msg string) { fmt.Fprintln(os.Stderr, msg) },
	})
	for _, s := range res.Steps {
		t.Log(s)
	}
	if !res.OK {
		t.Fatalf("Navigate failed: %s", res.Message)
	}
}
