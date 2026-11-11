// Package gesture implements the pure decision logic behind the hotkey
// "hold-to-cycle" behaviour. It has no operating-system dependencies so it can
// be tested on any platform.
//
// Semantics for a hotkey like Alt+Space:
//
//   - Alt down, first Space   -> open the overlay, remember we are in a gesture.
//   - Alt still down, more Space -> step the selection down (wrapping).
//   - Alt up after stepping   -> commit: switch to the selected window.
//   - Alt up without stepping -> keep the overlay open (a plain tap).
//   - Space with Alt not held and overlay already open -> toggle it closed.
package gesture

// Action is what the caller should do in response to a key event.
type Action int

const (
	// None means nothing to do (or the event was ignored).
	None Action = iota
	// Open the overlay.
	Open
	// Cycle the selection one step down.
	Cycle
	// Close the overlay.
	Close
	// Commit: switch to the selected window.
	Commit
)

// State is the hold-to-cycle state machine.
type State struct {
	open    bool // overlay currently visible
	cycling bool // a hold-gesture is in progress
	moved   bool // the user stepped the selection during this gesture
	altHeld bool // physical Alt state
	wantAlt bool // the hotkey uses Alt
	latched bool // the main key is currently down (filters auto-repeat)
}

// New creates a State for a hotkey that may or may not use Alt.
func New(wantAlt bool) State { return State{wantAlt: wantAlt} }

// SetAlt records the physical Alt state.
func (s *State) SetAlt(down bool) { s.altHeld = down }

// AltHeld reports the tracked Alt state.
func (s *State) AltHeld() bool { return s.altHeld }

// AltReleased handles Alt going up and returns the action to take. It resets
// the gesture. A Commit is returned only if the user actually stepped.
func (s *State) AltReleased() Action {
	if !s.cycling {
		return None
	}
	s.cycling = false
	if s.moved {
		s.moved = false
		return Commit
	}
	// Plain tap: leave the overlay open for searching.
	return None
}

// KeyDown handles a press of the hotkey's main key while the modifiers match.
func (s *State) KeyDown() Action {
	if s.latched {
		return None // auto-repeat, ignore
	}
	s.latched = true

	holdAlt := s.altHeld && s.wantAlt
	switch {
	case !s.open:
		s.cycling = holdAlt
		s.moved = false
		s.open = true
		return Open
	case s.cycling || holdAlt:
		s.cycling = true
		s.moved = true
		return Cycle
	default:
		s.open = false
		s.cycling = false
		return Close
	}
}

// KeyUp handles the main key release. It always clears the latch so a missed
// event cannot wedge the hotkey. Returns true if the event was ours.
func (s *State) KeyUp() bool {
	stuck := s.latched
	s.latched = false
	return stuck
}

// SetOpen lets external code (Esc, picking a window) sync the visibility flag.
func (s *State) SetOpen(v bool) {
	s.open = v
	if !v {
		s.cycling = false
		s.moved = false
	}
}

// OverlayOpen reports whether the overlay is tracked as visible.
func (s *State) OverlayOpen() bool { return s.open }

// Cycling reports whether a hold-to-cycle gesture is in progress.
func (s *State) Cycling() bool { return s.cycling }
