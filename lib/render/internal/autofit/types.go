package autofit

import (
	"github.com/stubbedev/lys/lib/third_party/typesetting/font/opentype/tables"
	"github.com/stubbedev/lys/lib/third_party/typesetting/language"
)

type uniRange struct{ first, last rune }

type script struct {
	hb              language.Script
	topToBottom     bool
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
	script   int
	writing  writingSystem
	blues    int
	coverage tables.Tag
}

type blueString struct {
	text       string
	properties uint16
}

type adjustment struct {
	codepoint rune
	flags     uint32
}
