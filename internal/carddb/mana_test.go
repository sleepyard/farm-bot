package carddb

import "testing"

func TestNormalizeManaCost(t *testing.T) {
	cases := map[string]string{
		"":      "",
		"2oW":   "{2}{W}",
		"oWoU":  "{W}{U}",
		"1oBoB": "{1}{B}{B}",
	}
	for in, want := range cases {
		if got := normalizeManaCost(in); got != want {
			t.Errorf("normalizeManaCost(%q)=%q want %q", in, got, want)
		}
	}
}

func TestLooksLikeRawDirMiss(t *testing.T) {
	if looksLikeRawDir(t.TempDir()) {
		t.Fatal("empty dir should not look like Raw")
	}
}
