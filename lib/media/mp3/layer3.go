package mp3

const (
	maxBigValues = granuleSize / 2
	shortBlock   = 2
	// gainOffset is the global gain of unit scale, 210, with the 4
	// quarter steps the synthesis window's scale takes back.
	gainOffset = 214
	// midSideSteps is the quarter steps a mid/side frame's gain drops,
	// the 1/√2 of its matrix.
	midSideSteps = 2
	sqrt2        = 1.41421356
	// mpeg1PosDefault is the intensity position of the top band of an
	// MPEG-1 granule when the band below it sounds in the right
	// channel: the middle.
	mpeg1PosDefault = 3
	mpeg1PosLimit   = 7
	lsfPosLimit     = 64
	lsfPreflagAt    = 500
	// bandSet8k is the band set of 8 kHz, whose mixed blocks minimp3
	// splits unlike its transform.
	bandSet8k       = 1
	mixedShortBands = 3 * 10
	// mixedShortFrom is the first short band of a mixed block, in
	// windows of three bands.
	mixedShortFrom = 3 * 3
	// istIllegal marks an MPEG-2 intensity position that codes none,
	// its scale factor's largest value.
	istIllegal = 255
)

// granule is one channel's side information for one granule.
type granule struct {
	// bands are the widths of the granule's scale factor bands, short
	// ones a window at a time, ending in 0.
	bands            []uint8
	part23           int
	bigValues        int
	globalGain       int
	scalefacCompress int
	blockType        int
	mixed            bool
	longBands        int
	shortBands       int
	tableSelect      [3]int
	regionCount      [3]int
	subblockGain     [3]int
	preflag          bool
	scalefacScale    int
	count1Table      bool
	scfsi            int
}

// mixedLongBands is how many long bands a mixed block begins with.
func mixedLongBands(mpeg1 bool) int {
	if mpeg1 {
		return 8
	}
	return 6
}

// mixedLongSubbands is how many subbands of long blocks a mixed block
// begins with: 36 values, or at 8 kHz, whose bands are twice as wide,
// 72.
func (d *Decoder) mixedLongSubbands() int {
	if d.bandSet == bandSet8k {
		return 4
	}
	return 2
}

// bandSet indexes minimp3's scale factor band tables by sample rate,
// and reports whether MPEG audio has the rate.
func bandSet(rate int) (int, bool) {
	switch rate {
	case 11025, 12000:
		return 0, true
	case 8000:
		return bandSet8k, true
	case 22050:
		return 2, true
	case 24000:
		return 3, true
	case 16000:
		return 4, true
	case 44100:
		return 5, true
	case 48000:
		return 6, true
	case 32000:
		return 7, true
	}
	return 0, false
}

// readSideInfo reads a frame's side information into d.granules and
// returns where its main data begins, in bytes before the frame's
// own frameData bytes of it.
func (d *Decoder) readSideInfo(r *bitReader, m frameMode, frameData int) (int, error) {
	var mainDataBegin, granules int
	var scfsi [maxChannels]int
	if m.mpeg1 {
		granules = 2
		mainDataBegin = r.get(9)
		r.get(5 - 2*(m.channels-1))
		for ch := range m.channels {
			scfsi[ch] = r.get(4)
		}
	} else {
		granules = 1
		mainDataBegin = r.get(8)
		r.get(m.channels)
	}
	bits := 0
	for g := range granules {
		for ch, reuse := range scfsi[:m.channels] {
			gr := &d.granules[g*m.channels+ch]
			if err := d.readGranule(r, gr, m.mpeg1); err != nil {
				return 0, err
			}
			if g == 1 && gr.blockType != shortBlock {
				gr.scfsi = reuse
			}
			bits += gr.part23
		}
	}
	if bits > 8*(mainDataBegin+frameData) {
		return 0, invalid("granules of %d bits in %d bytes of main data", bits, mainDataBegin+frameData)
	}
	return mainDataBegin, nil
}

