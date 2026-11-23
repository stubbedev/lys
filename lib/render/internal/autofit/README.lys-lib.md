# FreeType 2.14.3's light autohinter, ported to Go

A port of the auto-fitter in FreeType 2.14.3 (`src/autofit`) as FreeType
runs it for `FT_LOAD_TARGET_LIGHT`, the `hintslight` GTK and Qt render
with: vertical hinting only, stems keeping their widths, blue zones from
the script's reference characters, the x-height scale adjustment, and the
accent and dot separation of `afadjust.c`. FreeType Project License
(LICENSE). Portions of this software are copyright © 2026 The FreeType
Project (https://freetype.org). All rights reserved.

| File | FreeType source |
| --- | --- |
| `fixed.go` | `FT_MulFix`, `FT_DivFix`, `FT_MulDiv`, `ft_corner_is_flat` (`base/ftcalc.c`) |
| `hints.go` | `afhints.c`, `FT_Outline_Get_Orientation` (`base/ftoutln.c`) |
| `metrics.go` | `af_latin_metrics_init_widths`, `af_latin_metrics_init_blues` (`aflatin.c`) |
| `latin.go` | segments, links, edges, blue edges and edge hinting (`aflatin.c`) |
| `adjust.go` | tilde stretching and vertical separation (`aflatin.c`, `afadjust.c`) |
| `face.go` | style coverage (`afglobal.c`), metric scaling and `af_latin_hints_apply` |
| `tables.go` | `afranges.c`, `afblue.c`, `afscript.h`, `afstyles.h`, `afadjust.c`'s database, extracted unchanged |

Every mapped glyph of the five test faces in `render`, at 8 to 40 px, is
hinted to exactly FreeType's points: `render`'s
`TestAutofitHintsEveryMappedGlyphAsFreeTypesLightAutohinter` loads
libfreetype and compares.

## Not ported

- The CJK and Indic writing systems, and the CJK fallback style FreeType
  gives glyphs no script range covers: such glyphs draw unhinted.
- HarfBuzz coverage. FreeType built with HarfBuzz gives a glyph that a
  script's substitutions reach that script's style; here every glyph
  without a cmap entry takes the Latin style.
- Horizontal hinting, stem snapping and the monochrome and LCD modes,
  which light mode does not use.
- Stem darkening, which FreeType leaves off by default.
