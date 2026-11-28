package filebrowser

import (
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"
)

// viewMenuKey runs the view menu's command bound to sym with exactly
// held of Ctrl, Alt, Super and Shift down, reporting whether one was:
// the layouts, the reload and, where several files can be selected,
// the selection commands.
func (c *Browser) viewMenuKey(sym xkb.Keysym, held app.Mods) bool {
	ctrl, ctrlShift := held == app.ModCtrl, held == app.ModCtrl|app.ModShift
	multiple := c.o.Selection == widget.SelectionMultiple
	switch {
	case ctrl && sym == '1':
		c.SetGrid(false)
	case ctrl && sym == '2':
		c.SetGrid(true)
	case held == 0 && sym == xkb.KeyF5, ctrl && sym == 'r':
		c.Reload()
	case multiple && ctrl && sym == 'a':
		c.SelectAll()
	case multiple && ctrlShift && sym == 'a':
		c.SelectNone()
	case multiple && ctrlShift && sym == 'i':
		c.InvertSelection()
	default:
		return false
	}
	return true
}

// openViewMenu opens the header's view menu under its button, the
// button :checked until the menu closes.
func (c *Browser) openViewMenu() {
	if !c.active {
		return
	}
	b := c.ui.viewMenu
	b.SetState(widget.StateChecked, true)
	widget.OpenMenu(b, b, c.d.Font, widgets.TextPx(styling.Body, 1), c.viewMenu(), widget.MenuOptions{
		Gap: widget.PopoverGap, OnClosed: func() { b.SetState(widget.StateChecked, false) },
	})
}

// viewMenu is the header's menu as the browse stands: the layout, the
// sort column and direction, hidden files and the reload, then, where
// several files can be selected, the selection commands.
func (c *Browser) viewMenu() []widget.MenuItem {
	grid, v := c.s.view.grid, c.s.view
	items := []widget.MenuItem{
		{Label: i18n.T("file-chooser-menu-list"), Kind: widget.ItemRadio, Group: "layout", Checked: !grid, Accel: "Ctrl+1", OnClick: func() { c.SetGrid(false) }},
		{Label: i18n.T("file-chooser-menu-grid"), Kind: widget.ItemRadio, Group: "layout", Checked: grid, Accel: "Ctrl+2", OnClick: func() { c.SetGrid(true) }},
		widget.MenuSeparator(),
	}
	for col, title := range columnTitles {
		col := SortColumn(col)
		items = append(items, widget.MenuItem{
			Label: i18n.T(title), Kind: widget.ItemRadio, Group: "sort", Checked: v.sort == col,
			OnClick: func() { c.setSort(col, c.s.view.asc) },
		})
	}
	items = append(items,
		widget.MenuSeparator(),
		widget.MenuItem{Label: i18n.T("file-chooser-menu-ascending"), Kind: widget.ItemRadio, Group: "order", Checked: v.asc, OnClick: func() { c.setSort(c.s.view.sort, true) }},
		widget.MenuItem{Label: i18n.T("file-chooser-menu-descending"), Kind: widget.ItemRadio, Group: "order", Checked: !v.asc, OnClick: func() { c.setSort(c.s.view.sort, false) }},
		widget.MenuSeparator(),
		widget.MenuItem{Label: i18n.T("file-chooser-menu-hidden"), Kind: widget.ItemCheck, Checked: c.hidden, Accel: "Ctrl+H", OnClick: c.flipHidden},
		widget.MenuItem{Label: i18n.T("file-chooser-menu-reload"), Accel: "F5", OnClick: c.Reload, Disabled: c.s.loading},
	)
	if c.o.Selection != widget.SelectionMultiple {
		return items
	}
	return append(append(items, widget.MenuSeparator()), c.SelectionItems()...)
}

// SelectAllItem is the menu item selecting every file the view showing
// lists, disabled when they all are.
func (c *Browser) SelectAllItem() widget.MenuItem {
	return widget.MenuItem{
		Label: i18n.T("file-chooser-menu-select-all"), Accel: "Ctrl+A", OnClick: c.SelectAll,
		Disabled: len(c.activeView().Selection()) == len(c.s.entries),
	}
}

// SelectionItems are the selection commands as menu items, with their
// keys: Select All, Select None (disabled with nothing selected) and
// Invert Selection (disabled with nothing listed).
func (c *Browser) SelectionItems() []widget.MenuItem {
	return []widget.MenuItem{
		c.SelectAllItem(),
		{Label: i18n.T("file-chooser-menu-select-none"), Accel: "Shift+Ctrl+A", OnClick: c.SelectNone, Disabled: len(c.activeView().Selection()) == 0},
		{Label: i18n.T("file-chooser-menu-invert-selection"), Accel: "Shift+Ctrl+I", OnClick: c.InvertSelection, Disabled: len(c.s.entries) == 0},
	}
}
