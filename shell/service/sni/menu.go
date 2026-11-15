package sni

import (
	"context"
	"fmt"
	"time"

	"github.com/stubbedev/lys/lib/intconv"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

// MenuIface is the DBusMenu interface tray items publish their menus
// on.
const MenuIface = "com.canonical.dbusmenu"

// ToggleType is a menu row's toggle kind.
type ToggleType uint8

// Toggle types.
const (
	ToggleNone ToggleType = iota
	ToggleCheckmark
	ToggleRadio
)

// parseToggleType maps the toggle-type property; unknown is none.
func parseToggleType(s string) ToggleType {
	switch s {
	case "checkmark":
		return ToggleCheckmark
	case "radio":
		return ToggleRadio
	}
	return ToggleNone
}

// ToggleState is a toggle row's state; ToggleUnknown is the spec's -1.
type ToggleState int8

// Toggle states.
const (
	ToggleUnchecked ToggleState = 0
	ToggleChecked   ToggleState = 1
	ToggleUnknown   ToggleState = -1
)

// parseToggleState maps the toggle-state property.
func parseToggleState(v int32) ToggleState {
	switch v {
	case 0:
		return ToggleUnchecked
	case 1:
		return ToggleChecked
	}
	return ToggleUnknown
}

// MenuItem is one DBusMenu node with its children. The root is the invisible container whose children are
// the menu's rows.
type MenuItem struct {
	ID          int32
	Label       string
	Enabled     bool
	Visible     bool
	Separator   bool
	Toggle      ToggleType
	ToggleState ToggleState
	IconName    string
	// IconData is PNG data (the icon-data property).
	IconData []byte
	// Shortcut is the spec's shortcut list: each entry is modifiers
	// then the key ("Control", "Shift", "q").
	Shortcut [][]string
	Children []MenuItem
}

// parseMenuNode converts one (ia{sv}av) layout node, as godbus decodes
// a struct inside a variant: []any{int32, map[string]Variant,
// []Variant}. Malformed children are skipped.
func parseMenuNode(id int32, props map[string]dbus.Variant, children []dbus.Variant) MenuItem {
	item := MenuItem{ID: id, Enabled: true, Visible: true, ToggleState: ToggleUnchecked}
	if v, ok := props["label"]; ok {
		item.Label, _ = v.Value().(string)
	}
	if v, ok := props["enabled"]; ok {
		if b, ok := v.Value().(bool); ok {
			item.Enabled = b
		}
	}
	if v, ok := props["visible"]; ok {
		if b, ok := v.Value().(bool); ok {
			item.Visible = b
		}
	}
	if v, ok := props["type"]; ok {
		s, _ := v.Value().(string)
		item.Separator = s == "separator"
	}
	if v, ok := props["toggle-type"]; ok {
		s, _ := v.Value().(string)
		item.Toggle = parseToggleType(s)
	}
	if v, ok := props["toggle-state"]; ok {
		if n, ok := v.Value().(int32); ok {
			item.ToggleState = parseToggleState(n)
		}
	}
	if v, ok := props["icon-name"]; ok {
		item.IconName, _ = v.Value().(string)
	}
	if v, ok := props["icon-data"]; ok {
		item.IconData, _ = v.Value().([]byte)
	}
	if v, ok := props["shortcut"]; ok {
		item.Shortcut, _ = v.Value().([][]string)
	}
	for _, child := range children {
		fields, ok := child.Value().([]any)
		if !ok || len(fields) != 3 {
			continue
		}
		cid, ok1 := fields[0].(int32)
		cprops, ok2 := fields[1].(map[string]dbus.Variant)
		ckids, ok3 := fields[2].([]dbus.Variant)
		if !ok1 || !ok2 || !ok3 {
			continue
		}
		item.Children = append(item.Children, parseMenuNode(cid, cprops, ckids))
	}
	return item
}

// menuObject is the item's DBusMenu object; an item without a menu
// has none.
func (a *Actions) menuObject(it Item) (dbus.BusObject, error) {
	if it.MenuPath == "" {
		return nil, fmt.Errorf("sni: %s has no menu", it.Key())
	}
	return a.conn.Object(it.Bus, dbus.ObjectPath(it.MenuPath)), nil
}

// MenuOpening tells the app its menu is about to show (AboutToShow on
// the root, the spec's once per showing) without waiting for the answer,
// so an app slow to answer never holds the menu back: an app that
// changes the layout in answer announces it with LayoutUpdated, which
// reaches the open menu's change feed.
func (a *Actions) MenuOpening(ctx context.Context, it Item) error {
	obj, err := a.menuObject(it)
	if err != nil {
		return err
	}
	if err := obj.CallWithContext(ctx, MenuIface+".AboutToShow", dbus.FlagNoReplyExpected, int32(0)).Err; err != nil {
		return fmt.Errorf("sni: %s AboutToShow 0: %w", it.Key(), err)
	}
	return nil
}

// AboutToShow tells the app the (sub)menu id is about to show, the
// spec's once per showing; needUpdate is the app asking for its layout
// to be read again before it shows.
func (a *Actions) AboutToShow(ctx context.Context, it Item, id int32) (needUpdate bool, err error) {
	obj, err := a.menuObject(it)
	if err != nil {
		return false, err
	}
	if err := obj.CallWithContext(ctx, MenuIface+".AboutToShow", 0, id).Store(&needUpdate); err != nil {
		return false, fmt.Errorf("sni: %s AboutToShow %d: %w", it.Key(), id, err)
	}
	return needUpdate, nil
}

// MenuLayout reads the full layout (GetLayout) without AboutToShow:
// the re-read after a LayoutUpdated or ItemsPropertiesUpdated, which an
// app may itself send in answer to AboutToShow.
func (a *Actions) MenuLayout(ctx context.Context, it Item) (MenuItem, error) {
	obj, err := a.menuObject(it)
	if err != nil {
		return MenuItem{}, err
	}
	var revision uint32
	var layout struct {
		ID       int32
		Props    map[string]dbus.Variant
		Children []dbus.Variant
	}
	if err := obj.CallWithContext(ctx, MenuIface+".GetLayout", 0, int32(0), int32(-1), []string{}).Store(&revision, &layout); err != nil {
		return MenuItem{}, fmt.Errorf("sni: %s GetLayout: %w", it.Key(), err)
	}
	return parseMenuNode(layout.ID, layout.Props, layout.Children), nil
}

// MenuClicked sends the clicked event for a row (Event "clicked", data
// 0, the Unix time).
func (a *Actions) MenuClicked(ctx context.Context, it Item, id int32) error {
	obj, err := a.menuObject(it)
	if err != nil {
		return err
	}
	ts := intconv.Clamp[uint32](time.Now().Unix())
	return obj.CallWithContext(ctx, MenuIface+".Event", dbus.FlagNoReplyExpected, id, "clicked", dbus.MakeVariant(int32(0)), ts).Err
}
