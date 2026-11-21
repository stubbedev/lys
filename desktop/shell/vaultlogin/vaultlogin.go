// Package vaultlogin is signing in to lys-secrets' Bitwarden vault as
// every surface offering it says it, lys-shell's Bitwarden dropdown and
// lys-settings' Bitwarden page alike: a failure in words, and the code
// step a login can stop at.
package vaultlogin

import (
	"errors"

	"github.com/stubbedev/lys/secrets/vault"

	"github.com/stubbedev/lys/desktop/i18n"
)

// ErrorText is an operation's failure as what the user can do about it;
// a failure without words of its own keeps its text.
func ErrorText(err error) string {
	switch {
	case errors.Is(err, vault.ErrWrongPassword):
		return i18n.T("dropdown-bitwarden-error-wrong-password")
	case errors.Is(err, vault.ErrInvalidCredentials):
		return i18n.T("dropdown-bitwarden-error-credentials")
	case errors.Is(err, vault.ErrLoginRequired):
		return i18n.T("dropdown-bitwarden-error-login-required")
	case errors.Is(err, vault.ErrUnreachable):
		return i18n.T("dropdown-bitwarden-error-network")
	}
	if se, ok := errors.AsType[*vault.ServerError](err); ok {
		if se.Retryable() {
			return i18n.T("dropdown-bitwarden-error-unavailable")
		}
		return i18n.T("dropdown-bitwarden-error-server", i18n.Str("message", se.Message))
	}
	return err.Error()
}

// Challenge is a login waiting on a code: the attempt to repeat, and
// whether the server asked for a new device's code or which second
// factor.
type Challenge struct {
	Request   vault.LoginRequest
	NewDevice bool
	Provider  vault.TwoFactorProvider
}

// ChallengeOf is the code step a login of req failing with err asks
// for; false when err asks for none.
func ChallengeOf(req vault.LoginRequest, err error) (Challenge, bool) {
	if tf, ok := errors.AsType[*vault.TwoFactorRequiredError](err); ok {
		return Challenge{Request: req, Provider: provider(tf.Providers)}, true
	}
	if errors.Is(err, vault.ErrNewDeviceVerification) {
		return Challenge{Request: req, NewDevice: true}, true
	}
	return Challenge{}, false
}

// Asks reports whether err is a login asking for a code rather than
// failing.
func Asks(err error) bool {
	_, ok := ChallengeOf(vault.LoginRequest{}, err)
	return ok
}

// provider picks the second factor to ask for: the authenticator app
// when the account has one, else the emailed code.
func provider(ps []vault.TwoFactorProvider) vault.TwoFactorProvider {
	for _, p := range ps {
		if p == vault.TwoFactorAuthenticator {
			return p
		}
	}
	return vault.TwoFactorEmail
}

// Title heads the code step.
func (c Challenge) Title() string {
	if c.NewDevice {
		return i18n.T("dropdown-bitwarden-new-device-title")
	}
	return i18n.T("dropdown-bitwarden-two-factor-title")
}

// Description says where the code comes from.
func (c Challenge) Description() string {
	switch {
	case c.NewDevice:
		return i18n.T("dropdown-bitwarden-new-device-desc", i18n.Str("email", c.Request.Email))
	case c.Provider == vault.TwoFactorEmail:
		return i18n.T("dropdown-bitwarden-two-factor-email", i18n.Str("email", c.Request.Email))
	}
	return i18n.T("dropdown-bitwarden-two-factor-app")
}

// Emailed reports whether the code comes by an email the user may ask
// to be sent again.
func (c Challenge) Emailed() bool { return !c.NewDevice && c.Provider == vault.TwoFactorEmail }

// Answer is the login repeated with code.
func (c Challenge) Answer(code string) vault.LoginRequest {
	req := c.Request
	if c.NewDevice {
		req.NewDeviceOTP = code
		return req
	}
	req.TwoFactor = &vault.TwoFactor{Provider: c.Provider, Code: code}
	return req
}
