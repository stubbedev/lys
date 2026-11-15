package render

// The text caches: a process-wide LRU for shaping results, keyed by
// (font, pixel size, string), and the machinery both it and the glyph
// atlas share. Shaping is the most expensive text operation -
// itemization plus glyph positioning, allocating as it goes - and
// widgets re-shape the same string many times per frame (Entry's paint
// alone shapes three times). Text is immutable per key, so entries are
// never invalidated, only evicted: a string that edits reshapes under a
// new key and the stale one ages out.

import (
	"fmt"
	"hash/maphash"
	"math"
	"slices"
	"sync"
	"unicode/utf8"
	"unsafe"

	"github.com/stubbedev/memory-go/heapslab"

	"github.com/stubbedev/lys-lib/text"
)

// shapeKey identifies one shaped line. The font field compares by
// pointer identity - a *Typeface or a *Chain - so a chain held by the
// app reuses its entries across frames and two chains over the same
// faces stay separate keys; neither deep face equality nor per-chain
// rune picks leak into the key. The direction is part of the key: the
// same text under an explicit left-to-right vs right-to-left base
// resolves and reorders differently.
type shapeKey struct {
	font Font
	gen  uint64
	px   float64
	text string
	dir  text.Direction
}

// Cache budgets. Shaped lines are small (a few hundred bytes each) and
// rasterized glyphs are 8bpp masks, so tens of megabytes bound the
// worst text-heavy window with room to spare.
const (
	shapeCacheBudget = 16 << 20 // shaped lines
	glyphCacheBudget = 8 << 20  // glyph masks and scaled bitmap strikes
	// runeShapeCacheBudget bounds the rune-keyed cache the per-rune
	// probes share. Entries cost exactly what a one-rune shape costs
	// (tens of bytes plus its glyphs), and a text-heavy window touches
	// a few hundred distinct runes, so a megabyte is far above use and
	// the LRU covers the rest.
	runeShapeCacheBudget = 1 << 20
	// subpixelBuckets quantizes a glyph's fractional pen offset: four
	// buckets per axis bound the cache blowup a fractional device
	// scale (150/120) would otherwise cause while keeping positional
	// error at a quarter pixel, under the AA ramp's own resolution.
	subpixelBuckets = 4
	// shapeOverhead is the per-entry cost model constant: ShapedText
	// itself, the run slice, and map bookkeeping.
	shapeOverhead = 128
	// glyphBytesPerRun is the per-glyph cost of a shaping.Glyph: nine
	// fixed.Int26_6 fields plus cluster, counts, and ID.
	glyphBytesPerRun = 96
)

// shapeCache is the process-wide shaping cache. Hit from widgets on the
// loop goroutine today, mutex-guarded regardless; the fill work (the
// shaper itself) runs outside the lock, so concurrent misses on one
// Typeface remain as unsafe as ever - the same contract Typeface has
// always had.
var shapes = newLRU[shapeKey, *ShapedText](shapeCacheBudget)

// cachedShape returns the shape of text at px for font under base
// direction d, filling the cache on a miss. The returned pointer is
// shared: ShapedText is immutable once built, so every caller may keep
// and read it freely.
func cachedShape(font Font, px float64, s string, d text.Direction, fill func() *ShapedText) *ShapedText {
	key := shapeKey{font: font, gen: generationOf(font), px: px, text: s, dir: d}
	if sh, ok := shapes.get(key); ok {
		return sh
	}
	sh := fill()
	shapes.put(key, sh, shapedCost(sh))
	return sh
}

// runeKey identifies one shaped rune. The same font identity and size
// semantics as shapeKey, minus the string: a probe's cost was the
// one-rune string each lookup built, so the key stores the rune.
type runeKey struct {
	font Font
	gen  uint64
	px   float64
	r    rune
}

// runeShapes is the rune-keyed half of the shaping cache, serving the
// per-rune probes (Font.ShapeRune). Same LRU, same cost model.
var runeShapes = newLRU[runeKey, *ShapedText](runeShapeCacheBudget)

