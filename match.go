package main

import "strings"

// MatchResult carries a score plus the character ranges that matched so the UI
// can highlight them. Mirrors Switcheroo's matcher idea but flat and cheap.
type MatchResult struct {
	Score   int
	Ranges  [][2]int // byte offsets into the lowercased title
	Matched bool
}

// scoreTitle evaluates the query against title/process and returns a match
// result. Strategy, cheapest first:
//
//  1. prefix of the title          -> 1000 - len(title)
//  2. prefix of the process name   ->  900 - len(title)
//  3. substring in title           ->  800 - position
//  4. substring in process name    ->  700 - position
//  5. subsequence of title chars   ->  600 - gaps
//  6. subsequence of process name  ->  500 - gaps
//
// This gives "chrome" -> Chrome windows first, then things whose title contains
// it, and finally fuzzy character matches. Ties fall back to Z order.
func scoreTitle(query, title, process string) MatchResult {
	if query == "" {
		return MatchResult{Score: 1, Matched: true}
	}
	q := strings.ToLower(query)
	t := strings.ToLower(title)
	p := strings.ToLower(process)

	if strings.HasPrefix(t, q) {
		return MatchResult{Score: 1000 - len(t), Ranges: [][2]int{{0, len(q)}}, Matched: true}
	}
	if strings.HasPrefix(p, q) {
		return MatchResult{Score: 900 - len(t), Matched: true}
	}
	if i := strings.Index(t, q); i >= 0 {
		return MatchResult{Score: 800 - i, Ranges: [][2]int{{i, i + len(q)}}, Matched: true}
	}
	if i := strings.Index(p, q); i >= 0 {
		return MatchResult{Score: 700 - i, Matched: true}
	}
	if rng, gaps, ok := subsequence(t, q); ok {
		return MatchResult{Score: 600 - gaps, Ranges: rng, Matched: true}
	}
	if _, _, ok := subsequence(p, q); ok {
		return MatchResult{Score: 500, Matched: true}
	}
	return MatchResult{}
}

// subsequence finds the characters of q in order inside s and returns their
// ranges plus the total gap between them. It prefers the first occurrence.
func subsequence(s, q string) ([][2]int, int, bool) {
	if q == "" {
		return nil, 0, true
	}
	var ranges [][2]int
	gaps := 0
	pos := 0
	prevEnd := -1
	for _, rc := range q {
		i := strings.IndexRune(s[pos:], rc)
		if i < 0 {
			return nil, 0, false
		}
		start := pos + i
		end := start + len(string(rc))
		if prevEnd >= 0 {
			gaps += start - prevEnd
		}
		if n := len(ranges); n > 0 && ranges[n-1][1] == start {
			ranges[n-1][1] = end
		} else {
			ranges = append(ranges, [2]int{start, end})
		}
		prevEnd = end
		pos = end
	}
	return ranges, gaps, true
}
