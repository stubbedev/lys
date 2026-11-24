package usershelper

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"

	"github.com/stubbedev/lys/settings/internal/accounts"
)

type policyAction struct {
	ID       string `xml:"id,attr"`
	Annotate []struct {
		Key   string `xml:"key,attr"`
		Value string `xml:",chardata"`
	} `xml:"annotate"`
}

func TestThePolicyAuthorisesEachClassOfTheHelperByItsArgument(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "resources", "dev.stubbe.lys.settings.users.policy"))
	if err != nil {
		t.Fatal(err)
	}
	var policy struct {
		Actions []policyAction `xml:"action"`
	}
	if err := xml.Unmarshal(data, &policy); err != nil {
		t.Fatal(err)
	}
	if len(policy.Actions) != len(accounts.Classes) {
		t.Fatalf("%d actions, want one per class", len(policy.Actions))
	}
	for i, c := range accounts.Classes {
		a := policy.Actions[i]
		got := map[string]string{}
		for _, an := range a.Annotate {
			got[an.Key] = an.Value
		}
		if a.ID != c.Action() || got["org.freedesktop.policykit.exec.argv1"] != string(c) || got["org.freedesktop.policykit.exec.path"] != "/usr/libexec/"+accounts.HelperName {
			t.Errorf("class %s: action %q annotated %v", c, a.ID, got)
		}
	}
}
