package config

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/stubbedev/lys/lib/atomicfile"
)

// Service is the live configuration: the defaults, the config file
// layer, and the runtime override layer (runtime.yaml, written by
// `lys-shell config set` and the settings GUI), resolved into immutable
// *Config snapshots that subscribers receive on every effective change.
// Snapshots are never mutated; a reload publishes a new one.
type Service struct {
	dir         string
	runtimePath string
	sink        DiagnosticSink

	mu           sync.Mutex
	cur          layers
	subs         map[int]func(old, new *Config)
	nextSub      int
	session      sessionMotion
	motionWarned *bool

	secretsMu   sync.Mutex
	secretsSubs map[int]func()
	themeSubs   map[int]func()
	nextSignal  int

	watcher *watcher
}

type layers struct {
	cfg *Config
	// tree is the merged config document last loaded, nil for none.
	tree any
	// staged is the runtime leaves in effect, canonical paths.
	staged *table
	// configSet is the leaves the config file set, canonical paths.
	configSet *table
}

// Open loads the user's configuration from the platform directory and
// starts hot reload: config.yaml with its imports on the config layer,
// runtime.yaml on the runtime layer. A file-level failure logs and
// keeps the defaults for that layer; bad values are reported as
// diagnostics on stderr.
func Open() (*Service, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	s := Load(dir, StderrDiagnostics)
	if err := s.startWatcher(); err != nil {
		return s, err
	}
	return s, nil
}

// Load builds a service over dir without watching it (the CLI's
// one-shot load).
func Load(dir string, sink DiagnosticSink) *Service {
	runtimePath := RuntimeOverlayPath(filepath.Join(dir, "config.yaml"))
	s := &Service{
		dir:         dir,
		runtimePath: runtimePath,
		sink:        sink,
		subs:        map[int]func(old, new *Config){},
		secretsSubs: map[int]func(){},
		themeSubs:   map[int]func(){},
	}
	LoadEnvFiles(dir)
	tree, err := loadTree(s.MainPath())
	if err != nil {
		log.Printf("config: using defaults, %s failed:\n%v", filepath.Base(s.MainPath()), err)
		tree = nil
	}
	runtimeTree, err := readRuntime(s.RuntimePath())
	if err != nil {
		log.Printf("config: runtime.yaml failed:\n%v", err)
	}
	cur, rerr := build(tree, runtimeTree, nil, sink)
	if rerr != nil {
		log.Printf("config: invalid runtime.yaml value:\n%v", rerr)
	}
	s.cur = cur
	return s
}

// MainPath is the discovered main config file.
func (s *Service) MainPath() string { return DiscoverMain(s.dir) }

// RuntimePath is runtime.yaml in the config directory.
func (s *Service) RuntimePath() string { return s.runtimePath }

// Dir is the config directory the service reads.
func (s *Service) Dir() string { return s.dir }

// Config returns the current snapshot. Callers must not mutate it.
func (s *Service) Config() *Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cur.cfg
}

// Subscribe registers fn for every change of the effective config; fn
// runs on the reloading goroutine with the previous and new snapshots
// (UI callers hop to their loop). The returned func unsubscribes.
func (s *Service) Subscribe(fn func(old, new *Config)) (cancel func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.nextSub
	s.nextSub++
	s.subs[id] = fn
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.subs, id)
	}
}

// Watch calls fn with the selected value whenever it changes, for one
// field or section.
func Watch[T any](s *Service, selector func(*Config) T, fn func(T)) (cancel func()) {
	return s.Subscribe(func(old, new *Config) {
		next := selector(new)
		if reflect.DeepEqual(selector(old), next) {
			return
		}
		fn(next)
	})
}

// publish swaps in new layers and notifies the subscribers when the
// effective config changed. Only the session and the leaves the
// touched tables name can differ from the current snapshot.
func (s *Service) publish(next layers, touched ...*table) {
	cfg := next.cfg
	s.mu.Lock()
	cfg.Animations.session = s.session
	old := s.cur.cfg
	s.cur = next
	var subs []func(old, new *Config)
	if old.Animations.session != cfg.Animations.session || leavesDiffer(old, cfg, touched) {
		subs = slices.Collect(maps.Values(s.subs))
	}
	warn := s.shadowedMotionKey()
	s.mu.Unlock()
	if warn != nil {
		s.sink(*warn)
	}
	for _, fn := range subs {
		fn(old, cfg)
	}
}

