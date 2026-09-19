package appstate

import "testing"

func TestNormalizeAndLabel(t *testing.T) {
	if Normalize("starter") != ModeStarter {
		t.Fatal("starter")
	}
	if Normalize("historic") != ModeHistoric {
		t.Fatal("historic")
	}
	if Normalize("nope") != ModeHistoric {
		t.Fatal("fallback")
	}
	if ModeLabel(ModeStarter) != "新手套牌对决（每日任务）" {
		t.Fatal("starter label")
	}
	if ModeLabel(ModeHistoric) != "史迹（15胜）" {
		t.Fatal("historic label")
	}
}

func TestToggle(t *testing.T) {
	s := NewStore()
	if s.Mode() != ModeHistoric {
		t.Fatal("default")
	}
	if s.Toggle() != ModeStarter {
		t.Fatal("to starter")
	}
	if s.Toggle() != ModeHistoric {
		t.Fatal("to historic")
	}
}

func TestAutoSwitch(t *testing.T) {
	s := NewStore()
	if s.AutoSwitch() {
		t.Fatal("default off")
	}
	s.SetAutoSwitch(true)
	if !s.AutoSwitch() {
		t.Fatal("on")
	}
	if AutoSwitchTarget(ModeStarter, true, true) != ModeHistoric {
		t.Fatal("should switch")
	}
	if AutoSwitchTarget(ModeStarter, true, false) != ModeStarter {
		t.Fatal("quests remaining")
	}
	if AutoSwitchTarget(ModeStarter, false, true) != ModeStarter {
		t.Fatal("flag off")
	}
	if AutoSwitchTarget(ModeHistoric, true, true) != ModeHistoric {
		t.Fatal("already historic")
	}
}

func TestShutdownAfterWins(t *testing.T) {
	s := NewStore()
	if s.ShutdownAfterWins() {
		t.Fatal("default off")
	}
	s.SetShutdownAfterWins(true)
	if !s.ShutdownAfterWins() {
		t.Fatal("on")
	}
	s.SetShutdownAfterWins(false)
	if s.ShutdownAfterWins() {
		t.Fatal("off")
	}
}
