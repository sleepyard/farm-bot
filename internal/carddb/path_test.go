package carddb

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizePickedRawDirWalksDown(t *testing.T) {
	root := t.TempDir()
	raw := filepath.Join(root, "MTGA_Data", "Downloads", "Raw")
	if err := os.MkdirAll(raw, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(raw, "Raw_CardDatabase_test.mtga")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := NormalizePickedRawDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != raw {
		t.Fatalf("got %s want %s", got, raw)
	}
	got, err = NormalizePickedRawDir(raw)
	if err != nil || got != raw {
		t.Fatalf("raw itself: %s %v", got, err)
	}
}

func TestNormalizePickedRawDirRejectsEmpty(t *testing.T) {
	if _, err := NormalizePickedRawDir(t.TempDir()); err == nil {
		t.Fatal("empty folder should fail")
	}
}
