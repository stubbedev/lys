package autofit

type uniRange struct{ first, last rune }

type script struct {
	standard        string
	ranges, nonbase []uniRange
}

type style struct {
	script int
	latin  bool
	blues  int
}

type blueString struct {
	text       string
	properties uint16
}

type adjustment struct {
	codepoint rune
	flags     uint32
}
