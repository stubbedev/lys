package bar

// levelIndexFloor is the battery's level math: the percentage
// divided evenly across n icons from the top, empty first.
func levelIndexFloor(percent float64, n int) int {
	if n <= 0 {
		return 0
	}
	idx := int(percent / 100.0 * float64(n))
	return min(idx, n-1)
}

// levelIndexSpan is the volume's level math: with n icons,
// percent 0 takes the first and 1..100 divide the rest evenly, so
// 1-33% lands on icons[0] with n=3, 34-66% on icons[1], 67-100% on
// icons[2].
func levelIndexSpan(percent int, n int) int {
	if n <= 0 {
		return 0
	}
	if percent <= 0 {
		return 0
	}
	step := 100.0 / float64(n)
	idx := int((float64(percent) - 1.0) / step)
	return min(idx, n-1)
}
