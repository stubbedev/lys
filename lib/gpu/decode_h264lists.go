package gpu

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/stubbedev/lys-lib/internal/nal"
)

// h264ListPic is an entry of a reference picture list: the reference
// fields, or the frame, of a DPB slot, short- or long-term, with its
// PicNum or LongTermPicNum (8.2.4.1); slot is -1 for "no reference
// picture".
type h264ListPic struct {
	slot   int
	fields h264Fields
	long   bool
	num    int64
}

// h264Lists works out the reference picture lists of a picture's
// slices (8.2.4), into storage it keeps for the next picture.
type h264Lists struct {
	frames [h264MaxSlots]int
	list   [nal.H264MaxRefIdx + 1]h264ListPic
	init   [2][2 * h264MaxSlots]h264ListPic
	inits  [2]int
	want   [nal.H264MaxRefIdx]h264ListPic
}

// h264Avail are the fields of each DPB slot a picture may predict from.
type h264Avail [h264MaxSlots]h264Fields

// names reports the fields of avail the reference picture lists of the
// slices, headed by hdrs, name, into used: every entry within each
// list's num_ref_idx_active.
func (l *h264Lists) names(s *h264State, pic *h264Pic, hdrs []*nal.H264SliceHeader, avail, used *h264Avail) error {
	*used = h264Avail{}
	for _, h := range hdrs {
		for i := range lists(h) {
			list, err := l.slice(s, pic, h, avail, i)
			if err != nil {
				return err
			}
			for _, e := range list {
				if e.slot >= 0 {
					used[e.slot] |= e.fields
				}
			}
		}
	}
	return nil
}

// same reports whether the slices' reference picture lists from the
// fields in sub are the ones from the fields in avail.
func (l *h264Lists) same(s *h264State, pic *h264Pic, hdrs []*nal.H264SliceHeader, avail, sub *h264Avail) (bool, error) {
	for _, h := range hdrs {
		for i := range lists(h) {
			want, err := l.slice(s, pic, h, avail, i)
			if err != nil {
				return false, err
			}
			want = append(l.want[:0], want...)
			got, err := l.slice(s, pic, h, sub, i)
			if err != nil || !slices.Equal(want, got) {
				return false, nil
			}
		}
	}
	return true, nil
}

// lists is how many reference picture lists a slice h heads has.
func lists(h *nal.H264SliceHeader) int {
	switch h.SliceType {
	case nal.H264SliceP, nal.H264SliceSP:
		return 1
	case nal.H264SliceB:
		return 2
	}
	return 0
}

// slice is list i of the slice h heads, from the fields in avail: the
// initial list (8.2.4.2) as long as num_ref_idx_active, modified as
// the header says (8.2.4.3).
func (l *h264Lists) slice(s *h264State, pic *h264Pic, h *nal.H264SliceHeader, avail *h264Avail, i int) ([]h264ListPic, error) {
	l.initial(s, pic, h, avail)
	n := int(h.NumRefIdxActive[i])
	list := l.list[:n+1]
	init := l.init[i][:l.inits[i]]
	copy(list, init[:min(n, len(init))])
	for j := len(init); j < n; j++ {
		list[j] = h264ListPic{slot: -1}
	}
	field := pic.fields != h264Frame
	maxPicNum := int64(1) << pic.sps.Log2MaxFrameNum
	curr := int64(h.FrameNum)
	if field {
		maxPicNum, curr = 2*maxPicNum, 2*curr+1
	}
	pred := curr
	m := &h.Modifications[i]
	for idx, op := range m.Ops[:m.N] {
		long, num := op.Idc == 2, int64(op.Arg)
		if !long {
			diff := num + 1
			if op.Idc == 0 {
				diff = -diff
			}
			pred = (pred + diff + maxPicNum) % maxPicNum
			if num = pred; num > curr {
				num -= maxPicNum
			}
		}
		want, ok := l.find(s, pic, avail, long, num)
		if !ok {
			kind := "PicNum"
			if long {
				kind = "LongTermPicNum"
			}
			return nil, fmt.Errorf("gpu: an H.264 %s's slice modifies reference picture list %d to name the picture of %s %d, which is no reference picture", pic.fields, i, kind, num)
		}
		copy(list[idx+1:], list[idx:n])
		list[idx] = want
		k := idx + 1
		for _, e := range list[idx+1 : n+1] {
			if e != want {
				list[k] = e
				k++
			}
		}
	}
	return list[:n], nil
}

// find is the reference picture of avail a modification names: the
// short-term one of PicNum num, or the long-term one of LongTermPicNum
// num.
func (l *h264Lists) find(s *h264State, pic *h264Pic, avail *h264Avail, long bool, num int64) (h264ListPic, bool) {
	for slot, fields := range avail[:s.slots] {
		r := &s.refs[slot]
		marked := r.short
		if long {
			marked = r.long
		}
		marked &= fields
		if pic.fields == h264Frame {
			if marked == h264Frame && h264PicNum(s, pic, slot, h264Frame, long) == num {
				return h264ListPic{slot: slot, fields: h264Frame, long: long, num: num}, true
			}
			continue
		}
		for _, f := range [2]h264Fields{h264Top, h264Bottom} {
			if marked&f != 0 && h264PicNum(s, pic, slot, f, long) == num {
				return h264ListPic{slot: slot, fields: f, long: long, num: num}, true
			}
		}
	}
	return h264ListPic{}, false
}

