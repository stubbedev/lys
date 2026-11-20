// Package eaptext is the enterprise (802.1X) sign-in's wording in the
// desktop's language: lys-shell's messages for what a sign-in as typed
// gets wrong, which every form joining an enterprise network says the
// same way.
package eaptext

import (
	"errors"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/network"
)

// Problem is network.NewEAPSettings' refusal as what to fix; one
// without words of its own keeps its text.
func Problem(err error) string {
	switch {
	case errors.Is(err, network.ErrEAPIdentityEmpty):
		return i18n.T("dropdown-network-enterprise-error-identity")
	case errors.Is(err, network.ErrEAPPasswordEmpty):
		return i18n.T("dropdown-network-enterprise-error-password")
	case errors.Is(err, network.ErrEAPServerUnchecked):
		return i18n.T("dropdown-network-enterprise-error-server")
	case errors.Is(err, network.ErrEAPDomainInvalid):
		return i18n.T("dropdown-network-enterprise-error-domain")
	case errors.Is(err, network.ErrEAPPathNotAbsolute), errors.Is(err, network.ErrEAPPathInvalid):
		return i18n.T("dropdown-network-enterprise-error-path")
	case errors.Is(err, network.ErrEAPClientCertMissing):
		return i18n.T("dropdown-network-enterprise-error-client-cert")
	case errors.Is(err, network.ErrEAPPrivateKeyMissing):
		return i18n.T("dropdown-network-enterprise-error-private-key")
	}
	return err.Error()
}
