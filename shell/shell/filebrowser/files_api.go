package filebrowser

// Grid reports whether the files show as a grid, not a list.
func (c *Browser) Grid() bool { return c.s.view.grid }

// SetGrid shows the files as a grid, or as a list, as the header's
// toggle does; it does nothing outside a browse.
func (c *Browser) SetGrid(on bool) {
	if c.active && c.s.view.grid != on {
		c.toggleView()
	}
}

// Reload reads the folder shown again, keeping the scroll and the
// selection, as a change on disk the watch could not follow does; it
// does nothing outside a browse or while a listing is being read.
func (c *Browser) Reload() {
	if !c.active || c.s.loading {
		return
	}
	c.reloading, c.reloadKeep = true, c.selectedPath()
	c.s.reload()
}
