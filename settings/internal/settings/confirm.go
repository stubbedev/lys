package settings

import (
	"github.com/stubbedev/lys-ui/app"
	"github.com/stubbedev/lys-ui/widget/css"

	"github.com/stubbedev/lys-settings/internal/locale"
)

// confirmDialogWidth is the confirm's width, lys-ui's MessageBox's.
const confirmDialogWidth = 440

// Responses of the reset-all confirm.
const (
	responseConfirm = "confirm"
	responseCancel  = "cancel"
)

// confirmDialog is lys-ui's alert asking the question messages under key
// name: key-title over key-description, key-cancel and the destructive
// key-confirm. It is modal, Esc cancels, Enter does nothing, and only the
// confirm response runs onConfirm.
func confirmDialog(k *kit, key string, onConfirm func()) app.DialogConfig {
	t := locale.T()
	body := k.label(t.Get(key+"-description"), css.DimLabel)
	body.SetWrap(true)
	return app.DialogConfig{
		Title:   t.Get(key + "-title"),
		Width:   confirmDialogWidth,
		Content: body,
		Buttons: []app.DialogButton{
			{Label: t.Get(key + "-cancel"), Response: responseCancel, Role: app.ButtonRoleCancel, Appearance: app.AppearancePlain},
			{Label: t.Get(key + "-confirm"), Response: responseConfirm, Appearance: app.AppearanceDestructive},
		},
		Modal:          true,
		CancelResponse: responseCancel,
		OnResponse: func(r string) {
			if r == responseConfirm {
				onConfirm()
			}
		},
	}
}

// resetAllDialog is the reset-all question (ConfirmResetAll), dropping
// the overrides on confirm.
func resetAllDialog(k *kit) app.DialogConfig {
	return confirmDialog(k, "settings-reset-all", k.resetAll)
}
