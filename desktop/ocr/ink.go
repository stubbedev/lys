package ocr

import "image"

const (
	inkThreshold   = 0.45
	minContrast    = 40
	bgQuantShift   = 3
	bgQuantLevels  = 1 << (8 - bgQuantShift)
	contrastQuant  = 0.95
	channelsPerPix = 4
)

type inkMap struct {
	w, h int
	c    []float32
}

func newInkMap(img *image.RGBA) (inkMap, bool) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	m := inkMap{w: w, h: h, c: make([]float32, w*h)}
	if w == 0 || h == 0 {
		return m, false
	}
	bg := background(img)
	dist := make([]uint8, w*h)
	var hist [256]int
	for y := range h {
		row := img.Pix[img.PixOffset(b.Min.X, b.Min.Y+y):][:w*channelsPerPix]
		for x := range w {
			p := row[x*channelsPerPix:]
			d := max(absDiff(p[0], bg[0]), absDiff(p[1], bg[1]), absDiff(p[2], bg[2]))
			dist[y*w+x] = d
			hist[d]++
		}
	}
	inked := 0
	for d := minContrast; d < len(hist); d++ {
		inked += hist[d]
	}
	if inked == 0 {
		return m, false
	}
	target := int(float64(inked) * contrastQuant)
	high, seen := len(hist)-1, 0
	for d := minContrast; d < len(hist); d++ {
		seen += hist[d]
		if seen >= target {
			high = d
			break
		}
	}
	scale := 1 / float32(high)
	for i, d := range dist {
		m.c[i] = min(1, float32(d)*scale)
	}
	return m, true
}

func absDiff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

func background(img *image.RGBA) [3]uint8 {
	b := img.Bounds()
	var counts [bgQuantLevels * bgQuantLevels * bgQuantLevels]int
	var sums [len(counts)][3]int
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := img.Pix[img.PixOffset(b.Min.X, y):][:b.Dx()*channelsPerPix]
		for x := range b.Dx() {
			p := row[x*channelsPerPix:]
			k := int(p[0]>>bgQuantShift)*bgQuantLevels*bgQuantLevels + int(p[1]>>bgQuantShift)*bgQuantLevels + int(p[2]>>bgQuantShift)
			counts[k]++
			sums[k][0] += int(p[0])
			sums[k][1] += int(p[1])
			sums[k][2] += int(p[2])
		}
	}
	best := 0
	for k, n := range counts {
		if n > counts[best] {
			best = k
		}
	}
	n := counts[best]
	return [3]uint8{uint8(sums[best][0] / n), uint8(sums[best][1] / n), uint8(sums[best][2] / n)} //nolint:gosec // the mean of byte values is a byte
}
