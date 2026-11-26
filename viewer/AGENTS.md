# AGENTS.md

lys-viewer is the lys desktop's default app for pictures, videos and documents
(`resources/dev.stubbe.lys.viewer.desktop`): it shows one file at a time in the
shared preview (`desktop/shell/preview`, filling the window), steps through the
others of its folder in the order the file browser sorts them, and moves files to
the trash with undo, copies a picture, opens a file in another app and goes full
screen.

## Gates

Run before every commit, from the repository root:

```sh
just gates viewer
just packages lys-viewer
```

No test starts a compositor, a bus or a sound server: the viewer is tested
against a host the tests drive, and `lib/testhome` keeps every test away from the
desktop session.

The repository's own rules (the vendor hash, CI, releases, the tracker) are in
the root `AGENTS.md`.

## Tests define behavior

Every feature and every bug fix lands with tests, positive and negative: a
behavior holds when it should, and when it should not, the failure is the defined
one. A fix without a test that fails before it is not a fix.

## Where things belong

- How a file shows, and what it says when it cannot, is the preview's. The
  viewer adds no renderer and no message of its own for a file.
- The types the viewer claims are derived from the preview's picture types and
  demux's containers (`internal/viewer/types.go`); the `.desktop` file's
  `MimeType=` must list exactly them, which a test checks.
- An icon the viewer or the preview shows is one `resources/icons.txt` ships, in
  the app's own share directory.
- Every message lives in `internal/locale/locales`, in en-US, de, es, fr, it, nl
  and pt (European Portuguese).

## The tracker

GitHub issues on `stubbedev/lys`. Nothing is planned in local files.
