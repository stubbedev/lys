package settings

import (
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// cardStateKind is which state a cardState shows.
type cardStateKind uint8

// The states.
const (
	// stateNone shows nothing: the card has its content.
	stateNone cardStateKind = iota
	// stateEmpty is a list holding nothing.
	stateEmpty
	// stateLoading is content still being read.
	stateLoading
	// stateUnavailable is a service the card cannot reach.
	stateUnavailable
	// stateError is a failure, which a retry may get past.
	stateError
)

// cardStateIconPx is the size of a state's glyph on its media tile.
const cardStateIconPx = 16

// cardStateErrorIcon is the glyph of a failure.
const cardStateErrorIcon = "ld-alert-triangle-symbolic"

// cardState is the one row a card shows in place of content it lacks: a
// list holding nothing, content still loading, a service it cannot
// reach, a failure. A list or panel holds one and switches it, so no
// page adds or removes such labels by hand. It is a setting row, inset
// like the rows around it: the glyph (a spinner while loading) on the
// media tile beside a localized summary. The error behind a failure is
// never the summary: it waits in a dim monospace detail under it, shown
// once the disclosure at the row's end opens. A failure that can be
// retried carries the retry as the row's control.
type cardState struct {
	*settingRow
	kind    cardStateKind
	glyph   *widget.Icon
	spinner *widget.Spinner
	detail  *widget.Label
	// reveal opens the detail, offered only while there is one.
	reveal *disclosure
	retry  *widget.Button
	// onRetry is the shown failure's retry, nil while the retry button
	// is hidden.
	onRetry func()
}

// cardState is a state row showing nothing until it is switched.
func (k *kit) cardState() *cardState {
	s := &cardState{settingRow: k.row("", ""), glyph: k.icon(cardStateErrorIcon), spinner: widget.NewSpinner(cardStateIconPx)}
	s.AddClass("card-state")
	s.SetLines(0, rowDescriptionLines)
	s.detail = wrapped(k.label("", "card-state-detail", css.Caption, css.DimLabel, css.Monospace))
	s.AddDetail(s.detail)
	s.reveal = k.chevronDisclosure(false, s.detail.SetVisible)
	s.AddTrailing(s.reveal.button)
	s.retry = k.secondaryButton(locale.T().Get("settings-card-retry"), s.runRetry)
	s.withControl(s.retry)
	s.Clear()
	return s
}

// Clear hides the row: the card shows its content.
func (s *cardState) Clear() { s.show(stateNone, "", "", "", nil) }

// Empty says the list holds nothing, icon beside text.
func (s *cardState) Empty(icon, text string) { s.show(stateEmpty, icon, text, "", nil) }

// Loading says the content is being read, a spinner beside text.
func (s *cardState) Loading(text string) { s.show(stateLoading, "", text, "", nil) }

// Unavailable says a service cannot be reached: icon beside summary, a
// one-line localized sentence, and detail, the error why, behind the
// disclosure.
func (s *cardState) Unavailable(icon, summary string, detail error) {
	s.show(stateUnavailable, icon, summary, errText(detail), nil)
}

// Error says something failed: summary, and detail behind the
// disclosure, with retry, when not nil, run once per click of the row's
// retry button.
func (s *cardState) Error(summary string, detail error, retry func()) {
	s.show(stateError, cardStateErrorIcon, summary, errText(detail), retry)
}

// errText is err's text, "" for nil.
func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (s *cardState) show(kind cardStateKind, icon, summary, detail string, retry func()) {
	s.kind = kind
	s.SetVisible(kind != stateNone)
	s.SetTitle(summary)
	s.spinner.SetSpinning(kind == stateLoading)
	switch {
	case kind == stateLoading:
		s.SetMedia(s.spinner, false)
	case icon != "":
		s.glyph.SetThemeName(icon)
		s.SetMedia(s.glyph, false)
	default:
		s.SetMedia(nil, false)
	}
	s.detail.SetText(detail)
	s.reveal.button.SetVisible(detail != "")
	s.reveal.setOpen(false)
	s.onRetry = retry
	s.retry.SetVisible(retry != nil)
}

// runRetry runs the shown failure's retry; the hidden button runs none.
func (s *cardState) runRetry() {
	if s.onRetry != nil {
		s.onRetry()
	}
}

// stateBox is the body of a card whose content a service gives: the
// content, and the cardState shown in its place while there is none.
type stateBox struct {
	*widget.Box
	state   *cardState
	content *widget.Box
}

func (k *kit) stateBox() *stateBox {
	b := &stateBox{Box: gapBox(widget.Column), state: k.cardState(), content: gapBox(widget.Column)}
	b.Append(widget.NewItemGroup(b.state), false)
	b.Append(b.content, false)
	return b
}