func (d *Decoder) readGranule(r *bitReader, gr *granule, mpeg1 bool) error {
	gr.part23 = r.get(12)
	gr.bigValues = r.get(9)
	if gr.bigValues > maxBigValues {
		return invalid("%d big values", gr.bigValues)
	}
	gr.globalGain = r.get(8)
	if mpeg1 {
		gr.scalefacCompress = r.get(4)
	} else {
		gr.scalefacCompress = r.get(9)
	}
	gr.bands = scfLong[d.bandSet][:]
	gr.longBands, gr.shortBands = len(scfLong[0])-1, 0
	gr.scfsi = 0
	var tables int
	if r.flag() {
		gr.blockType = r.get(2)
		if gr.blockType == 0 {
			return invalid("window switching to block type 0")
		}
		gr.mixed = r.flag()
		gr.regionCount = [3]int{7, 255}
		if gr.blockType == shortBlock {
			if gr.mixed {
				gr.bands = d.t.mixed[d.bandSet][:]
				gr.longBands, gr.shortBands = mixedLongBands(mpeg1), mixedShortBands
			} else {
				gr.regionCount[0] = 8
				gr.bands = scfShort[d.bandSet][:]
				gr.longBands, gr.shortBands = 0, len(scfShort[0])-1
			}
		}
		tables = r.get(10) << 5
		for w := range gr.subblockGain {
			gr.subblockGain[w] = r.get(3)
		}
	} else {
		gr.blockType, gr.mixed = 0, false
		tables = r.get(15)
		gr.regionCount = [3]int{r.get(4), r.get(3), 255}
	}
	gr.tableSelect = [3]int{tables >> 10, tables >> 5 & 31, tables & 31}
	if mpeg1 {
		gr.preflag = r.flag()
	} else {
		gr.preflag = gr.scalefacCompress >= lsfPreflagAt
	}
	gr.scalefacScale = r.get(1)
	gr.count1Table = r.flag()
	return nil
}

// decodeGranule reads one granule's scale factors and spectra from main
// at bit pos into d.spec, joins its stereo, and returns the position
// after it.
func (d *Decoder) decodeGranule(main []byte, pos int, gr []granule, m frameMode) int {
	r := bitReader{b: main}
	for ch := range m.channels {
		limit := pos + gr[ch].part23
		r.pos = pos
		d.scalefactors(&r, &gr[ch], ch, m)
		d.huffman(d.spec[ch*granuleSize:(ch+1)*granuleSize], main, r.pos, &gr[ch], limit)
		pos = limit
	}
	switch {
	case m.intensity:
		d.intensityStereo(gr, m)
	case m.midSide:
		midSide(d.spec[:granuleSize], d.spec[granuleSize:])
	}
	return pos
}

// scalefactors reads a channel's scale factors and sets d.scf to the
// gain of each band.
func (d *Decoder) scalefactors(r *bitReader, gr *granule, ch int, m frameMode) {
	row := 0
	switch {
	case gr.longBands == 0:
		row = 2
	case gr.shortBands > 0:
		row = 1
	}
	partition := scfPartitions[row][:]
	var size [4]int
	scfsi := gr.scfsi
	if m.mpeg1 {
		part := int(scfcDecode[gr.scalefacCompress])
		size = [4]int{part >> 2, part >> 2, part & 3, part & 3}
	} else {
		ist := m.intensity && ch == 1
		k, sfc := 0, gr.scalefacCompress
		if ist {
			k, sfc = 12, sfc>>1
		}
		for ; sfc >= 0; k += 4 {
			product := 1
			for i := 3; i >= 0; i-- {
				mod := int(lsfModulus[k+i])
				size[i] = sfc / product % mod
				product *= mod
			}
			sfc -= product
		}
		partition = partition[k:]
		scfsi = -16
	}
	var iscf [maxBands + 1]uint8
	read := d.istPos[ch][:]
	at := 0
	for i := 0; i < 4 && partition[i] != 0; i, scfsi = i+1, scfsi*2 {
		n := int(partition[i])
		switch {
		case scfsi&8 != 0:
			copy(iscf[at:at+n], read[at:at+n])
		case size[i] == 0:
			clear(iscf[at : at+n])
			clear(read[at : at+n])
		default:
			largest := -1
			if scfsi < 0 {
				largest = 1<<size[i] - 1
			}
			for k := range n {
				s := r.get(size[i])
				iscf[at+k], read[at+k] = uint8(s), uint8(s) //nolint:gosec // at most 5 bits
				if s == largest {
					read[at+k] = istIllegal
				}
			}
		}
		at += n
	}
	shift := gr.scalefacScale + 1
	if gr.shortBands > 0 {
		sh := 3 - shift
		for i := 0; i < gr.shortBands; i += 3 {
			for w, g := range gr.subblockGain {
				iscf[gr.longBands+i+w] += uint8(g << sh) //nolint:gosec // at most 28
			}
		}
	} else if gr.preflag {
		for i, p := range pretab {
			iscf[11+i] += p
		}
	}
	gain := gr.globalGain - gainOffset
	if m.midSide {
		gain -= midSideSteps
	}
	for i := range gr.longBands + gr.shortBands {
		d.scf[i] = d.t.quarterPower(gain - int(iscf[i])<<shift)
	}
}

