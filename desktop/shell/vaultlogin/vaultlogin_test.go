package vaultlogin

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/secrets/vault"
)

// Each failure reads as what the user can do about it; an unknown one
// keeps its own text.
func TestErrorText(t *testing.T) {
	for err, want := range map[error]string{
		vault.ErrWrongPassword:                           i18n.T("dropdown-bitwarden-error-wrong-password"),
		fmt.Errorf("unlock: %w", vault.ErrWrongPassword): i18n.T("dropdown-bitwarden-error-wrong-password"),
		vault.ErrInvalidCredentials:                      i18n.T("dropdown-bitwarden-error-credentials"),
		vault.ErrLoginRequired:                           i18n.T("dropdown-bitwarden-error-login-required"),
		&vault.ServerError{Status: 503}:                  i18n.T("dropdown-bitwarden-error-unavailable"),
		&vault.ServerError{Status: 400, Message: "no"}:   i18n.T("dropdown-bitwarden-error-server", i18n.Str("message", "no")),
		vault.ErrUnreachable:                             i18n.T("dropdown-bitwarden-error-network"),
		errors.New("odd"):                                "odd",
	} {
		if got := ErrorText(err); got != want {
			t.Errorf("%v = %q, want %q", err, got, want)
		}
	}
}

// A login asking for a code becomes the code step: the authenticator
// app when the account has one, else the email, or a new device's
// code; answering repeats the login with the code where it belongs.
// Any other outcome asks for nothing.
func TestALoginAskingForACodeIsAChallenge(t *testing.T) {
	req := vault.LoginRequest{Email: "ada@example.com", Password: "pw"}
	app, ok := ChallengeOf(req, &vault.TwoFactorRequiredError{Providers: []vault.TwoFactorProvider{vault.TwoFactorEmail, vault.TwoFactorAuthenticator}})
	if !ok || app.Provider != vault.TwoFactorAuthenticator || app.Emailed() || app.Description() != i18n.T("dropdown-bitwarden-two-factor-app") {
		t.Errorf("with an authenticator: %+v, %v", app, ok)
	}
	if got := app.Answer("123456"); got.TwoFactor == nil || got.TwoFactor.Code != "123456" || got.TwoFactor.Provider != vault.TwoFactorAuthenticator || got.Password != "pw" || got.NewDeviceOTP != "" {
		t.Errorf("answering the app code: %+v", got)
	}
	mail, ok := ChallengeOf(req, fmt.Errorf("login: %w", &vault.TwoFactorRequiredError{Providers: []vault.TwoFactorProvider{vault.TwoFactorEmail}}))
	if !ok || !mail.Emailed() || mail.Description() != i18n.T("dropdown-bitwarden-two-factor-email", i18n.Str("email", "ada@example.com")) {
		t.Errorf("with an emailed code: %+v, %v", mail, ok)
	}
	device, ok := ChallengeOf(req, vault.ErrNewDeviceVerification)
	if !ok || !device.NewDevice || device.Emailed() || device.Title() != i18n.T("dropdown-bitwarden-new-device-title") {
		t.Errorf("for a new device: %+v, %v", device, ok)
	}
	if got := device.Answer("9876"); got.NewDeviceOTP != "9876" || got.TwoFactor != nil {
		t.Errorf("answering the device code: %+v", got)
	}
	for _, err := range []error{nil, vault.ErrWrongPassword, errors.New("odd")} {
		if _, ok := ChallengeOf(req, err); ok || Asks(err) {
			t.Errorf("%v asks for a code", err)
		}
	}
	if !Asks(vault.ErrNewDeviceVerification) {
		t.Error("a new device's code is not an ask")
	}
}
