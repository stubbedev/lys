package settings

import (
	"crypto/rand"
	"fmt"
	"io"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const generatorSampleChars = 32

type generatorRules interface {
	Generate(random io.Reader) (string, error)
}

// presetRules reads a generator preset card into the rules lys-shell's
// generator draws from: a key the card leaves unset is the preset's
// default, as the shell reads it.
type presetRules func(field func(key string) slot) (generatorRules, error)

func unmarshalSet(field func(key string) slot, key string, into interface{ UnmarshalConfig(any) error }) error {
	v := field(key).get()
	if v == nil {
		return nil
	}
	if err := into.UnmarshalConfig(v); err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	return nil
}

func passwordRules(field func(key string) slot) (generatorRules, error) {
	p := config.DefaultsBitwardenPasswordPreset()
	if err := unmarshalSet(field, "length", &p.Length); err != nil {
		return nil, err
	}
	if err := unmarshalSet(field, "charsets", &p.Charsets); err != nil {
		return nil, err
	}
	if v, ok := field("avoid-ambiguous").get().(bool); ok {
		p.AvoidAmbiguous = v
	}
	return p.Rules(), nil
}

func passphraseRules(field func(key string) slot) (generatorRules, error) {
	p := config.DefaultsBitwardenPassphrasePreset()
	if err := unmarshalSet(field, "words", &p.Words); err != nil {
		return nil, err
	}
	if v, ok := field("separator").get().(string); ok {
		p.Separator = v
	}
	if v, ok := field("capitalize").get().(bool); ok {
		p.Capitalize = v
	}
	if v, ok := field("number").get().(bool); ok {
		p.Number = v
	}
	return p.Rules(), nil
}

func generatorSampleField(rules presetRules) cardField {
	return cardField{label: "settings-generator-sample", fill: true, item: func(k *kit, field func(string) slot) control {
		return newGeneratorSample(k, field, rules)
	}}
}

// generatorSample is a preset's sample: a secret drawn from its rules
// as they stand, drawn again on demand or when they change, which can
// be copied.
type generatorSample struct {
	*widget.Box
	k       *kit
	field   func(key string) slot
	rules   presetRules
	text    *widget.Label
	again   *widget.Button
	copy    *widget.Button
	problem *inlineState
	sample  string
}

func newGeneratorSample(k *kit, field func(key string) slot, rules presetRules) *generatorSample {
	g := &generatorSample{Box: gapBox(widget.Column), k: k, field: field, rules: rules, problem: k.inlineState()}
	g.AddClass("generator-sample")
	g.text = k.label("", "generator-sample-text", css.Monospace)
	g.text.SetEllipsize(widget.EllipsizeMiddle)
	g.text.SetMaxWidthChars(generatorSampleChars)
	g.again = k.reloadButton("settings-generator-sample-again", g.refresh)
	g.copy = k.copyButton("settings-generator-sample-copy", g.copySample)
	line := gapBox(widget.Row)
	line.AppendAligned(g.text, true, widget.AlignCenter)
	line.AppendAligned(g.again, false, widget.AlignCenter)
	line.AppendAligned(g.copy, false, widget.AlignCenter)
	g.Append(line, false)
	g.Append(g.problem, false)
	g.refresh()
	return g
}

func (g *generatorSample) refresh() {
	g.sample = ""
	rules, err := g.rules(g.field)
	if err == nil {
		g.sample, err = rules.Generate(rand.Reader)
	}
	g.copy.SetEnabled(err == nil)
	g.text.SetText(g.sample)
	g.text.SetTooltip(g.sample)
	g.problem.Result(locale.T().Get("settings-generator-sample-failed"), err)
}

func (g *generatorSample) copySample() {
	g.problem.Result(locale.T().Get("settings-generator-sample-copy-failed"), g.k.copySecret(g.sample))
}
