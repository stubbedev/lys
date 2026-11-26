package land

import (
	_ "embed"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"github.com/stubbedev/lys/lib/render"
)

//go:generate go run ./internal/genland -src source/ne_110m_land.geojson -out land.bin

//go:embed land.bin
var asset []byte

const (
	stepsPerDegree = 20
	degreesWide    = 360
	degreesHigh    = 180
	stepsWide      = degreesWide * stepsPerDegree
	stepsHigh      = degreesHigh * stepsPerDegree
	minRingPoints  = 3
)

type step struct{ x, y int64 }

type ring []step

var errTruncated = errors.New("the data ends inside a ring")

var decoded = sync.OnceValue(func() []ring {
	rings, err := decode(asset)
	if err != nil {
		panic(fmt.Errorf("decode the embedded land asset: %w", err))
	}
	return rings
})

func encode(rings []ring) []byte {
	out := binary.AppendUvarint(nil, uint64(len(rings)))
	var at step
	for _, r := range rings {
		out = binary.AppendUvarint(out, uint64(len(r)))
		for _, p := range r {
			out = binary.AppendVarint(out, p.x-at.x)
			out = binary.AppendVarint(out, p.y-at.y)
			at = p
		}
	}
	return out
}

type reader struct {
	data []byte
	at   int
}

func (r *reader) uvarint() (uint64, error) {
	v, n := binary.Uvarint(r.data[r.at:])
	if n <= 0 {
		return 0, fmt.Errorf("at byte %d: %w", r.at, errTruncated)
	}
	r.at += n
	return v, nil
}

func (r *reader) varint() (int64, error) {
	v, n := binary.Varint(r.data[r.at:])
	if n <= 0 {
		return 0, fmt.Errorf("at byte %d: %w", r.at, errTruncated)
	}
	r.at += n
	return v, nil
}

func decode(data []byte) ([]ring, error) {
	rd := &reader{data: data}
	count, err := rd.uvarint()
	if err != nil {
		return nil, fmt.Errorf("read the ring count: %w", err)
	}
	if count > uint64(len(data)) {
		return nil, fmt.Errorf("%d rings cannot fit in %d bytes", count, len(data))
	}
	rings := make([]ring, 0, count)
	var at step
	for i := range count {
		n, err := rd.uvarint()
		if err != nil {
			return nil, fmt.Errorf("read ring %d's length: %w", i, err)
		}
		if n < minRingPoints || n > uint64(len(data)) {
			return nil, fmt.Errorf("ring %d has %d points, not %d to %d", i, n, minRingPoints, len(data))
		}
		r := make(ring, n)
		for j := range r {
			dx, err := rd.varint()
			if err != nil {
				return nil, fmt.Errorf("read ring %d's point %d: %w", i, j, err)
			}
			dy, err := rd.varint()
			if err != nil {
				return nil, fmt.Errorf("read ring %d's point %d: %w", i, j, err)
			}
			at = step{at.x + dx, at.y + dy}
			if at.x < 0 || at.x > stepsWide || at.y < 0 || at.y > stepsHigh {
				return nil, fmt.Errorf("ring %d's point %d, %d east and %d north of 180°W 90°S in 1/%d°, is off the globe", i, j, at.x, at.y, stepsPerDegree)
			}
			r[j] = at
		}
		rings = append(rings, r)
	}
	if rd.at != len(data) {
		return nil, fmt.Errorf("%d bytes follow the last ring", len(data)-rd.at)
	}
	return rings, nil
}

func (p step) latitude() float64  { return float64(p.y)/stepsPerDegree - degreesHigh/2 }
func (p step) longitude() float64 { return float64(p.x)/stepsPerDegree - degreesWide/2 }

func Path(project func(latitude, longitude float64) (x, y float64)) *render.Path {
	p := &render.Path{}
	for _, r := range decoded() {
		p.MoveTo(project(r[0].latitude(), r[0].longitude()))
		for _, pt := range r[1:] {
			p.LineTo(project(pt.latitude(), pt.longitude()))
		}
		p.Close()
	}
	return p
}
