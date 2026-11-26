package passwd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/userdb"
	"github.com/stubbedev/lys/lib/userdb/userdbtest"

	"github.com/stubbedev/lys/polkit/internal/passwd"
)

const ldapUID = 70001

func TestNameResolvesAUserOutsideEtcPasswd(t *testing.T) {
	passwd.UseSocket(t, userdbtest.Serve(t, userdbtest.DB{Users: []userdb.User{{Name: "carol", UID: ldapUID, GID: ldapUID}}}))
	name, err := passwd.Name(t.Context(), ldapUID)
	if err != nil || name != "carol" {
		t.Fatalf("Name = %q, %v; want carol", name, err)
	}
}

func TestNameOfAnUnknownUserFails(t *testing.T) {
	passwd.UseSocket(t, userdbtest.Serve(t, userdbtest.DB{}))
	if name, err := passwd.Name(t.Context(), ldapUID); !errors.Is(err, userdb.NoRecordFound) || !strings.Contains(err.Error(), "uid=70001") {
		t.Fatalf("Name = %q, %v; want NoRecordFound for uid 70001", name, err)
	}
}

func TestNameWithoutTheUserDatabaseFails(t *testing.T) {
	path := userdbtest.Missing(t)
	passwd.UseSocket(t, path)
	if name, err := passwd.Name(t.Context(), ldapUID); err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("Name = %q, %v; want an error naming %s", name, err, path)
	}
}
