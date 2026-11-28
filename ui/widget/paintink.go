package widget

import (
	"math"
	"reflect"
	"runtime"
	"sync"
	"unicode/utf8"

	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/ui/internal/style"
)

type inkBounder interface {
	ownInk() (owner Widget, reach render.Insets, closed bool)
}

type inkClipper interface{ clipsInk() bool }

type inkOverhanger interface {
	inkOverhang() (reach render.Insets, known bool)
}

func (l *Label) inkOverhang() (render.Insets, bool) {
	ascii := true
	for _, r := range l.shown() {
		if r >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if ascii {
		return render.Insets{}, true
	}
	reach, known := l.shaped.InkOverhang()
	if short := l.shaped.LineHeight() - l.bounds.H; short > 0 {
		reach.Top += short
		reach.Bottom += short
	}
	return reach, known
}

//nolint:unparam
func (b *Box) ownInk() (Widget, render.Insets, bool) {
	return b, render.Insets{}, len(b.departing) == 0
}

func (b *Box) clipsInk() bool {
	v := b.style(b)
	return v.Has(style.PropOverflowX) && v.Layout.OverflowX.Clips() && v.Has(style.PropOverflowY) && v.Layout.OverflowY.Clips()
}

func (s *Scroll) ownInk() (Widget, render.Insets, bool) { return s, render.Insets{}, true }
func (s *Scroll) clipsInk() bool                        { return true }

func (l *List) ownInk() (Widget, render.Insets, bool) { return l, render.Insets{}, true }
func (l *List) clipsInk() bool                        { return true }

func (r *listRow) ownInk() (Widget, render.Insets, bool) { return r, render.Insets{}, true }

func (s *Separator) ownInk() (Widget, render.Insets, bool) { return s, render.Insets{}, true }

func (s *Spinner) ownInk() (Widget, render.Insets, bool) { return s, render.Insets{}, true }

func (b *Button) ownInk() (Widget, render.Insets, bool) { return b, render.Insets{}, true }

func (g *Grid) ownInk() (Widget, render.Insets, bool) { return g, render.Insets{}, true }

func (f *FlowBox) ownInk() (Widget, render.Insets, bool) { return f, render.Insets{}, true }

func (c *FlowBoxChild) ownInk() (Widget, render.Insets, bool) { return c, render.Insets{}, true }

func (l *ListBox) ownInk() (Widget, render.Insets, bool) { return l, render.Insets{}, true }

func (r *ListBoxRow) ownInk() (Widget, render.Insets, bool) { return r, render.Insets{}, true }

func (i *Icon) ownInk() (Widget, render.Insets, bool) {
	v := i.style(i)
	if v.Has(style.PropIconTransform) && v.IconXform.M != render.Identity {
		return i, render.Insets{}, false
	}
	nat := i.natural()
	over := max(nat.W-i.bounds.W, nat.H-i.bounds.H, 0)/2 + 1
	return i, render.UniformInsets(over), true
}

func (l *Label) ownInk() (Widget, render.Insets, bool) {
	v := l.style(l)
	if l.pseudo != nil || v.TextShadow.N > 0 || l.shaped == nil || hasCombiningMark(l.shown()) || !l.textFits(v) {
		return l, render.Insets{}, false
	}
	return l, render.UniformInsets(2 * int(math.Ceil(fontPx(v, l.sizePx)))), true
}

// textFits reports whether the text's rows stay inside the label: a
// single line no wider than its bounds, wrapped rows no wider than its
// content box and no more than it holds.
func (l *Label) textFits(v *style.Values) bool {
	if !l.multiline(v) {
		return l.shaped.Advance() <= float64(l.bounds.W)
	}
	b := boxOf(v, render.Insets{})
	content := b.padding.Shrink(b.border.Shrink(l.bounds))
	lines := l.wrapped(float64(content.W))
	if len(lines)*l.lineBox(v) > content.H {
		return false
	}
	face, px := l.effStyleIn(v)
	indent := l.indentOf(v)
	for i, ln := range lines {
		adv := face.Shape(ln, px).Advance()
		if i == 0 {
			adv += indent
		}
		if adv > float64(content.W) {
			return false
		}
	}
	return true
}

var cullInk = true

// paintsAs reports whether self paints exactly as owner, a widget it
// embeds: no type on the way down from self to owner's declares a Paint
// of its own, so self.Paint is owner's, and so is its ink. An app's
// widget built on a kit widget is the usual case. The answer is per pair
// of types, kept once found.
func paintsAs(self, owner Widget) bool {
	key := [2]reflect.Type{reflect.TypeOf(self), reflect.TypeOf(owner)}
	paintPromotion.mu.Lock()
	defer paintPromotion.mu.Unlock()
	ok, known := paintPromotion.of[key]
	if !known {
		ok = embedsPaintOf(key[0], key[1])
		paintPromotion.of[key] = ok
	}
	return ok
}

var paintPromotion = struct {
	mu sync.Mutex
	of map[[2]reflect.Type]bool
}{of: map[[2]reflect.Type]bool{}}

// embedsPaintOf reports whether t, a pointer type, reaches owner through
// embedded fields whose types, like t, only promote Paint.
func embedsPaintOf(t, owner reflect.Type) bool {
	if t == owner {
		return true
	}
	if t.Kind() != reflect.Pointer || t.Elem().Kind() != reflect.Struct || !promotesPaint(t) {
		return false
	}
	for f := range t.Elem().Fields() {
		if !f.Anonymous {
			continue
		}
		ft := f.Type
		if ft.Kind() == reflect.Struct {
			ft = reflect.PointerTo(ft)
		}
		if embedsPaintOf(ft, owner) {
			return true
		}
	}
	return false
}

// promotesPaint reports whether t's Paint is one the compiler promoted
// from an embedded field rather than one t declares: a promoted method
// is a generated wrapper, which has no source file.
func promotesPaint(t reflect.Type) bool {
	m, ok := t.MethodByName("Paint")
	if !ok {
		return false
	}
	pc := m.Func.Pointer()
	file, _ := runtime.FuncForPC(pc).FileLine(pc)
	return file == "<autogenerated>"
}

var focusInk = render.UniformInsets(FocusRingWidth)

func (n *node) computeInk() {
	n.inkClosed, n.inkClips = false, false
	self := n.self
	b, ok := self.(inkBounder)
	if !ok || n.inkNodeless || n.opensInk(self) {
		return
	}
	owner, reach, closed := b.ownInk()
	if !closed || owner != self && !paintsAs(self, owner) {
		return
	}
	n.inkClosed = true
	n.inkBox = focusInk.Grow(n.ink.Grow(reach.Grow(n.bounds)))
	n.solidBox = n.ink.Grow(n.bounds)
	if t, ok := self.(inkOverhanger); ok {
		if reach, known := t.inkOverhang(); known {
			n.solidBox = n.ink.Grow(reach.Grow(n.bounds))
		} else {
			n.solidBox = n.inkBox
		}
	}
	if c, ok := self.(inkClipper); ok {
		n.inkClips = c.clipsInk()
	}
}

func (n *node) opensInk(self Widget) bool {
	if n.sharedTag != "" || n.motion != nil || n.anims != nil || n.trCancel != nil || n.pseudo != nil {
		return true
	}
	v := n.style(self)
	return v.Has(style.PropTransform) || v.Has(style.PropBackdropFilter) || n.effLayoutMotion(self) != LayoutMotionNone
}

func (n *node) settleInk() {
	n.computeInk()
	n.bubbleInk()
}

func (n *node) bubbleInk() {
	box, solid, closed := n.inkBox, n.solidBox, n.inkClosed
	for p := n.parent; p != nil; p = parentOf(p) {
		pn := nodeOf(p)
		switch {
		case pn == nil:
		case !closed && !pn.inkClosed:
			return
		case !closed:
			pn.inkClosed = false
		case pn.inkClips || !pn.inkClosed || (box.Intersect(pn.inkBox) == box && solid.Intersect(pn.solidBox) == solid):
			return
		default:
			pn.inkBox = pn.inkBox.Union(box)
			pn.solidBox = pn.solidBox.Union(solid)
		}
	}
}

func inkOutsideClip(cv *render.Canvas, n *node) bool {
	if !cullInk || !n.inkClosed {
		return false
	}
	clip := cv.PushClipDevice(render.Rect{})
	cv.PopClip(clip)
	return cv.MapRect(n.inkBox).Intersect(clip).Empty()
}
