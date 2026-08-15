// Package dup detects copy-paste duplication between files using Navune's
// normalized token streams (ADR 0005). Blocks are identical token sequences of
// length >= minTokens shared between two different files (cross-file) or two
// distinct regions of the same file (intra-file). Identifiers and keywords are
// compared verbatim; string/number literals are compared by kind, so
// copy-paste with changed constants is still detected.
package dup

import (
	"sort"

	"github.com/lcr/navune/internal/lang"
)

// DefaultMinTokens is the minimum block length (in normalized tokens) treated
// as duplication. Configurable per run via Engine{MinTokens}.
const DefaultMinTokens = 20

// Result reports duplication across the analyzed (production) files.
type Result struct {
	// ByFile holds per-file duplicated-token counts, aligned with the input.
	ByFile []int
	// Blocks is the number of distinct maximal duplicated blocks found.
	Blocks int
	// TotalTokens is the sum of tokens across all input files.
	TotalTokens int
	// DupTokens is the sum of duplicated tokens across all input files.
	DupTokens int
}

// Engine detects duplicated token blocks.
type Engine struct {
	MinTokens int
}

// New returns an Engine with sane defaults.
func New() *Engine { return &Engine{MinTokens: DefaultMinTokens} }

// maxPairsPerBucket caps pairwise verification work inside a single hash
// bucket to keep pathological repetitive code bounded.
const maxPairsPerBucket = 5000

type position struct {
	file  int
	start int
}

// Detect finds duplicated blocks across the given production files.
// Files is the normalized token stream per file (in the same order as
// lang.FileResult producers); result aligns by that order.
func (e *Engine) Detect(files [][]lang.Token) *Result {
	wt := e.MinTokens
	if wt < 1 {
		wt = 1
	}
	n := len(files)
	ids := make([][]int, n) // token text -> stable integer id, per file
	textID := map[string]int{}
	total := 0
	for f := 0; f < n; f++ {
		ids[f] = make([]int, 0, len(files[f]))
		for _, tok := range files[f] {
			id, ok := textID[tok.Text]
			if !ok {
				id = len(textID) + 1
				textID[tok.Text] = id
			}
			ids[f] = append(ids[f], id)
		}
		total += len(ids[f])
	}

	covered := make([][]bool, n)
	for f := 0; f < n; f++ {
		covered[f] = make([]bool, len(ids[f]))
	}

	const base = 1000003
	pow := make([]uint64, wt+1)
	pow[0] = 1
	for i := 1; i <= wt; i++ {
		pow[i] = pow[i-1] * base
	}

	// Build buckets of equal-length windows by rolling hash.
	buckets := map[uint64][]position{}
	for f := 0; f < n; f++ {
		arr := ids[f]
		if len(arr) < wt {
			continue
		}
		var h uint64
		for i := 0; i < wt; i++ {
			h = h*base + uint64(arr[i])
		}
		buckets[h] = append(buckets[h], position{f, 0})
		for i := wt; i < len(arr); i++ {
			h = h*base + uint64(arr[i]) - uint64(arr[i-wt])*pow[wt]
			buckets[h] = append(buckets[h], position{f, i - wt + 1})
		}
	}

	blocks := 0
	// deterministic iteration over buckets
	keys := make([]uint64, 0, len(buckets))
	for h := range buckets {
		keys = append(keys, h)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, h := range keys {
		occ := buckets[h]
		if len(occ) < 2 {
			continue
		}
		sort.Slice(occ, func(i, j int) bool {
			if occ[i].file != occ[j].file {
				return occ[i].file < occ[j].file
			}
			return occ[i].start < occ[j].start
		})
		// pair only occurrences from different files, or the same file at
		// sufficiently distinct offsets (intra-file duplication).
		compared := 0
		for i := 0; i < len(occ); i++ {
			for j := i + 1; j < len(occ); j++ {
				compared++
				if compared > maxPairsPerBucket {
					break
				}
				pi, pj := occ[i], occ[j]
				if pi.file == pj.file && abs(pi.start-pj.start) < wt {
					continue // same region, overlapping windows
				}
				if windowsEqual(ids[pi.file], ids[pj.file], pi.start, pj.start, wt) {
					s1, e1, s2, e2 := extend(ids[pi.file], ids[pj.file], pi.start, pj.start, wt)
					if mark(covered[pi.file], s1, e1) {
						blocks++
					}
					mark(covered[pj.file], s2, e2)
				}
			}
		}
	}

	res := &Result{ByFile: make([]int, n), TotalTokens: total}
	dupTotal := 0
	for f := 0; f < n; f++ {
		c := 0
		for _, b := range covered[f] {
			if b {
				c++
			}
		}
		res.ByFile[f] = c
		dupTotal += c
	}
	res.DupTokens = dupTotal
	res.Blocks = blocks
	return res
}

// windowsEqual compares wt tokens starting at s1 in a and s2 in b.
func windowsEqual(a, b []int, s1, s2, wt int) bool {
	for k := 0; k < wt; k++ {
		if s1+k >= len(a) || s2+k >= len(b) || a[s1+k] != b[s2+k] {
			return false
		}
	}
	return true
}

// extend grows a verified equal window of length wt outward to the maximal
// common block and returns the token ranges [s,e) in both sequences.
func extend(a, b []int, s1, s2, wt int) (s1e, e1, s2e, e2 int) {
	s1e, s2e = s1, s2
	e1, e2 = s1+wt, s2+wt
	// grow right
	for e1 < len(a) && e2 < len(b) && a[e1] == b[e2] {
		e1++
		e2++
	}
	// grow left
	for s1e > 0 && s2e > 0 && a[s1e-1] == b[s2e-1] {
		s1e--
		s2e--
	}
	return s1e, e1, s2e, e2
}

// mark sets covered[start:end) and reports whether any new token was marked.
func mark(covered []bool, start, end int) bool {
	added := false
	if start < 0 {
		start = 0
	}
	if end > len(covered) {
		end = len(covered)
	}
	for i := start; i < end; i++ {
		if !covered[i] {
			covered[i] = true
			added = true
		}
	}
	return added
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