// cachedShapeRune returns the shape of one rune at px for font,
// filling the cache on a miss; the fill must produce what Shape of the
// one-rune string would, so both caches agree entry for entry.
func cachedShapeRune(font Font, px float64, r rune, fill func() *ShapedText) *ShapedText {
	key := runeKey{font: font, gen: generationOf(font), px: px, r: r}
	if sh, ok := runeShapes.get(key); ok {
		return sh
	}
	sh := fill()
	runeShapes.put(key, sh, shapedCost(sh))
	return sh
}

// The rune-keyed half of the line cache: widgets that hold their
// contents as rune slices (text areas, entries while composing) shape
// through it, so a frame's probes never convert runes to a string per
// call. Slices are not comparable, so the key is a hash over
// (font, size, direction, runes) with full equality on the bucket
// compare. The table is set-associative with a fixed way count, so its
// memory is bounded by construction; entries leave only by collision.
const (
	runeLineSets = 512
	runeLineWays = 4
)

type runeLineEntry struct {
	font  Font
	gen   uint64
	px    float64
	dir   text.Direction
	rs    []rune
	val   *ShapedText
	stamp uint64
}

var (
	runeLineMu    sync.Mutex
	runeLines     [runeLineSets][runeLineWays]runeLineEntry
	runeLineClock uint64
)

var runeLineSeeding struct {
	once sync.Once
	seed maphash.Seed
}

// runeLineSeed is the seed of the rune line hashes, drawn when the
// first line is hashed.
func runeLineSeed() maphash.Seed {
	runeLineSeeding.once.Do(func() { runeLineSeeding.seed = maphash.MakeSeed() })
	return runeLineSeeding.seed
}

// cachedShapeRunes returns the shape of rs at px for font under base
// direction d, filling the table on a miss; the fill must produce what
// ShapeDir of the string form would, so both caches agree entry for
// entry.
func cachedShapeRunes(font Font, px float64, rs []rune, d text.Direction, fill func() *ShapedText) *ShapedText {
	key := runeLineHash(font, px, rs, d)
	gen := generationOf(font)
	runeLineMu.Lock()
	set := &runeLines[key%runeLineSets]
	runeLineClock++
	for i := range set {
		e := &set[i]
		if e.stamp != 0 && e.font == font && e.gen == gen && e.px == px && e.dir == d && slices.Equal(e.rs, rs) {
			e.stamp = runeLineClock
			sh := e.val
			runeLineMu.Unlock()
			return sh
		}
	}
	runeLineMu.Unlock()
	sh := fill()
	runeLineMu.Lock()
	set = &runeLines[key%runeLineSets]
	var victim *runeLineEntry
	for i := range set {
		if set[i].stamp == 0 || victim == nil || set[i].stamp < victim.stamp {
			victim = &set[i]
		}
	}
	*victim = runeLineEntry{font: font, px: px, dir: d, rs: slices.Clone(rs), val: sh, stamp: runeLineClock}
	runeLineMu.Unlock()
	return sh
}

// runeLineHash folds the key's parts into the set index. The hash is
// O(runes) per probe, the same order the string-keyed map's own hash
// pays over the bytes.
func runeLineHash(font Font, px float64, rs []rune, d text.Direction) uint64 {
	h := uint64(0xcbf29ce484222325)
	mix := func(x uint64) {
		h ^= x
		h *= 0x100000001b3
	}
	mix(maphash.Comparable(runeLineSeed(), font))
	mix(math.Float64bits(px))
	mix(uint64(d))
	for _, r := range rs {
		mix(uint64(r & 0x7fffffff))
	}
	return h
}

// shapedCost estimates an entry's memory: the string, the caret table,
// and one row per shaped glyph.
func shapedCost(s *ShapedText) int {
	return shapeOverhead + int(unsafe.Sizeof(glyphPlan{})) + len(s.text) +
		(glyphBytesPerRun+int(unsafe.Sizeof(resolvedGlyph{})))*s.glyphCount() +
		8*(utf8.RuneCountInString(s.text)+1)
}

// dropRuneLines empties the rune-keyed caches; tests use it to pin
// miss-path costs and keep caches fixture-local.
func dropRuneLines() {
	runeLineMu.Lock()
	defer runeLineMu.Unlock()
	runeLines = [runeLineSets][runeLineWays]runeLineEntry{}
	runeShapes.dropAll()
}

