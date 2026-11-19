package wmstate

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/settings/internal/wmconfig"
	"github.com/stubbedev/lys/settings/internal/wmtest"
)

func answering(answers map[string]string) wmconfig.Runner {
	return func(_ context.Context, argv ...string) ([]byte, error) {
		if len(argv) != 2 || argv[0] != wmconfig.Control {
			return nil, errors.New("unexpected program " + strings.Join(argv, " "))
		}
		out, ok := answers[argv[1]]
		if !ok {
			return nil, errors.New("unexpected query " + argv[1])
		}
		return []byte(out), nil
	}
}

func notRunning(_ context.Context, argv ...string) ([]byte, error) {
	return nil, &wmconfig.ToolError{Argv: argv, Exit: 3, Stderr: "lys-wmctl: ipc: no lys-wm socket", Err: errors.New("exit status 3")}
}

func failing(_ context.Context, argv ...string) ([]byte, error) {
	return nil, &wmconfig.ToolError{Argv: argv, Exit: 1, Stderr: "lys-wmctl: lys-wm: broken", Err: errors.New("exit status 1")}
}

func TestPointersAreTheDevicesAnInputSectionConfigures(t *testing.T) {
	run := answering(map[string]string{"devices": `[
		{"name":"AT keyboard","capabilities":["keyboard"],"class":"none","layout":{"index":0,"name":"English (US)"}},
		{"name":"Synaptics","capabilities":["pointer"],"class":"touchpad"},
		{"name":"virtual pointer","capabilities":["pointer"],"class":"none"},
		{"name":"G305","capabilities":["pointer"],"class":"pointer"}]`})
	got, err := Pointers(run)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(got))
	for i, d := range got {
		names[i] = d.Name + ":" + string(d.Class)
	}
	if want := []string{"Synaptics:touchpad", "G305:pointer"}; !slices.Equal(names, want) {
		t.Fatalf("pointers %v, want %v", names, want)
	}
}

func TestPointersFailAsDefined(t *testing.T) {
	for _, c := range []struct {
		name string
		run  wmconfig.Runner
		want error
	}{
		{"no lys-wm", notRunning, ErrNotRunning},
		{"a lys-wm that predates classes", answering(map[string]string{"devices": `[{"name":"Synaptics","capabilities":["pointer"]}]`}), ErrOutdated},
	} {
		if _, err := Pointers(c.run); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}
	if _, err := Pointers(failing); err == nil || errors.Is(err, ErrNotRunning) || !strings.Contains(err.Error(), "broken") {
		t.Errorf("a failing lys-wmctl: %v, want its own error", err)
	}
	if _, err := Pointers(answering(map[string]string{"devices": `{"oops":1}`})); err == nil || errors.Is(err, ErrOutdated) {
		t.Errorf("an answer that is no list: %v, want a decoding error", err)
	}
}

func TestTheRunningLysWMReportsEachDevicesClass(t *testing.T) {
	wmtest.Start(t, "")
	got, err := Pointers(wmconfig.Exec)
	if err != nil {
		t.Fatalf("the headless lys-wm's devices: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("the headless lys-wm's virtual keyboard and pointer, which no input section configures, are pointers: %+v", got)
	}
}

func TestNoRunningLysWMIsNotRunning(t *testing.T) {
	wmtest.Need(t)
	wmtest.Private(t)
	if _, err := Pointers(wmconfig.Exec); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("no lys-wm: %v, want %v", err, ErrNotRunning)
	}
}
