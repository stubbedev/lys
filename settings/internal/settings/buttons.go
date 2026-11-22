package settings

import (
	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// buttonRole is what a button does, which alone decides how it looks:
// a caller names the role and buttonRoles gives its classes, so one
// action looks the same on every page.
type buttonRole uint8

// The roles. The text roles carry a label, the icon roles an icon whose
// tooltip is the only text naming what they do.
const (
	// rolePrimary is the affirmative action of a form or row.
	rolePrimary buttonRole = iota
	// roleDestructive loses data: it deletes, forgets or logs out.
	roleDestructive
	// roleSecondary is any other action.
	roleSecondary
	// roleCancel abandons an inline form, looking like a dialog's cancel.
	roleCancel
	// roleLink goes to another page or reveals more.
	roleLink
	// roleQuiet is a text action with no fill until hovered: a footer
	// action, a toggle between two views of one value, a result row.
	roleQuiet
	// rolePicker opens a picker for a value it shows.
	rolePicker
	// roleTile is a card that opens what it shows.
	roleTile
	// rolePlain is a picture that is its own button, an avatar or a
	// speaker, with no fill of its own.
	rolePlain
	// roleAdd adds an item to a list.
	roleAdd
	// roleRemove removes an item, in the destructive color.
	roleRemove
	// roleReset puts a value back to its default.
	roleReset
	// roleClear empties a value.
	roleClear
	// roleReload reads a list again.
	roleReload
	// roleIcon is any other icon action.
	roleIcon
	// rolePickerIcon opens a picker, an icon alone naming it.
	rolePickerIcon
)

// buttonLook is a role's classes and, for an icon role with one glyph,
// that glyph.
type buttonLook struct {
	classes []string
	icon    string
}

// buttonRoles is every role's look, the one place a button's classes
// are chosen.
var buttonRoles = [...]buttonLook{
	rolePrimary:     {classes: []string{css.SuggestedAction}},
	roleDestructive: {classes: []string{css.DestructiveAction}},
	roleSecondary:   {classes: []string{css.Secondary}},
	roleCancel:      {classes: []string{css.Outline}},
	roleLink:        {classes: []string{css.Link}},
	roleQuiet:       {classes: []string{css.Ghost}},
	rolePicker:      {classes: []string{css.Outline}},
	roleTile:        {classes: []string{css.Card}},
	rolePlain:       {},
	roleAdd:         {classes: []string{css.Outline, css.Icon}, icon: "ld-plus-symbolic"},
	roleRemove:      {classes: []string{css.Ghost, css.DestructiveAction, css.Icon}, icon: "ld-trash-2-symbolic"},
	roleReset:       {classes: []string{css.Ghost, css.Icon}, icon: "ld-rotate-ccw-symbolic"},
	roleClear:       {classes: []string{css.Ghost, css.Icon}, icon: "ld-x-circle-symbolic"},
	roleReload:      {classes: []string{css.Ghost, css.Icon}, icon: "ld-refresh-cw-symbolic"},
	roleIcon:        {classes: []string{css.Ghost, css.Icon}},
	rolePickerIcon:  {classes: []string{css.Outline, css.Icon}},
}

// classes are the classes the role's button wears.
func (r buttonRole) classes() []string { return buttonRoles[r].classes }

// button wraps child in a button carrying classes, the one constructor
// every role goes through.
func (k *kit) button(child widget.Widget, onClick func(), classes ...string) *widget.Button {
	b := widget.NewButton(child, 0, 0)
	b.AddClass(classes...)
	b.OnClick = onClick
	return b
}

// roleButton is a button of role around child, for a child that is not
// a plain label or icon: a label whose text changes, a tile's face.
func (k *kit) roleButton(role buttonRole, child widget.Widget, onClick func()) *widget.Button {
	return k.button(child, onClick, role.classes()...)
}

func (k *kit) textButton(role buttonRole, text string, onClick func()) *widget.Button {
	return k.roleButton(role, k.label(text), onClick)
}

// primaryButton is the affirmative action reading text.
func (k *kit) primaryButton(text string, onClick func()) *widget.Button {
	return k.textButton(rolePrimary, text, onClick)
}

// destructiveButton is an action reading text that loses data.
func (k *kit) destructiveButton(text string, onClick func()) *widget.Button {
	return k.textButton(roleDestructive, text, onClick)
}

// secondaryButton is any other action reading text.
func (k *kit) secondaryButton(text string, onClick func()) *widget.Button {
	return k.textButton(roleSecondary, text, onClick)
}

// cancelButton abandons an inline form.
func (k *kit) cancelButton(onClick func()) *widget.Button {
	return k.textButton(roleCancel, locale.T().Get("settings-cancel"), onClick)
}

// linkButton goes where text says.
func (k *kit) linkButton(text string, onClick func()) *widget.Button {
	return k.textButton(roleLink, text, onClick)
}

// contentButton is a button of role wearing lys-ui's icon-and-label
// content, which it returns too so both can change in place.
func (k *kit) contentButton(role buttonRole, icon, text string, onClick func()) (*widget.Button, *widget.ButtonContent) {
	content := widget.NewButtonContent(k.face, k.textPx(), icon, text)
	return k.roleButton(role, content, onClick), content
}

// iconRoleButton is an icon-only button of role: its tooltip, the
// tooltipKey message, is the only text naming what it does, so it
// cannot be left out.
func (k *kit) iconRoleButton(role buttonRole, icon, tooltipKey string, onClick func()) *widget.Button {
	b := k.roleButton(role, k.icon(icon), onClick)
	b.SetTooltip(locale.T().Get(tooltipKey))
	return b
}

func (k *kit) glyphButton(role buttonRole, tooltipKey string, onClick func()) *widget.Button {
	return k.iconRoleButton(role, buttonRoles[role].icon, tooltipKey, onClick)
}

// iconButton is an icon action no other role covers.
func (k *kit) iconButton(icon, tooltipKey string, onClick func()) *widget.Button {
	return k.iconRoleButton(roleIcon, icon, tooltipKey, onClick)
}

// pickerIconButton opens a picker, icon naming it.
func (k *kit) pickerIconButton(icon, tooltipKey string, onClick func()) *widget.Button {
	return k.iconRoleButton(rolePickerIcon, icon, tooltipKey, onClick)
}

// addButton adds an item to a list.
func (k *kit) addButton(tooltipKey string, onClick func()) *widget.Button {
	return k.glyphButton(roleAdd, tooltipKey, onClick)
}

// removeButton removes an item: a list's entry, a file, a grant.
func (k *kit) removeButton(tooltipKey string, onClick func()) *widget.Button {
	return k.glyphButton(roleRemove, tooltipKey, onClick)
}

// clearButton empties a value.
func (k *kit) clearButton(tooltipKey string, onClick func()) *widget.Button {
	return k.glyphButton(roleClear, tooltipKey, onClick)
}

// reloadButton reads a list again.
func (k *kit) reloadButton(tooltipKey string, onClick func()) *widget.Button {
	return k.glyphButton(roleReload, tooltipKey, onClick)
}

// moveButtons move an item one place up and one place down its list.
func (k *kit) moveButtons(move func(by int)) (up, down *widget.Button) {
	return k.iconButton("ld-chevron-up-symbolic", "settings-list-move-up", func() { move(-1) }),
		k.iconButton("ld-chevron-down-symbolic", "settings-list-move-down", func() { move(1) })
}

// hiddenPriority puts a hidden reset button's opacity above every
// stylesheet, so no CSS overrides it.
const hiddenPriority = apptheme.Priority + 1

// resetButton puts a value back to its default. Hidden, it keeps its
// slot, invisible and inert, so the control beside it never moves.
type resetButton struct {
	*widget.Button
}

// resetButton is a shown reset button.
func (k *kit) resetButton(tooltipKey string, onClick func()) resetButton {
	return resetButton{k.glyphButton(roleReset, tooltipKey, onClick)}
}

// show shows the button, or hides it in place.
func (r resetButton) show(on bool) {
	r.SetEnabled(on)
	if on {
		r.SetInlineStyle("")
		return
	}
	r.SetInlineStylePriority("opacity: 0;", hiddenPriority)
}

// browseButton is the picker asking the file chooser, pick getting the
// chosen path.
func (k *kit) browseButton(pick func(path string)) *widget.Button {
	return k.chooserButton("settings-browse-file", func(p pickers) { p.openFile(pick) })
}

func (k *kit) folderButton(pick func(path string)) *widget.Button {
	return k.chooserButton("settings-browse-folder", func(p pickers) { p.openFolder(pick) })
}

func (k *kit) chooserButton(tooltipKey string, open func(pickers)) *widget.Button {
	return k.pickerIconButton("ld-folder-open-symbolic", tooltipKey, func() {
		if k.pickers != nil {
			open(k.pickers)
		}
	})
}
