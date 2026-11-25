package search

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"

	"github.com/stubbedev/lys/shell/service/launcher/answer"
	"github.com/stubbedev/lys/shell/service/launcher/modes"
)

// Calculator answers arithmetic, unit and currency conversions, times
// across zones and date arithmetic inline, in its loader's language.
type Calculator struct {
	l        *i18n.Loader
	rates    *answer.Rates
	currency answer.CurrencyParser
	zones    *answer.Zones
	now      func() time.Time
}

// NewCalculator answers in l's language and formats, currencies over
// rates (nil answers none), reading "kr" and symbols with currency,
// placing times through zones, on now's calendar.
func NewCalculator(l *i18n.Loader, rates *answer.Rates, currency answer.CurrencyParser, zones *answer.Zones, now func() time.Time) *Calculator {
	return &Calculator{l: l, rates: rates, currency: currency, zones: zones, now: now}
}

// Spec implements Provider.
func (*Calculator) Spec() Spec { return Spec{Section: SectionAnswer} }

// Load implements Provider: it reads or fetches the currency rates when
// due, and has the list answered again once they changed. Answer never
// waits for them.
func (c *Calculator) Load(ctx context.Context, publish func([]Entry)) {
	if c.rates != nil && c.rates.Refresh(ctx) {
		publish(nil)
	}
}

// looksCalculable is what makes a query look like something to calculate: a
// lone number is not a question, and neither is a word that happens to
// parse as a constant ("e", "pi").
func looksCalculable(q string) bool {
	if strings.IndexFunc(q, func(r rune) bool { return r >= '0' && r <= '9' }) < 0 {
		return strings.Contains(q, "(") || strings.HasPrefix(q, "pi ") || strings.HasPrefix(q, "e ")
	}
	return strings.ContainsAny(q, "+-*/%^(") || strings.ContainsFunc(q, func(r rune) bool {
		return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
	})
}

// Answer implements Answerer.
func (c *Calculator) Answer(query string) []Entry {
	q := strings.TrimSpace(query)
	if e, ok := c.answerCurrency(q); ok {
		return []Entry{e}
	}
	if tq, ok := answer.ParseTime(q); ok {
		if t, ok := c.zones.Eval(tq, c.now()); ok {
			return []Entry{c.timeEntry(q, t)}
		}
	}
	if d, ok := answer.EvalDate(q, c.now()); ok {
		return []Entry{c.dateEntry(q, d)}
	}
	if !looksCalculable(q) {
		return nil
	}
	if conv, ok := modes.Convert(q); ok {
		return []Entry{calcEntry(q, modes.FormatResult(conv.Value), conv.Show(c.l), q)}
	}
	v, ok := modes.Eval(q)
	if !ok {
		return nil
	}
	return []Entry{calcEntry(q, modes.FormatResult(v), modes.ShowResult(c.l, v), q)}
}

// calcEntry is an answer row showing shown, which continues with value,
// the plain digits, clock or ISO date a query reads back.
func calcEntry(q, value, shown, subtitle string) Entry {
	return Entry{ID: "calc", Title: shown, Subtitle: subtitle, Icon: iconCalc, Data: calcData{value: value, shown: shown, expr: q}}
}

func (c *Calculator) answerCurrency(q string) (Entry, bool) {
	if c.rates == nil {
		return Entry{}, false
	}
	cq, ok := c.currency.Parse(q)
	if !ok {
		return Entry{}, false
	}
	snap, ok := c.rates.Snapshot()
	if !ok {
		return Entry{}, false
	}
	v, ok := snap.Table.Convert(cq.Amount, cq.From, cq.To)
	if !ok {
		return Entry{}, false
	}
	value, shown := answer.FormatAmount(v), c.l.FixedDecimal(v, answer.AmountDecimals(v))
	now := c.now()
	source := c.l.Get("launcher-answer-currency-source",
		i18n.Str("amount", cq.Amount), i18n.Str("from", cq.From.String()), i18n.Str("to", cq.To.String()),
		i18n.Str("date", c.l.Date(snap.Table.Date)))
	if !snap.Fresh(now) {
		source = c.l.Get("launcher-answer-currency-stale", i18n.Str("source", source), i18n.Int("days", int64(snap.AgeDays(now))))
	}
	return calcEntry(q, value, shown+" "+cq.To.String(), source), true
}

// day is t's weekday and the locale's date, in t's location.
func (c *Calculator) day(t time.Time) string {
	return c.l.Get("cal-day-"+strings.ToLower(t.Weekday().String())) + " " + c.l.Date(t)
}

