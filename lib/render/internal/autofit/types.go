package autofit

type uniRange struct{ first, last rune }

type script struct {
	standard        string
	ranges, nonbase []uniRange
}

type writingSystem uint8

const (
	writingDummy writingSystem = iota
	writingLatin
	writingCJK
	writingIndic
)

type style struct {
	script  int
	writing writingSystem
	blues   int
}

type blueString struct {
	text       string
	properties uint16
}

type adjustment struct {
	codepoint rune
	flags     uint32
}
