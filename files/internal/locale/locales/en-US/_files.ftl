### lys-files

files-title = Files
files-new-tab = New Tab
files-open = Open
files-open-in-new-tab = Open in New Tab
files-open-with = Open With
files-no-apps = No app opens these files
files-cut = Cut
files-copy = Copy
files-paste = Paste
files-rename = Rename
files-trash = Move to Trash
files-delete = Delete Permanently
files-new-folder = New Folder
files-new-file = New File
files-folder-name = Folder name
files-file-name = File name
files-create = Create
files-terminal = Open Terminal Here
files-copy-path = Copy Path
files-properties = Properties
files-selection = Selection
files-select-all = Select All
files-select-none = Select None
files-restore = Restore
files-empty-trash = Empty Trash
files-undo = Undo
files-cancel = Cancel
files-close = Close
files-close-running-title = { $count ->
    [one] An operation is still running
   *[other] { $count } operations are still running
}
files-close-running-body = Closing the window cancels what has not finished.
files-keep-open = Keep Open
files-stop-and-close = Cancel and Close
files-skip = Skip
files-keep-both = Keep Both
files-replace = Replace
files-conflict-title = { $name } is already in { $folder }
files-conflict-file = Replace the file there, keep both, or skip this one?
files-conflict-folder = Replace the folder there and all it holds, keep both, or skip this one?
files-conflict-all = Do this for every conflict
files-delete-title = { $count ->
    [one] Delete { $name } permanently?
   *[other] Delete { $count } items permanently?
}
files-delete-body = This cannot be undone.
files-delete-no-trash-body = A network location has no trash: the files are deleted for good. This cannot be undone.
files-delete-confirm = Delete
files-empty-title = Empty the trash?
files-progress-copy = Copying { $done } of { $of }
files-progress-move = Moving { $done } of { $of }
files-progress-delete = Deleting { $done } of { $of }
files-progress-trash = Moving { $done } of { $of } to the trash
files-progress-empty = Emptying the trash: { $done } of { $of }
files-progress-undo = Undoing: { $done } of { $of }
files-copy-failed = The files could not all be copied
files-move-failed = The files could not all be moved
files-delete-failed = The files could not all be deleted
files-trash-failed = The files could not all be moved to the trash
files-empty-failed = The trash could not be emptied
files-undo-failed = The last change could not be undone
files-restore-failed = The files could not all be restored
files-rename-failed = { $name } could not be renamed
files-create-failed = { $name } could not be created
files-clipboard-failed = The clipboard could not be used
files-paste-empty = There are no files to paste
files-nothing-to-undo = There is nothing to undo
files-missing = { $path } is not there
files-open-failed = { $name } could not be opened
files-sent-back = { $name } was saved to { $server }
files-send-back-failed = { $name } could not be saved to { $server }
files-conflict-remote-title = { $name } changed on { $server }
files-conflict-remote-body = It changed there since it was opened. Replace it with the version saved here, or keep the one on the server?
files-keep-server = Keep the Server's
files-close-edits-title = { $count ->
    [one] A file opened from a network location is still open
   *[other] { $count } files opened from network locations are still open
}
files-close-edits-body = Once the window closes, changes saved to them stay on this computer and are not sent back.
files-open-with-failed = { $count ->
    [one] { $app } could not open { $name }
   *[other] { $app } could not open the { $count } items
}
files-terminal-failed = No terminal could be opened
files-properties-failed = The properties could not be read
files-trashed = { $count ->
    [one] { $name } was moved to the trash
   *[other] { $count } items were moved to the trash
}
files-prop-title = { $count ->
    [one] { $name }
   *[other] { $count } items
}
files-prop-name = Name
files-prop-type = Type
files-prop-location = Location
files-prop-size = Size
files-prop-contents = { $size }, { $count ->
    [one] one item
   *[other] { $count } items
}
files-prop-modified = Modified
files-prop-permissions = Permissions
files-status-items = { $count ->
    [0] No items
    [one] One item
   *[other] { $count } items
}
files-status-selected = { $total ->
    [one] One item selected
   *[other] { $count ->
        [one] One of { $total } items selected
       *[other] { $count } of { $total } items selected
    }
}
files-status-size = { $selection } ({ $size })
