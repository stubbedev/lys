#include <errno.h>
#include <fcntl.h>
#include <signal.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <time.h>
#include <unistd.h>
#include <drm_fourcc.h>
#include <xf86drm.h>
#include <xf86drmMode.h>

#define PLANES 32
#define RESAMPLE_EVERY 16

struct seen {
  uint32_t plane, crtc, fb;
  char summary[160];
};

static volatile sig_atomic_t stop;

static void on_signal(int sig) {
  (void)sig;
  stop = 1;
}

static void die(const char *what, const char *detail) {
  fprintf(stderr, "scanout-watch: %s %s: %s\n", what, detail, strerror(errno));
  exit(1);
}

static uint64_t now_us(void) {
  struct timespec ts;
  clock_gettime(CLOCK_MONOTONIC, &ts);
  return (uint64_t)ts.tv_sec * 1000000 + (uint64_t)ts.tv_nsec / 1000;
}

static void owner(const char *debugfs, uint32_t fb, char *out, size_t size) {
  snprintf(out, size, "?");
  char path[4096];
  snprintf(path, sizeof path, "%s/framebuffer", debugfs);
  FILE *f = fopen(path, "r");
  if (!f)
    die("open", path);
  char want[64], line[512];
  snprintf(want, sizeof want, "framebuffer[%u]:", fb);
  int found = 0;
  while (fgets(line, sizeof line, f)) {
    if (!found) {
      found = strncmp(line, want, strlen(want)) == 0;
      continue;
    }
    if (strncmp(line, "framebuffer[", 12) == 0)
      break;
    char *by = strstr(line, "allocated by = ");
    if (by) {
      by += strlen("allocated by = ");
      by[strcspn(by, "\n")] = 0;
      snprintf(out, size, "%s", by);
      break;
    }
  }
  fclose(f);
}

static void describe(int fd, const char *debugfs, uint32_t fb_id, char *out, size_t size) {
  if (fb_id == 0) {
    snprintf(out, size, "off");
    return;
  }
  char by[64];
  owner(debugfs, fb_id, by, sizeof by);
  drmModeFB2 *fb = drmModeGetFB2(fd, fb_id);
  if (!fb) {
    snprintf(out, size, "by=%s gone", by);
    return;
  }
  int n = snprintf(out, size, "by=%s %ux%u %.4s mod=%#llx", by, fb->width, fb->height, (const char *)&fb->pixel_format,
                   (unsigned long long)fb->modifier);
  int rgb32 = fb->pixel_format == DRM_FORMAT_XRGB8888 || fb->pixel_format == DRM_FORMAT_ARGB8888;
  int linear = fb->modifier == DRM_FORMAT_MOD_LINEAR || fb->modifier == DRM_FORMAT_MOD_INVALID;
  struct drm_mode_map_dumb map = {.handle = fb->handles[0]};
  if (!rgb32 || !linear || fb->handles[0] == 0 || drmIoctl(fd, DRM_IOCTL_MODE_MAP_DUMB, &map)) {
    snprintf(out + n, size - (size_t)n, " content=unread");
  } else {
    size_t len = (size_t)fb->pitches[0] * fb->height + fb->offsets[0];
    const uint8_t *pix = mmap(NULL, len, PROT_READ, MAP_SHARED, fd, (off_t)map.offset);
    if (pix == MAP_FAILED) {
      snprintf(out + n, size - (size_t)n, " content=unmapped");
    } else {
      uint64_t sum[3] = {0, 0, 0}, dark = 0, count = (uint64_t)fb->width * fb->height;
      for (uint32_t y = 0; y < fb->height; y++) {
        const uint8_t *row = pix + fb->offsets[0] + (size_t)y * fb->pitches[0];
        for (uint32_t x = 0; x < fb->width; x++) {
          const uint8_t *p = row + (size_t)x * 4;
          sum[0] += p[2];
          sum[1] += p[1];
          sum[2] += p[0];
          dark += p[0] < 16 && p[1] < 16 && p[2] < 16;
        }
      }
      snprintf(out + n, size - (size_t)n, " mean=%llu,%llu,%llu dark=%llu", (unsigned long long)(sum[0] / count),
               (unsigned long long)(sum[1] / count), (unsigned long long)(sum[2] / count),
               (unsigned long long)(dark * 1000 / count));
      munmap((void *)pix, len);
    }
  }
  for (int i = 0; i < 4; i++)
    if (fb->handles[i] && (i == 0 || fb->handles[i] != fb->handles[0]))
      drmCloseBufferHandle(fd, fb->handles[i]);
  drmModeFreeFB2(fb);
}

int main(int argc, char **argv) {
  if (argc != 3) {
    fprintf(stderr, "usage: scanout-watch CARD DEBUGFS-DIR\n"
                    "prints a line, with the monotonic time in microseconds, whenever a plane's framebuffer, CRTC or content changes, until SIGTERM\n");
    return 2;
  }
  signal(SIGTERM, on_signal);
  signal(SIGINT, on_signal);
  int fd = open(argv[1], O_RDWR | O_CLOEXEC);
  if (fd < 0)
    die("open", argv[1]);
  if (drmSetClientCap(fd, DRM_CLIENT_CAP_UNIVERSAL_PLANES, 1))
    die("enable universal planes on", argv[1]);
  struct seen seen[PLANES];
  memset(seen, 0, sizeof seen);
  setvbuf(stdout, NULL, _IOLBF, 0);
  printf("%llu start\n", (unsigned long long)now_us());
  for (unsigned tick = 0; !stop; tick++) {
    drmModePlaneRes *planes = drmModeGetPlaneResources(fd);
    if (!planes)
      die("list the planes of", argv[1]);
    for (uint32_t i = 0; i < planes->count_planes && i < PLANES; i++) {
      drmModePlane *p = drmModeGetPlane(fd, planes->planes[i]);
      if (!p)
        die("get a plane of", argv[1]);
      struct seen *s = &seen[i];
      int moved = s->plane != p->plane_id || s->crtc != p->crtc_id || s->fb != p->fb_id;
      if (moved || (p->fb_id && tick % RESAMPLE_EVERY == 0)) {
        char summary[sizeof s->summary];
        describe(fd, argv[2], p->fb_id, summary, sizeof summary);
        if (moved || strcmp(summary, s->summary) != 0) {
          printf("%llu plane=%u crtc=%u fb=%u %s\n", (unsigned long long)now_us(), p->plane_id, p->crtc_id, p->fb_id, summary);
          s->plane = p->plane_id;
          s->crtc = p->crtc_id;
          s->fb = p->fb_id;
          memcpy(s->summary, summary, sizeof summary);
        }
      }
      drmModeFreePlane(p);
    }
    drmModeFreePlaneResources(planes);
    struct timespec pause = {.tv_sec = 0, .tv_nsec = 1000000};
    nanosleep(&pause, NULL);
  }
  printf("%llu stop\n", (unsigned long long)now_us());
  close(fd);
  return 0;
}
