package assets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLangVariant(t *testing.T) {
	cases := []struct {
		rel, lang, want string
	}{
		{"Buttons/keep_hand.png", "cn", "Buttons/keep_hand.cn.png"},
		{"assert/starter_deck.PNG", "cn", "assert/starter_deck.cn.PNG"},
		{"assert/nav/nav_play_subtab.png", "jp", "assert/nav/nav_play_subtab.jp.png"},
	}
	for _, c := range cases {
		if got := LangVariant(c.rel, c.lang); got != c.want {
			t.Errorf("LangVariant(%q,%q) = %q, want %q", c.rel, c.lang, got, c.want)
		}
	}
}

// chdirToTempRoot points RootDir at a fresh temp assets root for the test.
func chdirToTempRoot(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	root := filepath.Join(tmp, "assets")
	if err := os.MkdirAll(filepath.Join(root, "assert"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "assert", "home_anchor.png"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(tmp)
	return root
}

func TestExistingVariants(t *testing.T) {
	root := chdirToTempRoot(t)
	rel := "Buttons/keep_hand.png"
	orig := filepath.Join(root, "Buttons", "keep_hand.png")

	// lang "en" only ever returns the original.
	if got := ExistingVariants(rel, "en"); len(got) != 1 || got[0] != orig {
		t.Fatalf("en got %v, want [%s]", got, orig)
	}

	// Variant missing on disk: only the original is a candidate.
	if got := ExistingVariants(rel, "cn"); len(got) != 1 || got[0] != orig {
		t.Fatalf("cn missing got %v, want [%s]", got, orig)
	}

	// Variant present: it comes first, original second.
	variant := filepath.Join(root, "Buttons", "keep_hand.cn.png")
	if err := os.MkdirAll(filepath.Dir(variant), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(variant, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ExistingVariants(rel, "cn")
	if len(got) != 2 || got[0] != variant || got[1] != orig {
		t.Fatalf("cn present got %v, want [%s %s]", got, variant, orig)
	}
}

func TestTemplateLangDefaultAndSet(t *testing.T) {
	if TemplateLang() != "en" {
		t.Fatalf("default got %q, want en", TemplateLang())
	}
	SetTemplateLang("cn")
	defer SetTemplateLang("en")
	if TemplateLang() != "cn" {
		t.Fatalf("after set got %q, want cn", TemplateLang())
	}
	SetTemplateLang("")
	if TemplateLang() != "en" {
		t.Fatalf("empty reset got %q, want en", TemplateLang())
	}
}

func TestCanonicalPathAllowsCNVariant(t *testing.T) {
	if got := canonicalPath("Buttons/keep_hand.cn.png"); got != "Buttons/keep_hand.cn.png" {
		t.Fatalf("cn variant got %q", got)
	}
	// Upper-case extension in the catalog keeps its casing.
	if got := canonicalPath("assert/starter_deck.cn.PNG"); got != "assert/starter_deck.cn.PNG" {
		t.Fatalf("cn variant (.PNG) got %q", got)
	}
	if got := canonicalPath("Buttons/keep_hand.jp.png"); got != "" {
		t.Fatalf("other lang should be rejected, got %q", got)
	}
	if got := canonicalPath("Buttons/not_in_catalog.cn.png"); got != "" {
		t.Fatalf("non-required variant should be rejected, got %q", got)
	}
}
