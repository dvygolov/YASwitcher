package gesture

import "testing"

func press(s *State) Action {
	a := s.KeyDown()
	s.KeyUp()
	return a
}

func TestTapLeavesOverlayOpen(t *testing.T) {
	s := New(true)
	s.SetAlt(true)
	if a := press(&s); a != Open {
		t.Fatalf("first press: want Open, got %v", a)
	}
	s.SetAlt(false)
	if a := s.AltReleased(); a != None {
		t.Fatalf("alt release after tap: want None (stay open), got %v", a)
	}
}

func TestCycleThenReleaseCommits(t *testing.T) {
	s := New(true)
	s.SetAlt(true)
	if a := press(&s); a != Open {
		t.Fatalf("first press: want Open, got %v", a)
	}
	if a := press(&s); a != Cycle {
		t.Fatalf("second press: want Cycle, got %v", a)
	}
	if a := press(&s); a != Cycle {
		t.Fatalf("third press: want Cycle, got %v", a)
	}
	s.SetAlt(false)
	if a := s.AltReleased(); a != Commit {
		t.Fatalf("alt release after cycling: want Commit, got %v", a)
	}
}

func TestAutoRepeatIgnored(t *testing.T) {
	s := New(true)
	s.SetAlt(true)
	s.KeyDown() // open, latch set
	if a := s.KeyDown(); a != None {
		t.Fatalf("auto-repeat should be ignored, got %v", a)
	}
	s.KeyUp()
	if a := s.KeyDown(); a != Cycle {
		t.Fatalf("after release a new press should cycle, got %v", a)
	}
}

func TestToggleCloseWithoutAlt(t *testing.T) {
	s := New(true)
	s.SetOpen(true)
	s.SetAlt(false)
	if a := press(&s); a != Close {
		t.Fatalf("press with alt up on an open overlay should close, got %v", a)
	}
}

func TestLatchCannotWedge(t *testing.T) {
	s := New(true)
	s.SetAlt(true)
	s.KeyDown() // latch true, no keyUp yet
	if a := s.KeyDown(); a != None {
		t.Fatalf("latched press should be ignored, got %v", a)
	}
	if !s.KeyUp() {
		t.Fatal("keyUp should report it consumed the latch")
	}
	if a := s.KeyDown(); a == None {
		t.Fatal("after keyUp the hotkey must work again")
	}
}

func TestEscResetsGesture(t *testing.T) {
	s := New(true)
	s.SetAlt(true)
	press(&s)
	press(&s)
	s.SetOpen(false)
	if a := s.AltReleased(); a != None {
		t.Fatalf("after SetOpen(false) alt release must be None, got %v", a)
	}
}

func TestOpenWithoutAltStaysOpen(t *testing.T) {
	s := New(true)
	s.SetAlt(false)
	if a := press(&s); a != Open {
		t.Fatalf("open without alt: want Open, got %v", a)
	}
	if a := press(&s); a != Close {
		t.Fatalf("second press without alt on open overlay: want Close, got %v", a)
	}
}
