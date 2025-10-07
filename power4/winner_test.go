package power4_test

import (
	"power4/power4"
	"testing"
)

func TestWinnerHorizontal(t *testing.T) {
	var b [6][7]string
	b[5][0], b[5][1], b[5][2], b[5][3] = "R", "R", "R", "R"
	if w := power4.Winner(b); w != "R" {
		t.Fatalf("expected R, got %q", w)
	}
}

func TestWinnerNone(t *testing.T) {
	var b [6][7]string
	if w := power4.Winner(b); w != "" {
		t.Fatalf("expected no winner, got %q", w)
	}
}