// h264PicNum is the PicNum, or the LongTermPicNum where long, of the
// fields of slot for pic (8.2.4.1).
func h264PicNum(s *h264State, pic *h264Pic, slot int, fields h264Fields, long bool) int64 {
	r := &s.refs[slot]
	n := int64(r.longIdx)
	if !long {
		n = int64(r.frameNum)
		if r.frameNum > pic.hdr.FrameNum {
			n -= int64(1) << pic.sps.Log2MaxFrameNum
		}
	}
	if pic.fields == h264Frame {
		return n
	}
	return 2*n + int64(b2i(fields == pic.fields))
}

// initial lays out the initial reference picture lists of the slice h
// heads from the fields in avail (8.2.4.2) in l.init.
func (l *h264Lists) initial(s *h264State, pic *h264Pic, h *nal.H264SliceHeader, avail *h264Avail) {
	l.inits = [2]int{}
	n := lists(h)
	cur := pic.order()
	for i := range n {
		short := l.frames[:0]
		for slot := range s.slots {
			if l.marked(s, pic, avail, slot, false) != 0 {
				short = append(short, slot)
			}
		}
		if h.SliceType == nal.H264SliceB {
			before := func(slot int) bool { return s.refs[slot].order() <= cur }
			slices.SortFunc(short, func(a, b int) int {
				ba, bb := before(a), before(b)
				switch {
				case ba != bb:
					return cmp.Compare(b2i(bb != (i == 1)), b2i(ba != (i == 1)))
				case ba:
					return cmp.Or(cmp.Compare(s.refs[b].order(), s.refs[a].order()), cmp.Compare(a, b))
				}
				return cmp.Or(cmp.Compare(s.refs[a].order(), s.refs[b].order()), cmp.Compare(a, b))
			})
		} else {
			slices.SortFunc(short, func(a, b int) int {
				return cmp.Or(cmp.Compare(h264PicNum(s, pic, b, h264Frame, false), h264PicNum(s, pic, a, h264Frame, false)), cmp.Compare(a, b))
			})
		}
		l.add(s, pic, avail, i, short, false)
		long := l.frames[:0]
		for slot := range s.slots {
			if l.marked(s, pic, avail, slot, true) != 0 {
				long = append(long, slot)
			}
		}
		slices.SortFunc(long, func(a, b int) int {
			return cmp.Or(cmp.Compare(s.refs[a].longIdx, s.refs[b].longIdx), cmp.Compare(a, b))
		})
		l.add(s, pic, avail, i, long, true)
	}
	if n == 2 && l.inits[1] > 1 && slices.Equal(l.init[0][:l.inits[0]], l.init[1][:l.inits[1]]) {
		l.init[1][0], l.init[1][1] = l.init[1][1], l.init[1][0]
	}
}

// marked are the fields of slot avail holds that are short-term, or
// long-term, references pic may predict from: a frame's both, a
// field's either.
func (l *h264Lists) marked(s *h264State, pic *h264Pic, avail *h264Avail, slot int, long bool) h264Fields {
	r := &s.refs[slot]
	marked := r.short
	if long {
		marked = r.long
	}
	marked &= avail[slot]
	if pic.fields == h264Frame && marked != h264Frame {
		return 0
	}
	return marked
}

// add appends the frames of DPB slots frames to initial list i: whole
// for a frame, else their fields, alternately of pic's parity and the
// other, starting with pic's (8.2.4.2.5).
func (l *h264Lists) add(s *h264State, pic *h264Pic, avail *h264Avail, i int, frames []int, long bool) {
	put := func(slot int, fields h264Fields) {
		l.init[i][l.inits[i]] = h264ListPic{slot: slot, fields: fields, long: long, num: h264PicNum(s, pic, slot, fields, long)}
		l.inits[i]++
	}
	if pic.fields == h264Frame {
		for _, slot := range frames {
			put(slot, h264Frame)
		}
		return
	}
	parity := [2]h264Fields{pic.fields, h264Frame &^ pic.fields}
	var at [2]int
	next := func(p int) (int, bool) {
		for ; at[p] < len(frames); at[p]++ {
			if slot := frames[at[p]]; l.marked(s, pic, avail, slot, long)&parity[p] != 0 {
				at[p]++
				return slot, true
			}
		}
		return 0, false
	}
	for p := 0; ; p = 1 - p {
		slot, ok := next(p)
		if !ok {
			for slot, ok := next(1 - p); ok; slot, ok = next(1 - p) {
				put(slot, parity[1-p])
			}
			return
		}
		put(slot, parity[p])
	}
}
