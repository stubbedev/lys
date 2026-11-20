# pico's face detector from nenadmarkus/pico 7d550c78, ported for lys-power

[pico](https://github.com/nenadmarkus/pico) (MIT, LICENSE) finds faces by
comparing pixel intensities in a cascade of decision trees. lys-power's
attention check runs it on one greyscale camera frame before the idle dim.

- `facefinder` is pico's `rnt/cascades/facefinder` byte for byte (sha256
  `d8014993e7298c7b1865d1f8b855d6dbf4ec5c808bf879e2091ab6837abf90cd`),
  embedded in the binary, so nothing is downloaded at run time.
- `pico.go` is a Go port of `rnt/picornt.c`'s `run_cascade`, `find_objects`,
  `get_overlap` and `cluster_detections`, without cgo. The window loop and the
  overlap test work in integers, so the result is the same on every
  architecture; the rotated cascade and the detection memory across frames
  are not carried.
- The cascade is parsed and checked once, and a truncated or overlong file is
  an error rather than a read past its end.