// lru is a mutex-guarded LRU map with a byte budget. Entries move to
// the front on hit; eviction walks the back when a put pushes the total
// over budget, always keeping the freshly inserted entry. K must not
// retain more than the caller intends; V's cost is computed by callers
// and passed to put. Entries live in chunks of a pool, linked newest to
// oldest, so an insert after eviction reuses the evicted entry's slot
// and the collector tracks a chunk, not each entry.
type lru[K comparable, V any] struct {
	gets    uint64
	mu      sync.Mutex
	entries map[K]heapslab.Item[lruEntry[K, V]]
	pool    *heapslab.Pool[lruEntry[K, V]]
	newest  heapslab.Item[lruEntry[K, V]]
	oldest  heapslab.Item[lruEntry[K, V]]
	live    int
	budget  int
}

type lruEntry[K comparable, V any] struct {
	key          K
	val          V
	cost         int
	newer, older heapslab.Item[lruEntry[K, V]]
}

const lruChunk = 64

// newLRU is an empty cache of budget, which holds nothing until its
// first put: a package-level cache costs its importer no code.
func newLRU[K comparable, V any](budget int) *lru[K, V] {
	return &lru[K, V]{budget: budget}
}

// get returns the cached value, marking it most recently used.
func (c *lru[K, V]) get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gets++
	it, ok := c.entries[key]
	if !ok {
		var zero V
		return zero, false
	}
	c.unlink(it)
	c.pushNewest(it)
	return it.Value().val, true
}

// put inserts or refreshes an entry and evicts least-recently-used
// entries while the total cost exceeds the budget. The new entry always
// survives: a cache that evicts what it was just asked for is useless.
func (c *lru[K, V]) put(key K, val V, cost int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if it, ok := c.entries[key]; ok {
		c.unlink(it)
		c.pushNewest(it)
		e := it.Value()
		c.live += cost - e.cost
		e.val, e.cost = val, cost
	} else {
		if c.entries == nil {
			pool, err := heapslab.NewPool[lruEntry[K, V]](lruChunk)
			if err != nil {
				panic(fmt.Sprintf("render: the cache's entry pool: %v", err))
			}
			c.entries, c.pool = make(map[K]heapslab.Item[lruEntry[K, V]]), pool
		}
		it := c.pool.Get()
		*it.Value() = lruEntry[K, V]{key: key, val: val, cost: cost}
		c.entries[key] = it
		c.pushNewest(it)
		c.live += cost
	}
	for c.live > c.budget && c.oldest != c.newest {
		old := c.oldest
		e := old.Value()
		c.unlink(old)
		delete(c.entries, e.key)
		c.live -= e.cost
		c.pool.Put(old)
	}
}

func (c *lru[K, V]) unlink(it heapslab.Item[lruEntry[K, V]]) {
	e := it.Value()
	if e.newer.IsNil() {
		c.newest = e.older
	} else {
		e.newer.Value().older = e.older
	}
	if e.older.IsNil() {
		c.oldest = e.newer
	} else {
		e.older.Value().newer = e.newer
	}
	e.newer, e.older = heapslab.Item[lruEntry[K, V]]{}, heapslab.Item[lruEntry[K, V]]{}
}

func (c *lru[K, V]) pushNewest(it heapslab.Item[lruEntry[K, V]]) {
	e := it.Value()
	e.older = c.newest
	if c.newest.IsNil() {
		c.oldest = it
	} else {
		c.newest.Value().newer = it
	}
	c.newest = it
}

// len returns the number of cached entries.
func (c *lru[K, V]) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// dropAll empties the cache. Tests use it to pin miss-path costs and
// to keep caches fixture-local; no production caller evicts wholesale
// (the budget LRU is the only production eviction).
func (c *lru[K, V]) dropAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pool != nil {
		c.pool.Free()
	}
	c.entries, c.pool = nil, nil
	c.newest, c.oldest = heapslab.Item[lruEntry[K, V]]{}, heapslab.Item[lruEntry[K, V]]{}
	c.live = 0
}
