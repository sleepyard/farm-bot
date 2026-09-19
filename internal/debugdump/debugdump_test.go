package debugdump

import "testing"

func TestEnabledEnvOverride(t *testing.T) {
	t.Setenv("MTGA_BOT_DEBUG", "0")
	if Enabled() {
		t.Fatal("env 0 should disable")
	}
	t.Setenv("MTGA_BOT_DEBUG", "1")
	if !Enabled() {
		t.Fatal("env 1 should enable")
	}
}

func TestSafeNames(t *testing.T) {
	if MissName("Buttons/claim.png") != "miss_Buttons_claim.png" {
		t.Fatal(MissName("Buttons/claim.png"))
	}
	if ClickName(`assert\home_anchor.png`) != "click_assert_home_anchor.png" {
		t.Fatal(ClickName(`assert\home_anchor.png`))
	}
}

func TestSavePNGDisabled(t *testing.T) {
	t.Setenv("MTGA_BOT_DEBUG", "0")
	if SavePNG(nil, "x.png") != "" {
		t.Fatal("disabled must not write")
	}
}
