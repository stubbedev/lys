package mp3

// midSide turns mid and side into left and right in place.
func midSide(left, right []float32) {
	right = right[:len(left)]
	for i, a := range left {
		b := right[i]
		left[i], right[i] = a+b, a-b
	}
}

// intensityStereo joins a granule's channels where the right one is
// coded as an intensity position of the left: in the bands above the
// last it sounds in, window by window in short bands; the long bands
// of a mixed block only when no short window sounds. Mid/side joins
// the bands below.
func (d *Decoder) intensityStereo(gr []granule, m frameMode) {
	g := &gr[0]
	n := g.longBands + g.shortBands
	right := d.spec[granuleSize:]
	topLong, top := -1, [3]int{-1, -1, -1}
	at := 0
	for i := range n {
		width := int(g.bands[i])
		if sounds(right[at : at+width]) {
			if i < g.longBands {
				topLong = i
			} else {
				top[(i-g.longBands)%3] = i
			}
		}
		at += width
	}
	if max(top[0], top[1], top[2]) >= 0 {
		topLong = g.longBands - 1
	}
	bound := func(i int) int {
		if i < g.longBands {
			return topLong
		}
		return top[(i-g.longBands)%3]
	}
	windows := 1
	if g.shortBands > 0 {
		windows = 3
	}
	ist := &d.istPos[1]
	for w := range windows {
		highest := n - windows + w
		below := highest - windows
		switch {
		case bound(highest) < below:
			ist[highest] = ist[below]
		case m.mpeg1:
			ist[highest] = mpeg1PosDefault
		default:
			ist[highest] = 0
		}
	}
	limit := lsfPosLimit
	if m.mpeg1 {
		limit = mpeg1PosLimit
	}
	var scale float32 = 1
	if m.midSide {
		scale = sqrt2
	}
	shift := gr[1].scalefacCompress & 1
	left := d.spec[:granuleSize]
	at = 0
	for i := 0; g.bands[i] != 0; i++ {
		width := int(g.bands[i])
		pos := int(ist[i])
		switch {
		case i > bound(i) && pos < limit:
			var kl, kr float32
			if m.mpeg1 {
				kl, kr = intensityPan[2*pos], intensityPan[2*pos+1]
			} else {
				kl, kr = 1, d.t.quarterPower(-((pos + 1) >> 1 << shift))
				if pos&1 != 0 {
					kl, kr = kr, 1
				}
			}
			kl, kr = kl*scale, kr*scale
			for k := at; k < at+width; k++ {
				right[k] = left[k] * kr
				left[k] *= kl
			}
		case m.midSide:
			midSide(left[at:at+width], right[at:at+width])
		}
		at += width
	}
}

// sounds reports whether a band of the right channel is not all zero.
func sounds(band []float32) bool {
	for _, v := range band {
		if v != 0 {
			return true
		}
	}
	return false
}

// hybrid turns a channel's spectrum into its 18 samples in each of the
// 32 subbands: short blocks reordered by frequency, aliasing between
// long subbands reduced, the inverse MDCT of each subband windowed by
// its block's window and overlapped with the last granule's, and every
// other sample of the odd subbands negated. The lowest subbands of a
// mixed block are long blocks of the normal window.
func (d *Decoder) hybrid(ch int, g *granule) {
	spec := d.spec[ch*granuleSize : (ch+1)*granuleSize]
	longSubbands := subbands
	if g.blockType == shortBlock {
		longSubbands = 0
		if g.mixed {
			longSubbands = d.mixedLongSubbands()
		}
	}
	aliased := subbands - 1
	if g.shortBands > 0 {
		aliased = longSubbands - 1
		reorder(spec[longSubbands*18:], d.syn[:granuleSize], g.bands[g.longBands:])
	}
	reduceAliasing(spec, aliased)
	window := &d.t.windows[0]
	if g.blockType != shortBlock {
		window = &d.t.windows[g.blockType]
	}
	overlap := &d.overlap[ch]
	for b := range subbands {
		x, ov := spec[b*18:(b+1)*18], (*[18]float32)(overlap[b*18:(b+1)*18])
		if b < longSubbands {
			inverseLong(x, ov, window)
		} else {
			inverseShort(x, ov, &d.t.short)
		}
	}
	for b := 1; b < subbands; b += 2 {
		for i := b*18 + 1; i < (b+1)*18; i += 2 {
			spec[i] = -spec[i]
		}
	}
}

// reorder interleaves the three windows of each short band, so the
// values run by frequency, then window.
func reorder(spec, scratch []float32, bands []uint8) {
	src, dst := 0, 0
	for b := 0; bands[b] != 0; b += 3 {
		n := int(bands[b])
		for i := range n {
			scratch[dst] = spec[src+i]
			scratch[dst+1] = spec[src+n+i]
			scratch[dst+2] = spec[src+2*n+i]
			dst += 3
		}
		src += 3 * n
	}
	copy(spec, scratch[:dst])
}

