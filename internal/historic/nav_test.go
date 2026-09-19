package historic

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/flourbrain/mtga-farm-bot/internal/assets"
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
			t.Fatal("go.mod not found from ", start)
		}
		dir = parent
	}
}

func TestRequiredHistoricAssetsExist(t *testing.T) {
	chdirModuleRoot(t)
	rels := []string{
		"assert/home_anchor.png",
		"assert/find_match_anchor.png",
		"assert/nav/nav_historic_play.png",
		"assert/nav/nav_my_decks.png",
		"Buttons/play_btn.png",
		"Buttons/find_match_btn.png",
		"Buttons/my_decks_grid_open.png",
		"Buttons/claim.png",
	}
	root := assets.RootDir()
	for _, rel := range rels {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing asset %s under %s: %v", rel, root, err)
		}
	}
}

func TestFirstDeckPoint(t *testing.T) {
	// 1920 (456,552) → 1280 ×2/3
	if ptFirstDeck.X != 304 || ptFirstDeck.Y != 368 {
		t.Fatalf("first deck pt=%v want (304,368)", ptFirstDeck)
	}
	if ptQueuePlay.X != 1133 || ptQueuePlay.Y != 664 {
		t.Fatalf("queue play pt=%v want (1133,664)", ptQueuePlay)
	}
	if ptHomeTab.X != 69 || ptHomeTab.Y != 26 {
		t.Fatalf("home tab pt=%v want (69,26)", ptHomeTab)
	}
}

func TestROISanity(t *testing.T) {
	checks := []struct {
		name       string
		x, y, w, h int
	}{
		{"homeAnchor", roiHomeAnchor.X, roiHomeAnchor.Y, roiHomeAnchor.W, roiHomeAnchor.H},
		{"homePlay", roiHomePlay.X, roiHomePlay.Y, roiHomePlay.W, roiHomePlay.H},
		{"bladeTabs", roiBladeTabs.X, roiBladeTabs.Y, roiBladeTabs.W, roiBladeTabs.H},
		{"bladeContent", roiBladeContent.X, roiBladeContent.Y, roiBladeContent.W, roiBladeContent.H},
		{"playSubtab", roiPlaySubtab.X, roiPlaySubtab.Y, roiPlaySubtab.W, roiPlaySubtab.H},
		{"formatList", roiFormatList.X, roiFormatList.Y, roiFormatList.W, roiFormatList.H},
		{"decksHeader", roiDecksHeader.X, roiDecksHeader.Y, roiDecksHeader.W, roiDecksHeader.H},
		{"decksGrid", roiDecksGrid.X, roiDecksGrid.Y, roiDecksGrid.W, roiDecksGrid.H},
	}
	for _, c := range checks {
		if c.w <= 0 || c.h <= 0 {
			t.Fatalf("%s empty", c.name)
		}
		if c.x+c.w > 1280 || c.y+c.h > 720 {
			t.Fatalf("%s out of 1280x720: right=%d bottom=%d", c.name, c.x+c.w, c.y+c.h)
		}
	}
}
