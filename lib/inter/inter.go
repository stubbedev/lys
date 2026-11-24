// Package inter embeds Inter Regular, the face lys draws text in when nothing
// else names one: the splash and lys-wm's tabs. LICENSE-Inter.txt is its
// licence, the SIL Open Font License 1.1, which travels with it.
package inter

import _ "embed"

// Regular is Inter Regular's TrueType file.
//
//go:embed Inter-Regular.ttf
var Regular []byte
