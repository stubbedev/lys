package eaptext

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/network"
)

func TestEachRefusalSaysWhatToFix(t *testing.T) {
	for err, key := range map[error]string{
		network.ErrEAPIdentityEmpty:                              "dropdown-network-enterprise-error-identity",
		network.ErrEAPPasswordEmpty:                              "dropdown-network-enterprise-error-password",
		network.ErrEAPServerUnchecked:                            "dropdown-network-enterprise-error-server",
		fmt.Errorf("%w: %q", network.ErrEAPDomainInvalid, "a b"): "dropdown-network-enterprise-error-domain",
		network.ErrEAPPathNotAbsolute:                            "dropdown-network-enterprise-error-path",
		network.ErrEAPPathInvalid:                                "dropdown-network-enterprise-error-path",
		network.ErrEAPClientCertMissing:                          "dropdown-network-enterprise-error-client-cert",
		network.ErrEAPPrivateKeyMissing:                          "dropdown-network-enterprise-error-private-key",
	} {
		if got, want := Problem(err), i18n.T(key); got != want {
			t.Errorf("%v reads %q, want %q", err, got, want)
		}
	}
	if got := Problem(errors.New("odd")); got != "odd" {
		t.Errorf("an unknown refusal reads %q, want its own text", got)
	}
}