// timeEntry shows the time in the locale's hour cycle with its zone's
// abbreviation; the value a row copies and continues with stays the
// 24-hour clock a query reads back.
func (c *Calculator) timeEntry(q string, t answer.TimeAnswer) Entry {
	clock := t.At.Format("15:04")
	shown := c.l.Time(t.At) + " " + t.At.Format("MST")
	if t.DayOffset != 0 {
		sign, days := "+", t.DayOffset
		if days < 0 {
			sign, days = "−", -days
		}
		shown = c.l.Get("launcher-answer-day-offset", i18n.Str("time", shown), i18n.Str("sign", sign), i18n.Int("days", int64(days)))
	}
	day := c.day(t.At)
	subtitle := day
	if t.Place != "" {
		subtitle = c.l.Get("launcher-answer-time-place", i18n.Str("day", day), i18n.Str("place", t.Place))
	}
	return calcEntry(q, clock, shown, subtitle)
}

// dateEntry shows a day in the locale's words; the value a row copies
// and continues with stays the ISO date a query reads back.
func (c *Calculator) dateEntry(q string, d answer.DateAnswer) Entry {
	if d.Kind == answer.DateIsSpan {
		days := strconv.Itoa(d.Days)
		return calcEntry(q, days, c.l.Get("launcher-answer-days", i18n.Int("days", int64(d.Days))), q)
	}
	return calcEntry(q, d.Day.Format(time.DateOnly), c.day(d.Day), q)
}

type calcData struct{ value, shown, expr string }

// Actions implements Provider.
func (c *Calculator) Actions(e Entry) []Action {
	d, ok := e.Data.(calcData)
	if !ok {
		return nil
	}
	actions := []Action{copyAction(c.l.Get("launcher-action-copy-result"), d.shown)}
	if d.value != d.shown {
		actions = append(actions, copyAction(c.l.Get("launcher-action-copy-number"), d.value))
	}
	return append(actions,
		copyAction(c.l.Get("launcher-action-copy-equation"), d.expr+" = "+d.shown),
		Action{Title: c.l.Get("launcher-action-continue"), Icon: "ld-corner-down-left-symbolic", Run: func(context.Context) Outcome {
			return SetQuery{Text: d.value}
		}},
	)
}

// Web is the fallback at the bottom of every list: search the web for
// what was typed. The template's {query} takes the escaped text.
type Web struct {
	env      Env
	name     string
	template string
}

// NewWeb searches with name's template.
func NewWeb(env Env, name, template string) *Web {
	return &Web{env: env, name: name, template: template}
}

// Spec implements Provider.
func (*Web) Spec() Spec { return Spec{Section: SectionWeb} }

// Load implements Provider.
func (*Web) Load(context.Context, func([]Entry)) {}

// Answer implements Answerer.
func (w *Web) Answer(query string) []Entry {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil
	}
	target, ok := config.FillQueryURL(w.template, q)
	if !ok {
		return nil
	}
	return []Entry{{
		ID: "web", Title: i18n.T("launcher-web-search", i18n.Str("engine", w.name), i18n.Str("query", q)),
		Icon: iconWeb, Data: target,
	}}
}

// Actions implements Provider.
func (w *Web) Actions(e Entry) []Action {
	target, ok := e.Data.(string)
	if !ok {
		return nil
	}
	return []Action{
		{Title: i18n.T("launcher-action-open"), Icon: "ld-external-link-symbolic", Run: func(context.Context) Outcome {
			return w.env.open(i18n.T("launcher-action-open"), target)
		}},
		copyAction(i18n.T("launcher-action-copy-url"), target),
	}
}

// Shell answers a query led by ">": run the rest as a command.
type Shell struct{ env Env }

// NewShell builds it.
func NewShell(env Env) *Shell { return &Shell{env: env} }

// Spec implements Provider.
func (*Shell) Spec() Spec { return Spec{Section: SectionAnswer} }

// Load implements Provider.
func (*Shell) Load(context.Context, func([]Entry)) {}

// Answer implements Answerer.
func (*Shell) Answer(query string) []Entry {
	cmd, ok := strings.CutPrefix(strings.TrimSpace(query), ">")
	cmd = strings.TrimSpace(cmd)
	if !ok || cmd == "" {
		return nil
	}
	return []Entry{{ID: "shell", Title: cmd, Subtitle: i18n.T("launcher-shell-subtitle"), Icon: iconTerminal, Data: cmd}}
}

// Actions implements Provider.
func (s *Shell) Actions(e Entry) []Action {
	cmd, ok := e.Data.(string)
	if !ok {
		return nil
	}
	return []Action{
		{Title: i18n.T("launcher-action-run"), Icon: "ld-play-symbolic", Run: func(context.Context) Outcome {
			s.env.SpawnShell(cmd)
			return Close{}
		}},
		{Title: i18n.T("launcher-action-run-in-terminal"), Icon: "ld-terminal-symbolic", Run: func(context.Context) Outcome {
			return s.env.inTerminal(cmd, "sh", "-c", cmd)
		}},
		copyAction(i18n.T("launcher-action-copy-command"), cmd),
	}
}
