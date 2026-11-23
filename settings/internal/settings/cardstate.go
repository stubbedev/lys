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

// stateFace is where a state's summary and glyph show: a card's row,
// whose title and media tile they are, or an inline line's.
type stateFace interface {
	showState(on bool)
	setSummary(kind cardStateKind, text string)
	setMedia(w widget.Widget)
}

// stateParts is what every state shows, whatever its face: the glyph (a
// spinner while loading), the error behind it in a dim monospace detail
// out of sight until the disclosure opens it, and the action a state
// may offer: a failure's retry, or what fills an empty card.
type stateParts struct {
	face    stateFace
	kind    cardStateKind
	glyph   *widget.Icon
	spinner *widget.Spinner
	detail  *widget.Label
	// reveal opens the detail, offered only while there is one.
	reveal *disclosure
	action *widget.Button
	// actionText is the action button's label.
	actionText *widget.Label
	// onAction is the shown state's action, nil while the action button
	// is hidden.
	onAction func()
}

// stateAction is a state's button: its label and what a click runs.
type stateAction struct {
	label string
	run   func()
}

func (k *kit) stateParts(face stateFace) *stateParts {
	s := &stateParts{face: face, glyph: k.icon(cardStateErrorIcon), spinner: widget.NewSpinner(cardStateIconPx), actionText: k.label("")}
	s.detail = wrapped(k.label("", "card-state-detail", css.Caption, css.DimLabel, css.Monospace))
	s.reveal = k.chevronDisclosure(false, s.detail.SetVisible)
	s.action = k.roleButton(roleSecondary, s.actionText, s.runAction)
	return s
}

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
	*stateParts
	// group is the item group lone made, shown only while the state is.
	group *widget.ItemGroup
}

// cardState is a state row showing nothing until it is switched.
func (k *kit) cardState() *cardState {
	s := &cardState{settingRow: k.row("", "")}
	s.stateParts = k.stateParts(s)
	s.AddClass("card-state")
	s.SetLines(0, rowDescriptionLines)
	s.AddDetail(s.detail)
	s.AddTrailing(s.reveal.button)
	s.withControl(s.action)
	s.Clear()
	return s
}

func (s *cardState) showState(on bool) {
	s.SetVisible(on)
	if s.group != nil {
		s.group.SetVisible(on)
	}
}

func (s *cardState) setSummary(_ cardStateKind, text string) { s.SetTitle(text) }

func (s *cardState) setMedia(w widget.Widget) { s.SetMedia(w, false) }

// inlineState is the same state outside a card's rows: a page's or a
// form's line saying why an action failed, a note in a preview, a
// row's status. The glyph sits beside a caption summary, red for a
// failure and dim otherwise, the disclosure and the action after it,
// and the detail under the line.
type inlineState struct {
	*widget.Box
	*stateParts
	summary *widget.Label
}

// inlineState is an inline state showing nothing until it is switched.
func (k *kit) inlineState() *inlineState {
	s := &inlineState{Box: gapBox(widget.Column), summary: wrapped(k.label("", "inline-state-summary", css.Caption))}
	s.stateParts = k.stateParts(s)
	s.AddClass("inline-state")
	line := gapBox(widget.Row)
	line.AppendAligned(s.glyph, false, widget.AlignCenter)
	line.AppendAligned(s.spinner, false, widget.AlignCenter)
	line.AppendAligned(s.summary, true, widget.AlignCenter)
	line.AppendAligned(s.reveal.button, false, widget.AlignCenter)
	line.AppendAligned(s.action, false, widget.AlignCenter)
	s.Append(line, false)
	s.Append(s.detail, false)
	s.stateParts.Clear()
	return s
}

// Clear hides the line.
func (s *inlineState) Clear() { s.stateParts.Clear() }

// Text is the summary the line shows.
func (s *inlineState) Text() string { return s.summary.Text() }

func (s *inlineState) showState(on bool) { s.SetVisible(on) }

func (s *inlineState) setSummary(kind cardStateKind, text string) {
	s.summary.SetText(text)
	s.summary.SetClass(css.Error, kind == stateError)
	s.summary.SetClass(css.DimLabel, kind != stateError)
}

func (s *inlineState) setMedia(w widget.Widget) {
	s.glyph.SetVisible(w == widget.Widget(s.glyph))
	s.spinner.SetVisible(w == widget.Widget(s.spinner))
}

// Clear hides the state: the card shows its content.
func (s *stateParts) Clear() { s.show(stateNone, "", "", nil, nil) }

// Empty says the list holds nothing, icon ("" for none) beside text.
func (s *stateParts) Empty(icon, text string) { s.show(stateEmpty, icon, text, nil, nil) }

// Offer says the content is not there yet, icon beside text, with the
// action labelled label, which starts it, run once per click.
func (s *stateParts) Offer(icon, text, label string, run func()) {
	s.show(stateEmpty, icon, text, nil, &stateAction{label: label, run: run})
}

// Loading says the content is being read, a spinner beside text.
func (s *stateParts) Loading(text string) { s.show(stateLoading, "", text, nil, nil) }

// Unavailable says a service cannot be reached: icon beside summary, a
// one-line localized sentence, and detail, the error why, behind the
// disclosure.
func (s *stateParts) Unavailable(icon, summary string, detail error) {
	s.show(stateUnavailable, icon, summary, detail, nil)
}

// Error says something failed: summary, and detail behind the
// disclosure, with retry, when not nil, run once per click of the
// state's retry button.
func (s *stateParts) Error(summary string, detail error, retry func()) {
	var act *stateAction
	if retry != nil {
		act = &stateAction{label: locale.T().Get("settings-card-retry"), run: retry}
	}
	s.show(stateError, cardStateErrorIcon, summary, detail, act)
}

// Result says how an action ended: nothing when err is nil, else the
// failure, summary over err behind the disclosure.
func (s *stateParts) Result(summary string, err error) {
	if err == nil {
		s.Clear()
		return
	}
	s.Error(summary, err, nil)
}

// errText is err's text, "" for nil.
func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (s *stateParts) show(kind cardStateKind, icon, summary string, detail error, act *stateAction) {
	s.kind = kind
	s.face.showState(kind != stateNone)
	s.face.setSummary(kind, summary)
	s.spinner.SetSpinning(kind == stateLoading)
	switch {
	case kind == stateLoading:
		s.face.setMedia(s.spinner)
	case icon != "":
		s.glyph.SetThemeName(icon)
		s.face.setMedia(s.glyph)
	default:
		s.face.setMedia(nil)
	}
	s.detail.SetText(errText(detail))
	s.reveal.button.SetVisible(detail != nil)
	s.reveal.setOpen(false)
	s.onAction = nil
	if act != nil {
		s.onAction = act.run
		s.actionText.SetText(act.label)
	}
	s.action.SetVisible(act != nil)
}

// lone is the state in an item group of its own, for a card whose other
// content is in no group with it: the group shows only while the state
// does, so a cleared state leaves no gap above the content.
func (s *cardState) lone() *widget.ItemGroup {
	s.group = widget.NewItemGroup(s)
	s.group.SetVisible(s.kind != stateNone)
	return s.group
}

// runAction runs the shown state's action; the hidden button runs none.
func (s *stateParts) runAction() {
	if s.onAction != nil {
		s.onAction()
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
	b.Append(b.state.lone(), false)
	b.Append(b.content, false)
	return b
}