// huffman decodes a channel's spectrum from main at bit pos up to
// limit into dst, scaled by d.scf band by band: the big values in pairs
// through the region's table, then quadruples of zeros and ones up to
// limit.
func (d *Decoder) huffman(dst []float32, main []byte, pos int, gr *granule, limit int) {
	pow43 := &d.t.pow43
	r := newHuffReader(main, pos)
	band, scf := 0, 0
	var one float32
	o := 0
	left := gr.bigValues
	for region := 0; left > 0; region++ {
		table := gr.tableSelect[region]
		tree := huffTrees[huffTreeAt[table]:]
		escape := int(linbits[table])
		for count := gr.regionCount[region]; ; count-- {
			pairs := int(gr.bands[band]) / 2
			band++
			if pairs == 0 {
				return
			}
			one = d.scf[scf]
			scf++
			for range min(left, pairs) {
				r.refill()
				w := 5
				leaf := int(tree[r.peek(w)])
				for leaf < 0 {
					r.skip(w)
					w = leaf & 7
					leaf = int(tree[r.peek(w)-leaf>>3])
				}
				r.skip(leaf >> 8)
				for range 2 {
					v := leaf & 15
					if v == 15 && escape > 0 {
						v += r.peek(escape)
						r.skip(escape)
					}
					x := pow43[v] * one
					if r.sign() {
						x = -x
					}
					dst[o] = x
					o++
					if v != 0 {
						r.skip(1)
					}
					leaf >>= 4
				}
			}
			if left -= pairs; left <= 0 || count == 0 {
				break
			}
		}
	}
	quad := count1A[:]
	if gr.count1Table {
		quad = count1B[:]
	}
	pairs := 1 - left
	next := func() bool {
		if pairs--; pairs > 0 {
			return true
		}
		pairs = int(gr.bands[band]) / 2
		band++
		if pairs == 0 {
			return false
		}
		one = d.scf[scf]
		scf++
		return true
	}
	value := func(flag bool, at int) {
		if !flag {
			return
		}
		dst[at] = one
		if r.sign() {
			dst[at] = -one
		}
		r.skip(1)
	}
	for ; ; o += 4 {
		r.refill()
		leaf := int(quad[r.peek(4)])
		if leaf&8 == 0 {
			r.skip(4)
			leaf = int(quad[leaf>>3+r.peek(leaf&3)])
			r.skip(leaf&7 - 4)
		} else {
			r.skip(leaf & 7)
		}
		if r.pos() > limit || !next() {
			return
		}
		value(leaf&128 != 0, o)
		value(leaf&64 != 0, o+1)
		if !next() {
			return
		}
		value(leaf&32 != 0, o+2)
		value(leaf&16 != 0, o+3)
	}
}
