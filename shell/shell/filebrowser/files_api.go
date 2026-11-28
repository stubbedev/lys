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

// SelectAll selects every file the view showing lists, as Ctrl+A on it
// does; it does nothing outside a browse or when they all are.
func (c *Browser) SelectAll() {
	v := c.activeView()
	if c.active && len(v.Selection()) != len(c.s.entries) {
		v.SelectAll()
	}
}

// SelectNone clears the selection of the view showing; it does nothing
// outside a browse.
func (c *Browser) SelectNone() {
	if c.active {
		c.activeView().UnselectAll()
	}
}

// InvertSelection selects exactly the files of the view showing that
// were not selected; it does nothing outside a browse.
func (c *Browser) InvertSelection() {
	if c.active {
		c.activeView().InvertSelection()
	}
}