func reduceAliasing(spec []float32, n int) {
	for b := range max(n, 0) {
		s := spec[b*18:]
		for i := range 8 {
			u, dn := s[18+i], s[17-i]
			s[18+i] = float32(u*antialias[0][i]) - float32(dn*antialias[1][i])
			s[17-i] = float32(u*antialias[1][i]) + float32(dn*antialias[0][i])
		}
	}
}

// dct3x9 is the 9-point DCT-III of y in place.
func dct3x9(y *[9]float32) {
	s0, s2, s4, s6, s8 := y[0], y[2], y[4], y[6], y[8]
	t0 := s0 + float32(s6*0.5)
	s0 -= s6
	t4 := float32((s4 + s2) * 0.93969262)
	t2 := float32((s8 + s2) * 0.76604444)
	s6 = float32((s4 - s8) * 0.17364818)
	s4 += s8 - s2

	s2 = s0 - float32(s4*0.5)
	y[4] = s4 + s0
	s8 = t0 - t2 + s6
	s0 = t0 - t4 + t2
	s4 = t0 + t4 - s6

	s1, s3, s5, s7 := y[1], y[3], y[5], y[7]
	s3 = float32(s3 * 0.86602540)
	t0 = float32((s5 + s1) * 0.98480775)
	t4 = float32((s5 - s7) * 0.34202014)
	t2 = float32((s1 + s7) * 0.64278761)
	s1 = float32((s1 - s5 - s7) * 0.86602540)

	s5 = t0 - s3 - t2
	s7 = t4 - s3 - t0
	s3 = t4 + s3 - t2

	y[0] = s4 - s7
	y[1] = s2 + s1
	y[2] = s0 - s3
	y[3] = s8 + s5
	y[5] = s8 - s5
	y[6] = s0 + s3
	y[7] = s2 - s1
	y[8] = s4 + s7
}

// imdct36 is the 36-point inverse MDCT of a long subband's 18 values:
// a, the first half's first 9 samples, whose second 9 mirror them
// negated, and b, the second half's first 9, whose second 9 mirror
// them.
func imdct36(x []float32) (a, b [9]float32) {
	var co, si [9]float32
	co[0] = -x[0]
	si[0] = x[17]
	for i := range 4 {
		si[8-2*i] = x[4*i+1] - x[4*i+2]
		co[1+2*i] = x[4*i+1] + x[4*i+2]
		si[7-2*i] = x[4*i+4] - x[4*i+3]
		co[2+2*i] = -(x[4*i+3] + x[4*i+4])
	}
	dct3x9(&co)
	dct3x9(&si)
	si[1], si[3], si[5], si[7] = -si[1], -si[3], -si[5], -si[7]
	for i := range 9 {
		a[i] = -(float32(co[i]*twiddle9[9+i]) + float32(si[i]*twiddle9[i]))
		b[i] = float32(co[i]*twiddle9[i]) - float32(si[i]*twiddle9[9+i])
	}
	return a, b
}

// inverseLong replaces a long subband's values with its first 18
// samples, windowed and added to the windowed second half the last
// granule left in overlap, and leaves its own there.
func inverseLong(x []float32, overlap *[18]float32, window *[36]float32) {
	a, b := imdct36(x)
	for i := range 9 {
		x[i] = overlap[i] + float32(a[i]*window[i])
		x[17-i] = overlap[17-i] - float32(a[i]*window[17-i])
		overlap[i] = b[i] * window[18+i]
		overlap[17-i] = b[i] * window[35-i]
	}
}

func idct3(x0, x1, x2 float32, dst *[3]float32) {
	m1 := float32(x1 * 0.86602540)
	a1 := x0 - float32(x2*0.5)
	dst[1] = x0 + x2
	dst[0] = a1 + m1
	dst[2] = a1 - m1
}

// imdct12 is the 12-point inverse MDCT of one short window's values, x
// read every third: its halves as imdct36's.
func imdct12(x []float32) (a, b [3]float32) {
	var co, si [3]float32
	idct3(-x[0], x[6]+x[3], x[12]+x[9], &co)
	idct3(x[15], x[12]-x[9], x[6]-x[3], &si)
	si[1] = -si[1]
	for i := range 3 {
		a[i] = -(float32(co[i]*twiddle3[3+i]) + float32(si[i]*twiddle3[i]))
		b[i] = float32(co[i]*twiddle3[i]) - float32(si[i]*twiddle3[3+i])
	}
	return a, b
}

// inverseShort is inverseLong of a subband of three short windows, which
// lie 6 samples into the block and overlap each other by half.
func inverseShort(x []float32, overlap *[18]float32, window *[12]float32) {
	var block [18 * 2]float32
	for w := range 3 {
		a, b := imdct12(x[w:])
		z := block[6+6*w:]
		for i := range 3 {
			z[i] += float32(a[i] * window[i])
			z[5-i] -= float32(a[i] * window[5-i])
			z[6+i] += float32(b[i] * window[6+i])
			z[11-i] += float32(b[i] * window[11-i])
		}
	}
	for i := range 18 {
		x[i] = overlap[i] + block[i]
		overlap[i] = block[18+i]
	}
}
