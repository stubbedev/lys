package i18ntest

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/stubbedev/lys/desktop/i18n"
)

type recorder struct {
	testing.TB
	failures []string
}

func (r *recorder) Helper() {}

func (r *recorder) Error(args ...any) { r.failures = append(r.failures, fmt.Sprint(args...)) }

func (r *recorder) Errorf(format string, args ...any) {
	r.failures = append(r.failures, fmt.Sprintf(format, args...))
}

func (r *recorder) Fatalf(format string, args ...any) {
	r.Errorf(format, args...)
	r.FailNow()
}

func (r *recorder) FailNow() { panic(r) }

func check(assets i18n.Assets, probe string, sentences ...string) []string {
	r := &recorder{}
	var wg sync.WaitGroup
	wg.Go(func() {
		defer func() {
			if p := recover(); p != nil && p != r {
				panic(p)
			}
		}()
		CheckAssets(r, assets, probe, sentences...)
	})
	wg.Wait()
	return r.failures
}

func catalogues(src map[string]string) i18n.Assets {
	fsys := fstest.MapFS{}
	for _, lang := range i18n.Shipped() {
		text, ok := src[lang.String()]
		if !ok {
			text = src[i18n.Fallback.String()]
		}
		fsys[lang.String()+"/_app.ftl"] = &fstest.MapFile{Data: []byte(text)}
	}
	for lang, text := range src {
		fsys[lang+"/_app.ftl"] = &fstest.MapFile{Data: []byte(text)}
	}
	return i18n.NewAssets(fsys)
}

const english = "app-title = Files\napp-why = Suspends only: { $reason }\n"

func TestCompleteCataloguesPass(t *testing.T) {
	t.Parallel()
	if failures := check(catalogues(map[string]string{"en-US": english}), "app-title", "app-why"); len(failures) != 0 {
		t.Errorf("complete catalogues fail: %q", failures)
	}
}

func TestEachShortfallFails(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		src       map[string]string
		probe     string
		sentences []string
		want      string
	}{
		{"an extra language", map[string]string{"en-US": english, "sv": english}, "app-title", []string{"app-why"}, "the catalogues are"},
		{"a missing message", map[string]string{"en-US": english, "de": "app-why = Nur: { $reason }\n"}, "app-why", []string{"app-why"}, "app-title"},
		{"a missing probe", map[string]string{"en-US": english}, "app-gone", []string{"app-why"}, "no app-gone"},
		{"an error's own text", map[string]string{"en-US": english}, "app-title", nil, "app-why takes an error's text"},
		{"a stale sentence", map[string]string{"en-US": english}, "app-title", []string{"app-why", "app-title"}, "app-title is listed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			failures := check(catalogues(tc.src), tc.probe, tc.sentences...)
			if !strings.Contains(strings.Join(failures, "\n"), tc.want) {
				t.Errorf("failures %q, want one saying %q", failures, tc.want)
			}
		})
	}
}
