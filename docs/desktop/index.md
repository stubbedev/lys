# lys-desktop

The packages lys-shell, lys-login and lys-settings share, under
`github.com/stubbedev/lys/desktop`. It builds on [lys-ui](/ui/) and
[lys-lib](/lib/). Its API is shaped by those three apps and changes with
them; [lys-ui](/ui/) and [lys-lib](/lib/) are the libraries meant for other
programs.

```sh
go get github.com/stubbedev/lys
```

| Package | What it holds |
| --- | --- |
| `config` | lys-shell's config: discovery, imports, the types of every section, the JSON schema and the descriptions the [config reference](/shell/config/) is generated from |
| `styling` | Design tokens, palettes, themes and the colour providers (Matugen, Pywal, Wallust) |
| `i18n` | Fluent localization in English (US), German, Spanish, French, Italian, Dutch and European Portuguese, picked from `LANGUAGE`, `LC_ALL`, `LC_MESSAGES` and `LANG` by language (Brazilian Portuguese reads European Portuguese) with English as the fallback |
| `icons` | Icon sources (Tabler, Simple Icons, Material, Lucide), fetching and installing them |
| `service/...` | Clients of the session's services: audio over the PulseAudio protocol, MPRIS, the kernel's power supplies, notifications, idle, night light, clipboard, wallpaper, Bitwarden, the lys-wm IPC |
| `shell/...` | Widgets and helpers shared by the shell and the lock screen |
| `shell/barview` | The bar drawn from a config, without services: its zones, module buttons, separator and workspace strip, which lys-shell fills with live modules and lys-settings previews |
| `shell/osdview` | The on-screen display drawn from a config and an event, without services: the card lys-shell maps on a layer surface and lys-settings previews |
| `shell/notifyview` | The notification popup card drawn from a config and what a notification shows, without services, with the helpers the popups and the notification dropdown share; lys-shell stacks it and lys-settings previews it |
| `shell/cavaview` | The cava visualizer drawn from a config and a frame, without capture or analysis: the bars, wave or falling caps lys-shell paints from its analyzer and lys-settings previews with a sample frame |
| `launchermatch` | The launcher's matcher: rofi's matching methods, case handling and sorting over a port of nucleo's fzf scoring, and the options `launcher` configures, which lys-shell filters its lists with and lys-settings' launcher preview matches its test query by |
| `shell/launcherview` | The launcher drawn from a config, without the engine, socket or surface: the panel and where it sits, its field, footer, list, rows and mode tabs, and the CSS a font or style preset adds, which lys-shell hosts on a layer surface and lys-settings previews |
| `shell/lockview` | The login screen drawn from a config, without PAM, input grabbing or a session: the background, clock, account and password prompt, the lock screen's media row, battery chip and switch-user button, and the greeter's user list and power buttons, which the greeter and the lock screen show and lys-settings previews |
| `settingspage` | The contract between lys-settings and the programs that open it: its command line and page list |
| `identity` | The desktop's directory name under each XDG base directory |
| `desktopentry`, `mime`, `opener` | Desktop entries, the shared-mime-info database, and opening a file with the default app |
| `spawn`, `shlex`, `strftime`, `glob`, `jinja`, `pango` | Running configured commands, splitting command lines, time formats, blocklist matching, the label templates module formats are written in, Pango markup read into lys-ui's rich labels |
| `thumbnail` | Thumbnails for every surface, all from lys's own decoders: the freedesktop cache, pictures decoded in process, documents drawn by `lys-docthumb` and videos' frames |
| `dochelper`, `docrender` | The contract of `lys-docthumb` (its command line, exit statuses and document types), and the renderer only it links |
| `fileops` | The file manager's operations: copy, move, rename, delete, new files and folders and the freedesktop trash, as cancellable jobs that report progress, ask how to resolve conflicts and can be undone |
| `argpaths` | The files a desktop app's command line names: paths, relative or not, and `file://` URIs, as a file manager and a desktop entry's `%f` and `%u` hand them over |
| `ocr` | Reading the text in a screen image by matching it against glyphs `render` draws in the fonts it is given, with no language model or external program |

