# FreeType 2.14.3's light autohinter, ported to Go

A port of the auto-fitter in FreeType 2.14.3 (`src/autofit`) as FreeType
runs it for `FT_LOAD_TARGET_LIGHT`, the `hintslight` GTK and Qt render
with. Its three writing systems: Latin hints only y, stems keeping their
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
| `tables.go` | `afranges.c`, `afblue.c`, `afscript.h`, `afstyles.h`, `afadjust.c`'s database, extracted unchanged |

`render`'s `TestAutofitHintsEveryMappedGlyphAsFreeTypesLightAutohinter`
loads libfreetype and compares every hinted point, x and y, of the
mapped glyphs of Cantarell, Quicksand, Inter, Go, Go Mono, WenQuanYi
Micro Hei (CJK) and Lohit Odia (Indic) at 8 to 40 px: all identical.

Two quirks are kept because FreeType has them: `af_cjk_hints_compute_segments`
bounds its roundness pass before the segments exist, so the pass never
runs; and the adjustment database is binary-searched although it is not
fully sorted.

## Not ported

- HarfBuzz coverage. FreeType built with HarfBuzz gives a glyph that a
  script's substitutions reach that script's style; here every glyph
  without a cmap entry takes the Latin style.
- Horizontal Latin hinting, stem snapping and the monochrome and LCD
  modes, which light mode does not use.
- Stem darkening, which FreeType leaves off by default.
