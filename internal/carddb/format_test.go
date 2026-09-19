package carddb

import "testing"

func TestFormatInstance(t *testing.T) {
	setGlobal(&DB{byGrp: map[int]Card{
		91301: {GrpID: 91301, Name: "Plains"},
	}})
	t.Cleanup(func() { setGlobal(nil) })

	got := FormatInstance(219, 91301)
	want := "instance=219||name=Plains"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if FormatInstance(1, 0) != "instance=1||name=?" {
		t.Fatalf("unknown: %q", FormatInstance(1, 0))
	}
}
