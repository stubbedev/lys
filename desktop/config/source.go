package config

import (
	"reflect"
	"strings"
	"sync"
)

// ValueSource is the layer a field's effective value comes from: the
// built-in default, the config file, or a runtime override
// (wayle-config's ValueSource) - what the settings GUI badges.
type ValueSource int

// The layers, lowest first.
const (
	SourceDefault ValueSource = iota
	SourceConfig
	SourceRuntime
)

func (v ValueSource) String() string {
	return [...]string{"default", "config", "runtime"}[v]
}

// canonicalPath resolves a dot path (aliases allowed) to its canonical
// keys; false for a path naming no field.
func canonicalPath(path string) (string, bool) {
	t := typeOf[Config]()
	var keys []string
	for seg := range strings.SplitSeq(path, ".") {
		_, f, ok := fieldAtPath(t, seg)
		if !ok {
			return "", false
		}
		keys = append(keys, f.key)
		t = f.typ
	}
	return strings.Join(keys, "."), len(keys) > 0
}

// Source is the layer the value at a dot path comes from. A container
// path takes the highest layer any leaf under it uses.
func (s *Service) Source(path string) ValueSource {
	canonical, ok := canonicalPath(path)
	if !ok {
		return SourceDefault
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case stagedAt(s.cur.staged, canonical):
		return SourceRuntime
	case stagedAt(s.cur.configSet, canonical):
		return SourceConfig
	}
	return SourceDefault
}

// ConfigValue is the value the config file sets at a dot path, false
// when it sets none (the property's config() slot).
func (s *Service) ConfigValue(path string) (any, bool) {
	canonical, ok := canonicalPath(path)
	if !ok {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return stagedValue(s.cur.configSet, canonical)
}

// RuntimeValue is the runtime override at a dot path, false for none.
func (s *Service) RuntimeValue(path string) (any, bool) {
	canonical, ok := canonicalPath(path)
	if !ok {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return stagedValue(s.cur.staged, canonical)
}

var defaultConfig = sync.OnceValue(Defaults)

// DefaultValue is the built-in default at a dot path, encoded as the
// config serializes it, a copy the caller owns.
func DefaultValue(path string) (any, error) { return encodedAt(defaultConfig(), path) }

func encodedAt(cfg *Config, path string) (any, error) {
	v := valueOf(cfg)
	var encoded any
	walked := true
	for seg := range strings.SplitSeq(path, ".") {
		if walked {
			child, found, structural := schemaChild(v, seg)
			if structural {
				if !found {
					return nil, &InvalidFieldError{Field: seg, Component: path, Reason: FieldNotFound}
				}
				v = child
				continue
			}
			encoded, walked = encodeValue(v), false
		}
		next, ok := lookup(encoded, seg)
		if !ok {
			return nil, &InvalidFieldError{Field: seg, Component: path, Reason: FieldNotFound}
		}
		encoded = next
	}
	if walked {
		encoded = encodeValue(v)
	}
	return widenFloats(encoded), nil
}

func schemaChild(v reflect.Value, key string) (child reflect.Value, found, structural bool) {
	if _, ok := marshalerOf(v); ok || enumVariants(v.Type()) != nil {
		return reflect.Value{}, false, false
	}
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return reflect.Value{}, false, true
		}
		return schemaChild(v.Elem(), key)
	case reflect.Struct:
		f, ok := fieldByKey(v.Type(), key)
		if !ok {
			return reflect.Value{}, false, true
		}
		child = v.FieldByIndex(f.index)
	case reflect.Map:
		child = v.MapIndex(reflect.ValueOf(key).Convert(v.Type().Key()))
		if !child.IsValid() {
			return reflect.Value{}, false, true
		}
	default:
		return reflect.Value{}, false, false
	}
	return child, !encodesAbsent(child), true
}

func encodesAbsent(v reflect.Value) bool {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return true
		}
		v = v.Elem()
	}
	return false
}

// stagedValue is the leaf staged at a canonical path, plain.
func stagedValue(root *table, path string) (any, bool) {
	v, ok := stagedLeaf(root, path)
	if !ok {
		return nil, false
	}
	return widenFloats(toPlain(v)), true
}

// Plain turns a GetByPath value into plain Go values: tables become
// map[string]any (their key order dropped), arrays []any, scalars as
// they are.
func Plain(v any) any { return toPlain(v) }

// Encode is the plain form a typed value stores as: what GetByPath
// reads for a field holding v, and what SetByPath takes back.
func Encode(v any) any { return toPlain(widenFloats(encode(v))) }