func leavesDiffer(old, cfg *Config, touched []*table) bool {
	for _, t := range touched {
		if containerDiffers(valueOf(old), valueOf(cfg), t) {
			return true
		}
	}
	return false
}

func containerDiffers(old, cfg reflect.Value, staged *table) bool {
	for _, key := range staged.keys {
		f := mustFieldByKey(old.Type(), key)
		a, b := old.FieldByIndex(f.index), cfg.FieldByIndex(f.index)
		if !isContainer(f.typ) {
			if !reflect.DeepEqual(a.Interface(), b.Interface()) {
				return true
			}
			continue
		}
		if containerDiffers(a, b, stagedContainer(staged, key)) {
			return true
		}
	}
	return false
}

func stagedContainer(staged *table, key string) *table {
	child, ok := staged.vals[key].(*table)
	if !ok {
		panic(fmt.Sprintf("config: staged section %q holds %T, not a table", key, staged.vals[key]))
	}
	return child
}

func mustFieldByKey(t reflect.Type, key string) *fieldInfo {
	f, ok := fieldByKey(t, key)
	if !ok {
		panic(fmt.Sprintf("config: staged key %q names no field of %s", key, t))
	}
	return f
}

func withStaged(base, src *table, t reflect.Type) *table {
	out := &table{keys: slices.Clone(base.keys), vals: maps.Clone(base.vals)}
	for _, key := range src.keys {
		f := mustFieldByKey(t, key)
		v := src.vals[key]
		if isContainer(f.typ) {
			child, ok := base.vals[key].(*table)
			if !ok {
				child = newTable()
			}
			v = withStaged(child, stagedContainer(src, key), f.typ)
		}
		out.set(key, v)
	}
	return out
}

// reducedMotionKey is the key a compositor that owns reduced motion
// leaves without effect.
const reducedMotionKey = "animations.reduced-motion"

// shadowedMotionKey is the warning due when the compositor owns reduced
// motion and the key is set, once for each value it is set to. The lock
// is held.
func (s *Service) shadowedMotionKey() *Diagnostic {
	set := stagedAt(s.cur.staged, reducedMotionKey) || stagedAt(s.cur.configSet, reducedMotionKey)
	if !s.cur.cfg.Animations.CompositorOwnsMotion() || !set {
		s.motionWarned = nil
		return nil
	}
	value := s.cur.cfg.Animations.ReducedMotion
	if s.motionWarned != nil && *s.motionWarned == value {
		return nil
	}
	s.motionWarned = &value
	d := Diagnostic{Kind: DiagnosticWarning, Title: "config key has no effect under lys-wm"}.
		field("key", reducedMotionKey).
		field("value", strconv.FormatBool(value))
	d.Hint = "lys-wm owns this setting for the desktop: set animations.reduced_motion in its config, ~/.config/lys-wm/config.yaml"
	return &d
}

// SetSessionReducedMotion records what the compositor that owns reduced
// motion says. It holds across reloads and reaches subscribers as a
// change of the effective config.
func (s *Service) SetSessionReducedMotion(on bool) {
	say := sessionFull
	if on {
		say = sessionReduced
	}
	s.setSessionMotion(say)
}

// ClearSessionReducedMotion hands reduced motion back to the key: the
// compositor that owned it has gone.
func (s *Service) ClearSessionReducedMotion() {
	s.setSessionMotion(sessionSilent)
}

func (s *Service) setSessionMotion(say sessionMotion) {
	s.mu.Lock()
	if s.session == say {
		s.mu.Unlock()
		return
	}
	s.session = say
	next := s.cur
	cfg := *next.cfg
	next.cfg = &cfg
	s.mu.Unlock()
	s.publish(next)
}

