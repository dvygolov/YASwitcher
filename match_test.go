//go:build windows

package main

import "testing"

func TestScoreTitle(t *testing.T) {
	cases := []struct {
		q, title, proc string
		wantMatched    bool
		minScore       int
	}{
		{"", "anything", "code", true, 1},
		{"chrome", "Chrome — GitHub", "chrome", true, 900},
		{"code", "Visual Studio Code", "code", true, 900},
		{"git", "Chrome — GitHub", "chrome", true, 800},
		{"zzz", "Chrome", "chrome", false, 0},
		{"chr", "Chrome", "chrome", true, 1000},
	}
	for _, c := range cases {
		got := scoreTitle(c.q, c.title, c.proc)
		if got.Matched != c.wantMatched {
			t.Fatalf("scoreTitle(%q,%q,%q) matched=%v want %v", c.q, c.title, c.proc, got.Matched, c.wantMatched)
		}
		if got.Matched && got.Score < c.minScore {
			t.Fatalf("scoreTitle(%q,%q,%q) score=%d want >=%d", c.q, c.title, c.proc, got.Score, c.minScore)
		}
	}
}

func TestSubsequence(t *testing.T) {
	if _, _, ok := subsequence("visual studio code", "vsc"); !ok {
		t.Fatal("expected vsc to match visual studio code")
	}
	if _, _, ok := subsequence("chrome", "xyz"); ok {
		t.Fatal("expected xyz not to match chrome")
	}
}