The package documentation is on
[pkg.go.dev](https://pkg.go.dev/github.com/stubbedev/lys/desktop).

## Document thumbnails

`lys-docthumb` (`desktop/cmd/lys-docthumb`) draws the first page of a PDF,
EPUB, MOBI or AZW3, FB2, CHM, DOCX, PPTX or XLSX file with
[folio](https://github.com/gen2brain/folio), pinned at exactly v0.3.1. folio
is pure Go (no cgo, `unsafe`, `os/exec` or `net`; its rasterizer has Go
assembly kernels), but it parses untrusted input with no deadline, so nothing
but `lys-docthumb` links it: `docrender` wraps it, and `thumbnail`'s tests fail
if the thumbnailer's binary links it.

The thumbnailer runs nothing but `lys-docthumb`: no installed `.thumbnailer`
program and no thumbnail command of the user's (rofi's `-preview-cmd` and the
`launcher.preview-cmd` key are refused). A picture is decoded in process within
the decode budget and turned upright as its EXIF orientation says; a video's
thumbnail is a frame a tenth into it: a YUV4MPEG2 stream decoded in process,
any other video's keyframe there decoded on the GPU and read back fitted to the
thumbnail (`media.KeyframeImage`, `gpu.VideoFrame.ReadRGBA`), by one worker that
owns the device and closes it after 10 s idle; a video no GPU here decodes keeps
its icon (`thumbnail.ErrVideoOnGPU`, with the GPU's reason); any other type keeps its icon
(`thumbnail.ErrNoDecoder`).

For the types `dochelper` lists, the thumbnailer runs `lys-docthumb` as a plain
child process: an empty environment, a process group of its own killed whole at
the 10 s timeout, and its output in a temporary directory of its own. The
helper limits its own data segment (`RLIMIT_DATA`; `RLIMIT_AS` would count the
address space Go reserves) to `thumbnail.DocumentMemory`, 768 MiB, before it
opens the file, and the garbage collector to a third of that
(`dochelper.LimitMemory`). Its exit status says why it failed: a damaged,
encrypted, too large (the limit included) or unsupported document is
`dochelper.ErrDamaged`, `ErrEncrypted`, `ErrTooLarge` or `ErrUnsupported`. The
font directories lys finds (fontconfig's, the XDG data directories' and the
usual system ones) are passed in by path. A failed thumbnail is recorded in the
freedesktop cache's `fail/` directory, so it is not tried again until the file
changes.

The helper is looked up next to the running program, so lys-shell and
lys-viewer install it beside their binaries; a path to it that is not an
absolute one to a file called `lys-docthumb` is refused. Where it is missing,
documents have no thumbnail (`thumbnail.ErrNoHelper`) and keep their icon, and
the thumbnailer logs why once.

### Regression suite

`desktop/docrender/testdata` holds the small fixtures every test run checks:
page counts, a non-blank first page and a block comparison with MuPDF 1.28.5's
render of it (`SOURCE.txt` says where each file comes from). Before folio is
bumped, also run the evaluation corpus of #392, which the repository does not
hold:

| File | Source |
| --- | --- |
| lshort.pdf | <https://mirrors.ctan.org/info/lshort/english/lshort.pdf> |
| pgfmanual.pdf | <https://mirrors.ctan.org/graphics/pgf/base/doc/generic/pgf/pgfmanual.pdf> (page 778 holds every shading type) |
| ctex.pdf | <https://mirrors.ctan.org/language/chinese/ctex/ctex.pdf> |
| f1040.pdf | <https://www.irs.gov/pub/irs-pdf/f1040.pdf> |
| wikipdf.pdf | <https://en.wikipedia.org/api/rest_v1/page/pdf/PDF> |
| LVRR 1874 Report | <https://archive.org/details/LVRR_1874_Report> (the PDF) |
| comptesrendusdel00athn_10 | <https://archive.org/details/comptesrendusdel00athn_10> (the PDF) |
| b97429879 | <https://archive.org/details/b97429879> (the PDF) |

1. Fetch each file into a directory of its own, outside the repository.
2. Build the helper with the new folio: `go build -o lys-docthumb ./desktop/cmd/lys-docthumb`.
3. For every file, run `lys-docthumb -size 1024 -memory 805306368 -- <file> <file>.png`
   and `mutool draw -c rgb -w 1024 -h 1024 -o <file>.mupdf.png <file> 1`, each
   under `timeout 10`. Every run exits 0 within the timeout; compare each pair
   side by side for missing content (text, shadings, form widgets, scans).
4. Truncate and flip bytes in a few of them and run the helper again: every run
   ends with status 0 or one of the typed statuses (3 to 6), never a timeout or
   a crash.
5. Read the diff of folio between the two versions, then update the fixtures'
   references only if MuPDF's render of them changed.
