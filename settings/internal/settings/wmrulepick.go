package settings

import (
	"errors"
	"regexp"

	"github.com/stubbedev/lys/lib/wmipc"

	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmstate"
)

const (
	rulesKey = "rules"
	titleKey = "title"
)

var errUnnamedWindow = errors.New("settings: the picked window has neither an app ID nor a title")

func exactPattern(text string) string { return "^" + regexp.QuoteMeta(text) + "$" }

func ruleFromWindow(w wmipc.Window) (map[string]any, error) {
	switch {
	case w.AppID != "":
		return map[string]any{appIDKey: exactPattern(w.AppID)}, nil
	case w.Title != "":
		return map[string]any{titleKey: exactPattern(w.Title)}, nil
	}
	return nil, errUnnamedWindow
}

func rulePick(w *wmBackend) *cardPick {
	run := w.run
	return &cardPick{
		key: "wm-rules-pick", waiting: "wm-rules-picking",
		item: func() (map[string]any, error) {
			win, err := wmstate.PickWindow(run)
			if err != nil {
				return nil, err
			}
			return ruleFromWindow(win)
		},
		refusal: rulePickRefusal,
	}
}

func rulePickRefusal(err error) string {
	t := locale.T()
	switch {
	case errors.Is(err, wmstate.ErrNotRunning):
		return t.Get("wm-rules-pick-not-running")
	case errors.Is(err, wmstate.ErrNoWindow):
		return t.Get("wm-rules-pick-none")
	case errors.Is(err, wmstate.ErrWindowGone):
		return t.Get("wm-rules-pick-gone")
	case errors.Is(err, errUnnamedWindow):
		return t.Get("wm-rules-pick-unnamed")
	}
	return t.Get("wm-rules-pick-failed")
}
