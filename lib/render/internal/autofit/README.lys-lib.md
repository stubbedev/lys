# FreeType 2.14.3's light autohinter, ported to Go

A port of the auto-fitter in FreeType 2.14.3 (`src/autofit`) as FreeType
runs it for `FT_LOAD_TARGET_LIGHT`, the `hintslight` GTK and Qt render
with, and as it runs with HarfBuzz loaded, which FreeType 2.14 does at
runtime whenever libharfbuzz is present. What FreeType asks HarfBuzz is
answered in Go by `third_party/typesetting/harfbuzz`, so no C library is
involved. Its three writing systems: Latin hints only y, stems keeping their
widths, with blue zones from the script's reference characters, the
x-height scale adjustment and the accent and dot separation of
`afadjust.c`; CJK hints both axes against horizontal and vertical blue
zones, and also takes the glyphs no script's ranges cover, as FreeType's
fallback style; Indic is CJK without blue zones. FreeType Project License
(LICENSE). Portions of this software are copyright © 2026 The FreeType
Project (https://freetype.org). All rights reserved.

| File | FreeType source |
| --- | --- |
| `fixed.go` | `FT_MulFix`, `FT_DivFix`, `FT_MulDiv`, `ft_corner_is_flat` (`base/ftcalc.c`) |
| `hints.go` | `afhints.c`, `FT_Outline_Get_Orientation` (`base/ftoutln.c`) |
| `metrics.go` | `af_latin_metrics_init_widths`, `af_latin_metrics_init_blues` (`aflatin.c`) |
| `latin.go` | segments, links, edges, blue edges and edge hinting (`aflatin.c`) |
| `adjust.go` | tilde stretching and vertical separation (`aflatin.c`, `afadjust.c`) |
| `cjk.go` | the CJK and Indic writing systems (`afcjk.c`, `afindic.c`), run on both axes by transposing the hint points |
| `face.go` | style coverage (`afglobal.c`), metric scaling and `af_latin_hints_apply` |
| `layout.go` | the HarfBuzz side of `afshaper.c` (OpenType feature coverage of styles, shaping of the blue and standard character clusters), `afgsub.c` and the GSUB half of `af_reverse_character_map_new` (`afadjust.c`) |
| `fthash.go` | `FT_Hash` for numbers (`base/fthash.c`), whose bucket order decides which character a substituted glyph inherits |
| `tables.go` | `afranges.c`, `afblue.c`, `afscript.h` (with each script's HarfBuzz script and hinting direction), `afstyles.h` (with each style's OpenType feature), `afadjust.c`'s database, extracted unchanged |

`render`'s `TestAutofitHintsEveryGlyphAsFreeTypesLightAutohinter` loads
libfreetype with libharfbuzz and compares every hinted point, x and y,
of every glyph of Cantarell, Quicksand, Inter, Go, Go Mono, WenQuanYi
Micro Hei (CJK), Lohit Odia (Indic) and Libertinus Serif (CFF, with
small capitals, superscripts and subscripts) at 8 to 40 px: all
identical. `TestCFFOutlinesLoadAsFreeTypeLoadsThem` holds the CFF points
the hinter starts from to FreeType's: a move is emitted with the first
segment drawn from it, a contour of one point is dropped, and
coordinates are truncated after FreeType's 1/64 fixed-point scaling.

Shaping runs on the face's default instance with the language `c`, as
FreeType's HarfBuzz font does in a process that has not set a locale.

Quirks are kept because FreeType has them: `af_cjk_hints_compute_segments`
bounds its roundness pass before the segments exist, so the pass never
runs; the adjustment database is binary-searched although it is not
fully sorted; and `afgsub.c`'s coverage iterator yields only the first
glyph of a format 2 coverage's last range.

## Not ported

- HarfBuzz's limits on the scripts, language systems and feature indices
  one lookup collection visits, which guard against hostile fonts and
  are far above what any real font reaches.
- Horizontal Latin hinting, stem snapping and the monochrome and LCD
  modes, which light mode does not use.
- Stem darkening, which FreeType leaves off by default.
