package usershelper

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/useraccount"

	"github.com/stubbedev/lys/settings/internal/accounts"
)

func resource(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "resources", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestThePolicyHasOneActionPerClassAndNoProgramToRun(t *testing.T) {
	var policy struct {
		Actions []struct {
			ID       string `xml:"id,attr"`
			Annotate []struct {
				Key string `xml:"key,attr"`
			} `xml:"annotate"`
		} `xml:"action"`
	}
	if err := xml.Unmarshal(resource(t, "dev.stubbe.lys.settings.users.policy"), &policy); err != nil {
		t.Fatal(err)
	}
	if len(policy.Actions) != len(accounts.Classes) {
		t.Fatalf("%d actions, want one per class", len(policy.Actions))
	}
	for i, c := range accounts.Classes {
		if a := policy.Actions[i]; a.ID != c.Action() || len(a.Annotate) != 0 {
			t.Errorf("class %s: action %q annotated %v", c, a.ID, a.Annotate)
		}
	}
}

func TestTheBusLetsRootOwnTheServiceAndAnyoneCallOnlyChange(t *testing.T) {
	type rule struct {
		Own         string `xml:"own,attr"`
		Destination string `xml:"send_destination,attr"`
		Interface   string `xml:"send_interface,attr"`
		Member      string `xml:"send_member,attr"`
	}
	var conf struct {
		Policies []struct {
			User    string `xml:"user,attr"`
			Context string `xml:"context,attr"`
			Allow   []rule `xml:"allow"`
		} `xml:"policy"`
	}
	if err := xml.Unmarshal(resource(t, accounts.BusName+".conf"), &conf); err != nil {
		t.Fatal(err)
	}
	var owners []string
	var calls []rule
	for _, p := range conf.Policies {
		for _, a := range p.Allow {
			if a.Own != "" {
				owners = append(owners, p.User+" "+a.Own)
			}
			if a.Destination != "" && p.Context == "default" {
				calls = append(calls, a)
			}
		}
	}
	if !slices.Equal(owners, []string{"root " + accounts.BusName}) {
		t.Errorf("owners %q", owners)
	}
	want := []rule{
		{Destination: accounts.BusName, Interface: accounts.BusInterface, Member: accounts.ChangeMethod},
		{Destination: accounts.BusName, Interface: "org.freedesktop.DBus.Introspectable"},
	}
	if !slices.Equal(calls, want) {
		t.Errorf("calls %+v, want %+v", calls, want)
	}
}

func TestTheBusActivatesTheServiceThroughItsUnit(t *testing.T) {
	activation := string(resource(t, accounts.BusName+".service"))
	unit := string(resource(t, "lys-users.service"))
	for _, line := range []string{"Name=" + accounts.BusName, "User=root", "SystemdService=lys-users.service", "Exec=/usr/libexec/lys-users"} {
		if !slices.Contains(strings.Split(activation, "\n"), line) {
			t.Errorf("the activation file lacks %s", line)
		}
	}
	for _, line := range []string{"Type=dbus", "BusName=" + accounts.BusName, "ExecStart=/usr/libexec/lys-users", "ProtectSystem=strict", "ReadWritePaths=/etc /home -/var/spool/mail -/var/mail", "StateDirectory=" + strings.TrimPrefix(useraccount.AvatarsDir, "/var/lib/")} {
		if !slices.Contains(strings.Split(unit, "\n"), line) {
			t.Errorf("the unit lacks %s", line)
		}
	}
}
