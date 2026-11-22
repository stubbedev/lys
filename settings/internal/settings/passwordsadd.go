package settings

import (
	"context"
	"errors"
	"strings"

	"github.com/stubbedev/lys/desktop/i18n"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	genericSchema = "org.freedesktop.Secret.Generic"
	usernameAttr  = "username"
	serviceAttr   = "service"
	itemNoName    = formProblem("settings-passwords-add-no-name")
	itemNoSecret  = formProblem("settings-passwords-add-no-secret")
)

type formProblem string

func (p formProblem) Error() string { return locale.T().Get(string(p)) }

type newKeyringItem struct {
	label  string
	attrs  map[string]string
	secret []byte
}

func parseNewItem(label, username, service string, secret []byte) (newKeyringItem, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		return newKeyringItem{}, itemNoName
	}
	if len(secret) == 0 {
		return newKeyringItem{}, itemNoSecret
	}
	attrs := map[string]string{schemaAttribute: genericSchema}
	for key, value := range map[string]string{usernameAttr: username, serviceAttr: service} {
		if value = strings.TrimSpace(value); value != "" {
			attrs[key] = value
		}
	}
	return newKeyringItem{label: label, attrs: attrs, secret: secret}, nil
}

type keyringItemForm struct {
	*widget.Box
	label, username, service, secret *widget.Entry
	messages                         formNote
	busy                             bool
}

func (p *passwordsPanel) newItemForm() *keyringItemForm {
	t := locale.T()
	f := &keyringItemForm{Box: gapBox(widget.Column)}
	text := func(key string) *widget.Entry {
		e := p.k.entry()
		e.SetAccessibleName(t.Get(key))
		f.Append(p.k.row(t.Get(key), "").withControl(e), false)
		return e
	}
	f.label = text("settings-passwords-label")
	f.username = text("settings-passwords-add-username")
	f.service = text("settings-passwords-add-service")
	f.secret = p.k.passwordEntry("settings-passwords-secret")
	f.Append(p.k.row(t.Get("settings-passwords-secret"), "").withControl(f.secret).withTrailing(p.k.revealButton(f.secret)), false)
	f.messages = p.k.dialogMessages()
	f.Append(f.messages, false)
	return f
}

func (f *keyringItemForm) clear() {
	for _, e := range []*widget.Entry{f.label, f.username, f.service, f.secret} {
		e.SetText("")
	}
}

func (p *passwordsPanel) openAddItem() {
	c, ok := p.current()
	if !ok {
		return
	}
	if p.k.pickers == nil {
		p.fail(errors.New("settings: add a keyring item: no window to ask in"))
		return
	}
	t := locale.T()
	f := p.newItemForm()
	var closeDialog func()
	closeDialog = p.k.pickers.dialog(app.DialogConfig{
		Title:   t.Get("settings-passwords-add-title", i18n.Str("collection", collectionTitle(c))),
		Width:   passwordDialogWidth,
		Content: f,
		Focus:   f.label,
		Buttons: []app.DialogButton{
			{Label: t.Get("settings-passwords-add-cancel"), Response: responseCancel, Role: app.ButtonRoleCancel, Appearance: app.AppearancePlain},
			{Label: t.Get("settings-passwords-add-confirm"), Response: responseConfirm, Role: app.ButtonRoleDefault, Appearance: app.AppearanceSuggested},
		},
		Modal:           true,
		CancelResponse:  responseCancel,
		DefaultResponse: responseConfirm,
		ValidateResponse: func(r string) bool {
			if r != responseConfirm {
				return true
			}
			p.addItem(f, c.Path, func() { closeDialog() })
			return false
		},
		OnClosed: f.clear,
	})
}

func (p *passwordsPanel) addItem(f *keyringItemForm, coll dbus.ObjectPath, done func()) {
	if f.busy || p.c == nil {
		return
	}
	item, err := parseNewItem(f.label.Text(), f.username.Text(), f.service.Text(), []byte(f.secret.Text()))
	if err != nil {
		f.messages.say(err.Error())
		return
	}
	f.busy = true
	c := p.c
	async(p.k, func() error {
		defer clear(item.secret)
		ctx, cancel := context.WithTimeout(context.Background(), sysCallTimeout)
		defer cancel()
		return c.AddItem(ctx, coll, item.label, item.attrs, item.secret)
	}, func(err error) {
		f.busy = false
		if err != nil {
			f.messages.say(err.Error())
			return
		}
		f.messages.say("")
		f.clear()
		p.reload()
		done()
	})
}
