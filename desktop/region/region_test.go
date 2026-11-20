package region

import (
	"errors"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/glibclocale"
	"github.com/stubbedev/lys/lib/glibclocale/glibclocaletest"
)

func envOf(vars map[string]string) func(string) string {
	return func(name string) string { return vars[name] }
}

func TestTheRegionIsReadFromTheLocaleOfEachCategory(t *testing.T) {
	src := glibclocale.NewSource(glibclocaletest.Archive(t))
	for name, c := range map[string]struct {
		env  map[string]string
		want Prefs
	}{
		"nothing set":          {map[string]string{}, Prefs{}},
		"american":             {map[string]string{"LANG": "en_US.UTF-8"}, Prefs{Imperial: true, TwelveHour: true}},
		"german":               {map[string]string{"LANG": "de_DE.UTF-8"}, Prefs{WeekStart: time.Monday}},
		"german units":         {map[string]string{"LANG": "en_US.UTF-8", "LC_MEASUREMENT": "de_DE.UTF-8"}, Prefs{TwelveHour: true}},
		"british clock":        {map[string]string{"LANG": "en_US.UTF-8", "LC_TIME": "en_GB.utf8"}, Prefs{Imperial: true, WeekStart: time.Monday}},
		"LC_ALL over the rest": {map[string]string{"LANG": "en_US.UTF-8", "LC_TIME": "en_US.UTF-8", "LC_ALL": "de_DE.UTF-8"}, Prefs{WeekStart: time.Monday}},
	} {
		got, err := Read(src, envOf(c.env))
		if err != nil || got != c.want {
			t.Errorf("%s: %+v (%v), want %+v", name, got, err, c.want)
		}
	}
}

func TestARegionWhoseLocaleIsNotInstalledIsAnError(t *testing.T) {
	src := glibclocale.NewSource(glibclocaletest.Archive(t))
	if _, err := Read(src, envOf(map[string]string{"LANG": "xx_YY.UTF-8"})); !errors.Is(err, glibclocale.ErrNoLocale) {
		t.Errorf("an uninstalled locale reads %v, want ErrNoLocale", err)
	}
}
