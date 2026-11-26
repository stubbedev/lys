# AGENTS.md

lys-files is the lys desktop's file manager and its default app for folders
(`resources/dev.stubbe.lys.files.desktop`, `MimeType=inode/directory;`): a
window of tabs, each a browse of the shell's file browser
(`shell/shell/filebrowser`), with the file operations of `desktop/fileops`
behind its context menu, keys, the clipboard and drag and drop, a panel of the
operations running, the conflict dialog and undo. It serves
`org.freedesktop.FileManager1` on the session bus, started by D-Bus activation
(`resources/org.freedesktop.FileManager1.service`, `lys-files --service`), so
"show in folder" in other apps opens it. A second launch hands its paths to the
running one (`ui/app`'s single instance).

## Gates

Run before every commit, from the repository root:

```sh
just gates files
just packages lys-files
```

No test starts a compositor: the file manager is tested against a host the
tests drive, `lib/testhome` keeps every test away from the desktop session, and
the FileManager1 tests run on a private bus of `lib/dbustest`.

The repository's own rules (the vendor hash, CI, releases, the tracker) are in
the root `AGENTS.md`.

## Tests define behavior

Every feature and every bug fix lands with tests, positive and negative: a
behavior holds when it should, and when it should not, the failure is the
defined one. A fix without a test that fails before it is not a fix.

## Where things belong

- Browsing (places, views, thumbnails, search, Quick Look, dragging files out,
  the inline rename's entry) is the file browser's. lys-files adds what a file
  manager does with the files, through the browser's `Options` and methods.
- Copying, moving, deleting, trashing and undoing are `desktop/fileops`'; the
  file manager plans no file operation of its own.
- Clipboard and drag payloads are `filebrowser.FilesContent`'s and
  `ui/transfer`'s formats: `x-special/gnome-copied-files` and `text/uri-list`.
- An icon the window shows is one `resources/icons.txt` ships, in the app's own
  share directory; a test checks the list against what the code names.
- Every message lives in `internal/locale/locales`, in en-US, de, es, fr, it, nl
  and pt (European Portuguese).

## The tracker

GitHub issues on `stubbedev/lys`. Nothing is planned in local files.
