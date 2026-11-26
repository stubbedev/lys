package mp3

import "github.com/stubbedev/lys/lib/media/internal/pcm"

// qmfHistory is the floats of synthesis history a granule leaves for
// the next: 15 of the 16 time slots the window spans, each a pair of
// channels at two phases.
const qmfHistory = 15 * 2 * subbands

// dctII is the 32-point DCT-II, as the polyphase synthesis wants it,
// of the first n of the 18 time slots in a channel's subband samples,
// in place.
func dctII(spec []float32, n int) {
	for k := range n {
		var t [4][8]float32
		y := spec[k:]
		for i := range 8 {
			x0, x1 := y[i*18], y[(15-i)*18]
			x2, x3 := y[(16+i)*18], y[(31-i)*18]
			t0, t1 := x0+x3, x1+x2
			t2 := (x1 - x2) * dctSecant[3*i]
			t3 := (x0 - x3) * dctSecant[3*i+1]
			t[0][i] = t0 + t1
			t[1][i] = (t0 - t1) * dctSecant[3*i+2]
			t[2][i] = t3 + t2
			t[3][i] = (t3 - t2) * dctSecant[3*i+2]
		}
		for i := range t {
			x := &t[i]
			x0, x1, x2, x3, x4, x5, x6, x7 := x[0], x[1], x[2], x[3], x[4], x[5], x[6], x[7]
			xt := x0 - x7
			x0 += x7
			x7 = x1 - x6
			x1 += x6
			x6 = x2 - x5
			x2 += x5
			x5 = x3 - x4
			x3 += x4
			x4 = x0 - x3
			x0 += x3
			x3 = x1 - x2
			x1 += x2
			x[0] = x0 + x1
			x[4] = (x0 - x1) * 0.70710677
			x5 += x6
			x6 = (x6 + x7) * 0.70710677
			x7 += xt
			x3 = (x3 + x4) * 0.70710677
			x5 -= x7 * 0.198912367
			x7 += x5 * 0.382683432
			x5 -= x7 * 0.198912367
			x0 = xt - x6
			xt += x6
			x[1] = (xt + x7) * 0.50979561
			x[2] = (x4 + x3) * 0.54119611
			x[3] = (x0 - x5) * 0.60134488
			x[5] = (x0 + x5) * 0.89997619
			x[6] = (x4 - x3) * 1.30656302
			x[7] = (xt - x7) * 2.56291556
		}
		for i := range 7 {
			o := 4 * i * 18
			y[o] = t[0][i]
			y[o+18] = t[2][i] + t[3][i] + t[3][i+1]
			y[o+2*18] = t[1][i] + t[1][i+1]
			y[o+3*18] = t[2][i+1] + t[3][i] + t[3][i+1]
		}
		y[28*18] = t[0][7]
		y[29*18] = t[2][7] + t[3][7]
		y[30*18] = t[1][7]
		y[31*18] = t[3][7]
	}
}

// synthesize runs the granule's subband samples through the polyphase
// synthesis filterbank into out, 576 interleaved samples per channel.
func (d *Decoder) synthesize(out []int16) {
	nch := d.channels
	for ch := range nch {
		dctII(d.spec[ch*granuleSize:], 18)
	}
	lins := d.syn[:]
	copy(lins, d.qmf[:])
	clear(lins[qmfHistory:])
	right := granuleSize * (nch - 1)
	for i := 0; i < 18; i += 2 {
		synth(d.spec[i:], d.spec[right+i:], out[32*nch*i:], nch, lins[i*2*subbands:])
	}
	copy(d.qmf[:], lins[18*2*subbands:])
}

// synthPair windows the two outputs of a time slot pair that take no
// part in the slot's butterflies.
func synthPair(out []int16, at, nch int, z []float32) {
	const s = 64
	a := (z[14*s] - z[0]) * 29
	a += (z[1*s] + z[13*s]) * 213
	a += (z[12*s] - z[2*s]) * 459
	a += (z[3*s] + z[11*s]) * 2037
	a += (z[10*s] - z[4*s]) * 5153
	a += (z[5*s] + z[9*s]) * 6574
	a += (z[8*s] - z[6*s]) * 37489
	a += z[7*s] * 75038
	out[at] = pcm.Int16(a)

	z = z[2:]
	a = z[14*s] * 104
	a += z[12*s] * 1567
	a += z[10*s] * 9727
	a += z[8*s] * 64019
	a += z[6*s] * -9975
	a += z[4*s] * -45
	a += z[2*s] * 146
	a += z[0*s] * -5
	out[at+16*nch] = pcm.Int16(a)
}

// synth turns two time slots of left and right subband samples, xl and
// xr from their first slot, into 64 samples per channel, keeping them
// in lins, the window's history.
func synth(xl, xr []float32, out []int16, nch int, lins []float32) {
	const s = 64
	z := 15 * s
	l, r := 0, nch-1
	lins[z+4*15] = xl[18*16]
	lins[z+4*15+1] = xr[18*16]
	lins[z+4*15+2] = xl[0]
	lins[z+4*15+3] = xr[0]
	lins[z+4*31] = xl[1+18*16]
	lins[z+4*31+1] = xr[1+18*16]
	lins[z+4*31+2] = xl[1]
	lins[z+4*31+3] = xr[1]

	synthPair(out, r, nch, lins[4*15+1:])
	synthPair(out, r+32*nch, nch, lins[4*15+s+1:])
	synthPair(out, l, nch, lins[4*15:])
	synthPair(out, l+32*nch, nch, lins[4*15+s:])

	w := 0
	for i := 14; i >= 0; i-- {
		lins[z+4*i] = xl[18*(31-i)]
		lins[z+4*i+1] = xr[18*(31-i)]
		lins[z+4*i+2] = xl[1+18*(31-i)]
		lins[z+4*i+3] = xr[1+18*(31-i)]
		lins[z+4*(i+16)] = xl[1+18*(1+i)]
		lins[z+4*(i+16)+1] = xr[1+18*(1+i)]
		lins[z+4*(i-16)+2] = xl[18*(1+i)]
		lins[z+4*(i-16)+3] = xr[18*(1+i)]

		var a, b [4]float32
		for k := range 8 {
			w0, w1 := synthWindow[w], synthWindow[w+1]
			w += 2
			vz := lins[z+4*i-k*s:]
			vy := lins[z+4*i-(15-k)*s:]
			for j := range a {
				b[j] += vz[j]*w1 + vy[j]*w0
				if k&1 == 0 {
					a[j] += vz[j]*w0 - vy[j]*w1
				} else {
					a[j] += vy[j]*w1 - vz[j]*w0
				}
			}
		}
		out[r+(15-i)*nch] = pcm.Int16(a[1])
		out[r+(17+i)*nch] = pcm.Int16(b[1])
		out[l+(15-i)*nch] = pcm.Int16(a[0])
		out[l+(17+i)*nch] = pcm.Int16(b[0])
		out[r+(47-i)*nch] = pcm.Int16(a[3])
		out[r+(49+i)*nch] = pcm.Int16(b[3])
		out[l+(47-i)*nch] = pcm.Int16(a[2])
		out[l+(49+i)*nch] = pcm.Int16(b[2])
	}
}
