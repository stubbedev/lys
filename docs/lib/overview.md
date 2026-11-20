# Overview and package map

lys-lib is the part of the `github.com/stubbedev/lys` module that holds the
building blocks that are not widgets. It has no binary; import each package on
its own. It targets Linux.

```sh
go get github.com/stubbedev/lys
```

The Go version is the one in the root `go.mod`. Signatures are in the
[API reference](https://pkg.go.dev/github.com/stubbedev/lys/lib).

## Native libraries

Everything builds with `CGO_ENABLED=0`. These libraries are dlopened at run time
through [purego](https://github.com/ebitengine/purego):

| Library | Loaded by | If missing |
| --- | --- | --- |
| `libvulkan.so.1` | `third_party/vk`, for `gpu` | `gpu.Open` fails with `ErrNoVulkan`; draw on the CPU with `render` |
| `libva.so.2`, `libva-drm.so.2` | `internal/va`, for `gpu.Encoder` | Optional; the encoder needs the device's Vulkan video queue instead |
| `libpam.so.0`, libc | `pam` | `pam.Start` and `pam.CheckLibrary` fail |

`LYS_LIBVULKAN` names another Vulkan loader, and a build can pin one
(see [vk](/lib/third-party/vk#the-loader-path-can-be-pinned)).

Test helpers load `libpipewire-0.3.so.0` (`pipewire/pwtest`) and `libei.so.1`
(`eis/eistest`) as reference clients. `pipewire` and `eis` link neither.

## Package map

### Rendering and GPU

| Package | Purpose |
| --- | --- |
| [`render`](/lib/rendering#render) | 2D canvas and CPU rasterizer |
| [`text`](/lib/rendering#text) | Word and grapheme motion, bidirectional runs |
| [`gpu`](/lib/rendering#gpu) | Vulkan devices, GPU renderer, dmabuf export |
| [`motion`](/lib/rendering#motion) | Spring and cubic-bezier solvers |
| [`drm`, `fourcc`, `edid`](/lib/rendering#drm-fourcc-edid) | DRM ioctls, pixel formats and modifiers, EDID parsing |
| [`dmabuf`, `dmabufwl`, `udmabuf`, `dmabufprobe`](/lib/rendering#dmabufs) | dmabuf descriptions, linux-dmabuf plumbing, memfd-backed allocation, modifier probing |

### Wayland

| Package | Purpose |
| --- | --- |
| [`capture`](/lib/wayland#capture) | Screen, window and cursor capture |
| [`vinput`, `keymapfd`](/lib/wayland#vinput-keymapfd) | Virtual pointer, keyboard and touch; reading a keymap fd |
| [`xcursor`](/lib/wayland#xcursor) | Xcursor files and themes |
| [`xkb`](/lib/xkb) | The XKB keyboard model, plus `xkb/registry` and `xkb/x11` |
| [`wlr`](/lib/wayland#wlr) | Protocol bindings beyond the core and xdg-shell |
| [`third_party/wayland`](/lib/third-party/wayland) | Core client library and scanner |
| `protocols` | The Wayland protocol XML every lys binding is generated from |

### Media

| Package | Purpose |
| --- | --- |
| [`gpu.Encoder`](/lib/media#video-encoding) | H.264 and H.265 hardware encoding |
| [`media/mux`](/lib/media#media-mux) | Fragmented MP4 and Matroska muxing |
| [`media/demux`](/lib/media#media-demux) | MP4 and Matroska demuxing with keyframe seeking |
| [`media/opus`](/lib/media#media-opus) | Opus encoder and decoder (CELT, SILK, hybrid, multistream) |
| [`media/aac`](/lib/media#media-aac) | AAC-LC decoder |
| [`media/vorbis`](/lib/media#media-vorbis) | Vorbis decoder, Ogg files or packets |
| [`audiomix`](/lib/media#audiomix) | 16-bit PCM mixer |
| [`v4l2`](/lib/media#v4l2) | Webcam capture |

### System services

| Package | Purpose |
| --- | --- |
| [`pipewire`](/lib/system#pipewire) | ScreenCast video producer over PipeWire's native protocol |
| [`eis`](/lib/system#eis) | EIS server (the receiving end of libei) |
| [`dbusx`](/lib/system#dbusx) | D-Bus services, signal watches and the system-bus dial on [patched godbus](/lib/third-party/godbus) |
| [`pulse/native`](/lib/system#pulse-native) | PulseAudio native-protocol client: introspection, control, subscriptions, record and playback streams, sample cache |
| [`logind`](/lib/system#logind) | systemd-logind client: sessions, power, inhibitors, devices, lock signals |
| [`polkit`](/lib/system#polkit) | polkitd Authority client: typed subjects, authorization checks with cancellation, agent registration |
| [`portaldialogs`](/lib/system#portaldialogs) | The `dev.stubbe.lys.PortalDialogs1` contract |
| [`wmipc`](/lib/system#wmipc) | The wire contract of lys-wm's IPC socket |
| [`pam`](/lib/system#pam) | libpam transactions |
| [`shacrypt`](/lib/system#shacrypt) | SHA-512 crypt(3) password hashes |
| [`appearance`, `portalsettings`](/lib/system#appearance-portalsettings) | Desktop preferences as xdg-desktop-portal publishes them |
| [`icons`, `icontheme`](/lib/system#icons-icontheme) | Freedesktop icon theme lookup and raster cache |
| [`i18n`](/lib/system#i18n) | gettext catalogs in pure Go |
| [`glibclocale`](/lib/system#glibclocale) | glibc's compiled locale data and strftime in pure Go |
| `vaultserver` | A validated Bitwarden server address |
| [`clib`, `intconv`, `atomicfile`, `assetfile`, `xdg`, `peercred`, `procstat`, `debug`, `logutil`, `testbuild`, `wallclock`](/lib/system#small-helpers) | Shared helpers |

### Test support

[`golden`, `gputest`, `wltest`, `pipewire/pwtest`, `eis/eistest`, `dbustest`, `pulse/pulsetest`, `logind/logindtest`, `polkit/polkittest`, `peercred/peercredtest`, `wallclock/wallclocktest`, `sockdir`](/lib/development#test-support)
can be imported from any test.

### Third party

[`third_party/`](/lib/third-party/) holds patched copies of godbus, Go's `math`,
oksvg, rasterx, go-text/typesetting, x/image/vector, the Vulkan bindings and the
Wayland client library.