// build applies the config tree then the runtime tree onto the
// defaults. warn, when set, holds the runtime overrides active before
// a reload: config leaves they shadow get the "change ignored"
// warning.
func build(configTree, runtimeTree any, warn *table, sink DiagnosticSink) (layers, error) {
	migrated, runtimeTree := migrate(configTree), migrate(runtimeTree)
	cfg := Defaults()
	ca := &layerApply{kind: configLayer, sink: sink, staged: newTable()}
	if warn != nil {
		ca.overridden = func(path string) bool { return stagedAt(warn, path) }
	}
	if migrated != nil {
		_ = ca.applyContainer(valueOf(cfg), migrated, "")
	}
	ra := &layerApply{kind: runtimeLayer, sink: sink, staged: newTable()}
	var err error
	if runtimeTree != nil {
		err = ra.applyContainer(valueOf(cfg), runtimeTree, "")
	}
	return layers{cfg: cfg, tree: configTree, staged: ra.staged, configSet: ca.staged}, err
}

// readRuntime parses runtime.yaml. A missing one is no overrides (the
// file exists only once something wrote one), unless a runtime.toml
// from before the YAML switch is beside it: that is an error naming
// the file to convert, so the old overrides are not dropped without a
// word.
func readRuntime(path string) (any, error) {
	data, err := os.ReadFile(path) //nolint:gosec // runtime.yaml in the config dir
	if errors.Is(err, fs.ErrNotExist) {
		return nil, legacyTOMLError(filepath.Join(filepath.Dir(path), "runtime.toml"), path)
	}
	if err != nil {
		return nil, &LoadError{msg: "cannot read file", err: err}
	}
	tree, err := parseYAML(data)
	if err != nil {
		return nil, &LoadError{msg: fmt.Sprintf("%s: %v", path, err), err: err}
	}
	return tree, nil
}

// Value is the effective config as it serializes, in the parsed shape:
// map[string]any tables (canonical keys), []any arrays, string, bool,
// int64 and float64 leaves.
func (s *Service) Value() any {
	return toPlain(encode(s.Config()))
}

// GetByPath returns the effective value at a dot path, as the config
// serializes (canonical keys; f32 values widened to f64).
func (s *Service) GetByPath(path string) (any, error) { return encodedAt(s.Config(), path) }

// SetByPath sets a runtime override at a dot path. A value the field
// rejects is a *RuntimeValueError; a path, or a table under it, that
// names no config field is an error naming the path, and applies nothing.
func (s *Service) SetByPath(path string, value any) error {
	root := map[string]any{}
	if err := InsertPath(root, path, value); err != nil {
		return err
	}
	s.mu.Lock()
	next := s.cur
	s.mu.Unlock()
	cfg := *next.cfg
	touched := newTable()
	ra := &layerApply{kind: runtimeLayer, sink: s.sink, staged: touched}
	if err := ra.applyContainer(valueOf(&cfg), root, ""); err != nil {
		return err
	}
	if touched.len() == 0 {
		return fmt.Errorf("config: %s names no config field", path)
	}
	next.cfg, next.staged = &cfg, withStaged(next.staged, touched, typeOf[Config]())
	s.publish(next, touched)
	return nil
}

// ResetByPath drops the runtime override at a dot path; cleared reports
// whether one existed. The path must name one leaf.
func (s *Service) ResetByPath(path string) (cleared bool, err error) {
	canonical, err := clearRuntimePath(typeOf[Config](), path)
	if err != nil {
		return false, err
	}
	s.mu.Lock()
	next := s.cur
	s.mu.Unlock()
	cleared = stagedAt(next.staged, canonical)
	cfg := *next.cfg
	if cleared {
		base, err := layeredBelowRuntime(next.configSet, canonical)
		if err != nil {
			return false, err
		}
		fieldAt(valueOf(&cfg), canonical).Set(base)
	}
	staged := cloneTable(next.staged)
	deleteStaged(staged, strings.Split(canonical, "."))
	next.cfg, next.staged = &cfg, schemaOrdered(staged, typeOf[Config]())
	touched := newTable()
	setStaged(touched, canonical, nil)
	s.publish(next, touched)
	return cleared, nil
}

func layeredBelowRuntime(configSet *table, canonical string) (reflect.Value, error) {
	defaults := fieldAt(valueOf(defaultConfig()), canonical)
	encoded, ok := stagedLeaf(configSet, canonical)
	if !ok {
		return defaults, nil
	}
	v, err := decodeLeaf(defaults.Type(), toPlain(encoded))
	if err != nil {
		return reflect.Value{}, fmt.Errorf("config: decode the config file's %s again: %w", canonical, err)
	}
	return v, nil
}

