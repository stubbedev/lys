---
layout: home

hero:
  name: lys-lib
  text: Building blocks in pure Go
  tagline: Rendering, Vulkan, Wayland capture, media, PipeWire, EIS and D-Bus with CGO_ENABLED=0.
  image:
    src: /lys-lib.svg
    alt: lys-lib
  actions:
    - theme: brand
      text: Package map
      link: /lib/overview
    - theme: alt
      text: Patch policy
      link: /lib/third-party/
    - theme: alt
      text: GitHub
      link: https://github.com/stubbedev/lys/tree/main/lib

features:
  - title: Rendering and GPU
    details: 2D canvas with CPU and Vulkan renderers, dmabufs, DRM ioctls.
    link: /lib/rendering
  - title: Wayland
    details: Screen and window capture, virtual input, protocol bindings.
    link: /lib/wayland
  - title: Media
    details: Hardware H.264/H.265, MP4 and Matroska, Opus, Vorbis, PCM mixing, V4L2.
    link: /lib/media
  - title: System services
    details: PipeWire video producer, EIS server, D-Bus, PAM.
    link: /lib/system
---
