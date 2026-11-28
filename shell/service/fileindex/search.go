package fileindex

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
)

// MinQuery is the shortest query, in bytes, Search looks through the
// whole index for; shorter ones match recent files only.
const MinQuery = 2

// checkEvery is how many files Search compares between looks at its
// context.
const checkEvery = 4096

// Hit is one file a search found.
type Hit struct {
	Path string
	Name string
	Dir  bool
	// Recent is a recently used file, ranked before the rest.
	Recent bool
}

// quality orders a name's match: lower is better.
type quality uint8

const (
	exact quality = iota
	prefix
	wordStart
	inside
)

// Query is a search's words, folded, without the ones a negation char
// leads (the launcher's matcher applies those).
type Query struct{ words []string }

// ParseQuery splits text into words; negate is the launcher's negation
// char, 0 for none.
func ParseQuery(text string, negate rune) Query {
	var q Query
	for w := range strings.FieldsSeq(strings.ToLower(text)) {
		if negate != 0 && strings.HasPrefix(w, string(negate)) {
			continue
		}
		q.words = append(q.words, w)
	}
	return q
}

// Empty reports a query with no word to look for.
func (q Query) Empty() bool { return len(q.words) == 0 }

func (q Query) long() bool {
	n := 0
	for _, w := range q.words {
		n += len(w)
	}
	return n >= MinQuery
}

// match is how well fold matches every word, false if one is missing:
// the first word's place decides, as it is what the person typed first.
func (q Query) match(fold string) (quality, bool) {
	for _, w := range q.words[1:] {
		if !strings.Contains(fold, w) {
			return 0, false
		}
	}
	first := q.words[0]
	at := strings.Index(fold, first)
	switch {
	case at < 0:
		return 0, false
	case len(q.words) == 1 && fold == first:
		return exact, true
	case at == 0:
		return prefix, true
	}
	for i := at; i >= 0; i = nextIndex(fold, first, i) {
		if isSeparator(fold[i-1]) {
			return wordStart, true
		}
	}
	return inside, true
}

func nextIndex(s, sub string, after int) int {
	i := strings.Index(s[after+1:], sub)
	if i < 0 {
		return -1
	}
	return after + 1 + i
}

func isSeparator(b byte) bool {
	switch b {
	case ' ', '-', '_', '.', '(', '[':
		return true
	}
	return false
}

type ranked struct {
	file    File
	quality quality
	order   int
}

// better orders candidates: match quality, then the shorter name, then
// the shallower file.
func better(a, b ranked) int {
	if a.quality != b.quality {
		return int(a.quality) - int(b.quality)
	}
	if la, lb := len(a.file.Name), len(b.file.Name); la != lb {
		return la - lb
	}
	return a.order - b.order
}

// Search finds up to limit files whose names hold every word of q:
// recent files first, newest first, then the index's best. A query
// shorter than MinQuery looks at recent files only. It returns ctx's
// error once ctx ends, which a newer query does to an older one.
func Search(ctx context.Context, s *Snapshot, q Query, limit int) ([]Hit, error) {
	if q.Empty() || limit <= 0 {
		return nil, nil
	}
	hits := make([]Hit, 0, limit)
	recent := make(map[string]bool, len(s.Recent))
	for _, p := range s.Recent {
		recent[p] = true
		if len(hits) == limit {
			continue
		}
		name := filepath.Base(p)
		if _, ok := q.match(strings.ToLower(name)); ok {
			hits = append(hits, Hit{Path: p, Name: name, Recent: true})
		}
	}
	if !q.long() || len(hits) == limit {
		return hits, nil
	}
	keep := limit
	best := make([]ranked, 0, keep+1)
	for i, f := range s.Files {
		if i%checkEvery == 0 && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		qual, ok := q.match(f.Fold)
		if !ok {
			continue
		}
		c := ranked{file: f, quality: qual, order: i}
		if len(best) == keep && better(c, best[keep-1]) >= 0 {
			continue
		}
		at, _ := slices.BinarySearchFunc(best, c, better)
		best = slices.Insert(best, at, c)
		if len(best) > keep {
			best = best[:keep]
		}
	}
	for _, c := range best {
		if len(hits) == limit {
			break
		}
		p := c.file.Path()
		if recent[p] {
			continue
		}
		hits = append(hits, Hit{Path: p, Name: c.file.Name, Dir: c.file.Dir})
	}
	return hits, nil
}
