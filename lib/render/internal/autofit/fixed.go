package autofit

import "math/bits"

func abs(a int64) int64 {
	if a < 0 {
		return -a
	}
	return a
}

func moveSign(a int64, s int) (uint64, int) {
	if a < 0 {
		return uint64(-a), -s
	}
	return uint64(a), s
}

func applySign(d uint64, s int) int64 {
	v := int64(d) //nolint:gosec // a quotient of operands that fit in int64, as FreeType's FT_Long results
	if s < 0 {
		return -v
	}
	return v
}

func short(v int64) int16 {
	return int16(v) //nolint:gosec // FreeType stores these as FT_Short and truncates the same way
}

func mulFix(a, b int64) int64 {
	ab := a * b
	return (ab + 0x8000 + (ab >> 63)) >> 16
}

func divFix(a, b int64) int64 {
	ua, s := moveSign(a, 1)
	ub, s := moveSign(b, s)
	if ub == 0 {
		return applySign(0x7FFFFFFF, s)
	}
	return applySign(((ua<<16)+(ub>>1))/ub, s)
}

func mulDiv(a, b, c int64) int64 {
	ua, s := moveSign(a, 1)
	ub, s := moveSign(b, s)
	uc, s := moveSign(c, s)
	if uc == 0 {
		return applySign(0x7FFFFFFF, s)
	}
	return applySign((ua*ub+(uc>>1))/uc, s)
}

func msb(z uint32) int { return 31 - bits.LeadingZeros32(z) }

func pixRound(a int64) int64 { return (a + 32) &^ 63 }

func hypot(x, y int64) int64 {
	x, y = abs(x), abs(y)
	if x > y {
		return x + (3 * y >> 3)
	}
	return y + (3 * x >> 3)
}

func cornerIsFlat(inX, inY, outX, outY int64) bool {
	dIn := hypot(inX, inY)
	dOut := hypot(outX, outY)
	dHypot := hypot(inX+outX, inY+outY)
	return dIn+dOut-dHypot < dHypot>>4
}