func fieldAt(v reflect.Value, canonical string) reflect.Value {
	for seg := range strings.SplitSeq(canonical, ".") {
		v = v.FieldByIndex(mustFieldByKey(v.Type(), seg).index)
	}
	return v
}

func schemaOrdered(staged *table, t reflect.Type) *table {
	out := newTable()
	for _, f := range fieldsOf(t) {
		v, ok := staged.get(f.key)
		if !ok {
			continue
		}
		if isContainer(f.typ) {
			v = schemaOrdered(stagedContainer(staged, f.key), f.typ)
		}
		out.set(f.key, v)
	}
	return out
}

// ResetAllRuntime drops every runtime override and removes
// runtime.yaml; a missing file is not an error.
func (s *Service) ResetAllRuntime() error {
	s.mu.Lock()
	tree, previous := s.cur.tree, s.cur.staged
	s.mu.Unlock()
	next, _ := build(tree, nil, nil, DiscardDiagnostics)
	s.publish(next, previous)
	err := os.Remove(s.RuntimePath())
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// RuntimeYAML renders the runtime overrides as runtime.yaml content.
func (s *Service) RuntimeYAML() (string, error) {
	out, err := s.runtimeDocument()
	return string(out), err
}

func (s *Service) runtimeDocument() ([]byte, error) {
	s.mu.Lock()
	staged := s.cur.staged
	s.mu.Unlock()
	return yamlDocumentBytes(widenFloats(staged))
}

// Save persists the runtime overrides to runtime.yaml atomically
// (atomicfile).
func (s *Service) Save() error {
	content, err := s.runtimeDocument()
	if err != nil {
		return &LoadError{msg: "cannot render the runtime overrides", err: err}
	}
	if err := atomicfile.WriteFile(s.runtimePath, content, 0o644); err != nil {
		return &LoadError{msg: fmt.Sprintf("cannot persist config to '%s'", s.runtimePath), err: err}
	}
	return nil
}

// reloadMain re-reads the main config and runtime.yaml after a file
// change. A broken main file leaves everything as it was; an
// unreadable runtime.yaml drops the runtime layer.
func (s *Service) reloadMain() error {
	tree, err := loadTree(s.MainPath())
	if err != nil {
		return err
	}
	s.mu.Lock()
	previous := s.cur
	s.mu.Unlock()
	runtimeTree, _ := readRuntime(s.RuntimePath())
	next, _ := build(tree, runtimeTree, previous.staged, s.sink)
	s.publish(next, previous.staged, next.staged, previous.configSet, next.configSet)
	return nil
}

// reloadRuntime re-reads only runtime.yaml; an unreadable file keeps
// the current overrides.
func (s *Service) reloadRuntime() {
	runtimeTree, err := readRuntime(s.RuntimePath())
	if err != nil {
		return
	}
	s.mu.Lock()
	previous := s.cur
	s.mu.Unlock()
	next, _ := build(previous.tree, runtimeTree, nil, s.sink)
	s.publish(next, previous.staged, next.staged)
}

// SubscribeSecrets registers fn for every reload of the .env files.
func (s *Service) SubscribeSecrets(fn func()) (cancel func()) {
	return s.subscribeSignal(s.secretsSubs, fn)
}

// SubscribeThemes registers fn for every change under themes/.
func (s *Service) SubscribeThemes(fn func()) (cancel func()) {
	return s.subscribeSignal(s.themeSubs, fn)
}

func (s *Service) subscribeSignal(subs map[int]func(), fn func()) func() {
	s.secretsMu.Lock()
	defer s.secretsMu.Unlock()
	id := s.nextSignal
	s.nextSignal++
	subs[id] = fn
	return func() {
		s.secretsMu.Lock()
		defer s.secretsMu.Unlock()
		delete(subs, id)
	}
}

func (s *Service) signal(subs map[int]func()) {
	s.secretsMu.Lock()
	fns := make([]func(), 0, len(subs))
	for _, fn := range subs {
		fns = append(fns, fn)
	}
	s.secretsMu.Unlock()
	for _, fn := range fns {
		fn()
	}
}

// Close stops hot reload.
func (s *Service) Close() {
	if s.watcher != nil {
		s.watcher.close()
	}
}
