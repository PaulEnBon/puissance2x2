package main

import (
	"power4"
	"testing"
)

func TestWinnerHorizontal(t *testing.T) {
	var b [6][7]string
	b[5][0] = "R"
	b[5][1] = "R"
	b[5][2] = "R"
	b[5][3] = "R"
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
