package main

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

const testModule = "example.com/m"

func fixture() map[string]string {
	return map[string]string{
		"flake.nix": `{
  imports = [
    ./app/nix/package.nix
    ./app/nix/shell.nix
    (checks "app" ./app/nix/checks.nix)
  ];
}
`,
		"app/nix/package.nix": `{lysGo, lib, bubblewrap, ...}: lysGo.build {
  pname = "app";
  subPackages = ["app/cmd/lys-app"];
  postFixup = ''
    wrapProgram $out/bin/lys-app \
      --suffix PATH : "${lib.makeBinPath [ bubblewrap ]}" \
      --prefix PATH : ${tool}/bin
  '';
  module = import ./module.nix;
}
`,
		"app/nix/module.nix": `{cfg, pkgs, lib, ...}: let
  helper = with pkgs; [ hello ];
in {
  # environment.systemPackages = [pkgs.commented];
  environment.systemPackages = [cfg.package] ++ lib.optional (pkgs ? xdg-utils) pkgs.xdg-utils;
  systemd.services.app = {
    path = [ "/run/current-system/sw" ];
    serviceConfig.ExecStart = "${lib.getExe cfg.package} run";
    serviceConfig.ExecStartPre = "-${lib.getExe' cfg.package "lys-app"} check";
    serviceConfig.ExecStop = "${pkgs.coreutils}/bin/kill $MAINPID";
  };
  xdg.configFile."unit".text = ''
    ExecStart=${pkgs.foo}/bin/foo --flag
    ExecStart=
  '';
}
`,
		"app/nix/shell.nix": `{
  perSystem = {pkgs, ...}: {
    devShells.app = pkgs.mkShell {};
    environment.systemPackages = [pkgs.devonly];
  };
}
`,
		"app/nix/checks.nix": `{pkgs, ...}: {
  environment.systemPackages = [pkgs.checkonly];
  subPackages = ["app/cmd/dev"];
}
`,
		"app/units/app.service":              "[Service]\nExecStart=@lys_app@ serve\nExecStartPost=-/usr/bin/logger started\nExecReload=\n",
		"app/units/testdata/ignored.service": "[Service]\nExecStart=/usr/bin/ignored\n",
		"ci/components.json":                 `{"components": {"repository": {"go-packages": "nix/go-packages.json"}, "app": {}}}`,
		"nix/go-packages.json": `{
  "app/cmd/lys-app": {"imports": ["app/internal/run", "app/internal/names"]},
  "app/cmd/dev": {"imports": ["app/internal/devonly"]},
  "app/internal/run": {"imports": ["app/internal/names"]},
  "app/internal/names": {},
  "app/internal/devonly": {}
}
`,
		"app/cmd/lys-app/main.go": `package main

import (
	"os"
	run "os/exec"
	"syscall"

	"golang.org/x/sys/unix"

	"example.com/m/app/internal/names"
)

const prefix = "lys-"

func main() {
	_ = run.Command(prefix + "app")
	_ = run.CommandContext(nil, names.Program, "x")
	_, _ = os.StartProcess(os.Args[0], nil, nil)
	_ = syscall.Exec("/bin/true", nil, nil)
	_ = unix.Exec(self, nil, nil)
}

type server[T any] struct{}

func (s *server[T]) start(args []string) {
	const local = "local-tool"
	_ = run.Command(local)
	_ = run.Command(args[0], args[1:]...)
	_ = &run.Cmd{Path: helper}
	_ = run.Cmd{Args: args}
}
`,
		"app/cmd/lys-app/more.go": `package main

const self = "/proc/self/exe"

const helper = "help" + "er"
`,
		"app/cmd/lys-app/main_test.go": `package main

import "os/exec"

func helperForTests() { _ = exec.Command("test-only") }
`,
		"app/cmd/lys-app/darwin.go": `//go:build darwin

package main

import "os/exec"

func darwinOnly() { _ = exec.Command("launchctl") }
`,
		"app/internal/run/run.go": `package run

import "os/exec"

var lookPath = exec.LookPath

func shadowed() {
	exec := struct{ Command func(string) }{}
	exec.Command("not-a-process")
}

func Program() { _ = exec.Command("sh", "-c", "true") }
`,
		"app/internal/names/names.go":     "package names\n\nconst Program = name\n\nconst name = \"lys-other\"\n",
		"app/internal/devonly/devonly.go": "package devonly\n\nimport \"os/exec\"\n\nfunc Run() { _ = exec.Command(\"dev-tool\") }\n",
		"app/cmd/dev/main.go":             "package main\n\nimport _ \"example.com/m/app/internal/devonly\"\n\nfunc main() {}\n",
	}
}

const fixtureAllowlist = `{
  "go": [
    {"package": "app/cmd/lys-app", "function": "main", "program": "lys-app", "class": "own", "reason": "itself"},
    {"package": "app/cmd/lys-app", "function": "main", "program": "lys-other", "class": "own", "reason": "a sibling"},
    {"package": "app/cmd/lys-app", "function": "main", "program": "dynamic", "class": "own", "reason": "its argv[0]"},
    {"package": "app/cmd/lys-app", "function": "main", "program": "/bin/true", "class": "integration", "reason": "true"},
    {"package": "app/cmd/lys-app", "function": "main", "program": "/proc/self/exe", "class": "own", "reason": "re-exec"},
    {"package": "app/cmd/lys-app", "function": "server.start", "program": "local-tool", "class": "replace", "issue": 1, "reason": "a tool"},
    {"package": "app/cmd/lys-app", "function": "server.start", "program": "dynamic", "class": "integration", "reason": "the user's argv"},
    {"package": "app/cmd/lys-app", "function": "server.start", "program": "helper", "class": "integration", "reason": "a helper"},
    {"package": "app/internal/run", "function": "lookPath", "program": "dynamic", "class": "integration", "reason": "a lookup"},
    {"package": "app/internal/run", "function": "Program", "program": "sh", "class": "integration", "reason": "the user's commands"}
  ],
  "packaging": [
    {"file": "app/nix/package.nix", "program": "bubblewrap", "class": "replace", "issue": 2, "reason": "bwrap"},
    {"file": "app/nix/package.nix", "program": "${tool}/bin", "class": "integration", "reason": "a tool"},
    {"file": "app/nix/module.nix", "program": "[cfg.package] ++ lib.optional (pkgs ? xdg-utils) pkgs.xdg-utils", "class": "replace", "issue": 3, "reason": "xdg-email"},
    {"file": "app/nix/module.nix", "program": "[ \"/run/current-system/sw\" ]", "class": "integration", "reason": "the system profile"},
    {"file": "app/nix/module.nix", "program": "${lib.getExe cfg.package}", "class": "own", "reason": "the package"},
    {"file": "app/nix/module.nix", "program": "kill", "class": "integration", "reason": "stop"},
    {"file": "app/nix/module.nix", "program": "foo", "class": "integration", "reason": "foo"},
    {"file": "app/units/app.service", "program": "logger", "class": "integration", "reason": "a log line"}
  ]
}
`

func scanFixture(t *testing.T, files map[string]string) (tree, error) {
	t.Helper()
	fsys := fstest.MapFS{}
	for name, src := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(src)}
	}
	return scanTree(fsys, testModule)
}

func mustScanFixture(t *testing.T, files map[string]string) tree {
	t.Helper()
	found, err := scanFixture(t, files)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	return found
}

func problemsOf(t *testing.T, files map[string]string, allow string) []string {
	t.Helper()
	list, err := parseAllowlist("allow.json", []byte(allow))
	if err != nil {
		t.Fatalf("parse the allowlist: %v", err)
	}
	var out []string
	for _, p := range check("allow.json", list, mustScanFixture(t, files)) {
		out = append(out, p.String())
	}
	return out
}

func keysAt[K comparable](sites []site[K]) map[string]K {
	out := map[string]K{}
	for _, s := range sites {
		out[s.at.String()] = s.key
	}
	return out
}

func TestProcessStartsResolveTheirProgramThroughAliasesAndConstants(t *testing.T) {
	got := keysAt(mustScanFixture(t, fixture()).goSites)
	main, start := goKey{"app/cmd/lys-app", "main", ""}, goKey{"app/cmd/lys-app", "server.start", ""}
	with := func(k goKey, program string) goKey { k.program = program; return k }
	want := map[string]goKey{
		"app/cmd/lys-app/main.go:16": with(main, "lys-app"),
		"app/cmd/lys-app/main.go:17": with(main, "lys-other"),
		"app/cmd/lys-app/main.go:18": with(main, dynamic),
		"app/cmd/lys-app/main.go:19": with(main, "/bin/true"),
		"app/cmd/lys-app/main.go:20": with(main, "/proc/self/exe"),
		"app/cmd/lys-app/main.go:27": with(start, "local-tool"),
		"app/cmd/lys-app/main.go:28": with(start, dynamic),
		"app/cmd/lys-app/main.go:29": with(start, "helper"),
		"app/cmd/lys-app/main.go:30": with(start, dynamic),
		"app/internal/run/run.go:5":  {"app/internal/run", "lookPath", dynamic},
		"app/internal/run/run.go:12": {"app/internal/run", "Program", "sh"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Go sites:\n%v\nwant:\n%v", got, want)
	}
}

func TestPackagingSitesComeOnlyFromTheFlakesPackagesModulesAndUnits(t *testing.T) {
	got := keysAt(mustScanFixture(t, fixture()).packagingSites)
	want := map[string]packagingKey{
		"app/nix/package.nix:6":   {"app/nix/package.nix", "bubblewrap"},
		"app/nix/package.nix:7":   {"app/nix/package.nix", "${tool}/bin"},
		"app/nix/module.nix:5":    {"app/nix/module.nix", "[cfg.package] ++ lib.optional (pkgs ? xdg-utils) pkgs.xdg-utils"},
		"app/nix/module.nix:7":    {"app/nix/module.nix", `[ "/run/current-system/sw" ]`},
		"app/nix/module.nix:8":    {"app/nix/module.nix", "${lib.getExe cfg.package}"},
		"app/nix/module.nix:10":   {"app/nix/module.nix", "kill"},
		"app/nix/module.nix:13":   {"app/nix/module.nix", "foo"},
		"app/units/app.service:3": {"app/units/app.service", "logger"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("packaging sites:\n%v\nwant:\n%v", got, want)
	}
}

func TestAnAllowlistedTreePasses(t *testing.T) {
	if problems := problemsOf(t, fixture(), fixtureAllowlist); len(problems) != 0 {
		t.Fatalf("problems:\n%s", strings.Join(problems, "\n"))
	}
}

func TestANewProcessStartFailsAtItsLine(t *testing.T) {
	files := fixture()
	files["app/internal/run/new.go"] = "package run\n\nimport \"os/exec\"\n\nfunc fresh() {\n\t_ = exec.Command(\"curl\")\n}\n"
	want := []string{`app/internal/run/new.go:6: os/exec.Command (package app/internal/run, function fresh, program "curl") has no go entry in allow.json`}
	if got := problemsOf(t, files, fixtureAllowlist); !slices.Equal(got, want) {
		t.Fatalf("problems = %q, want %q", got, want)
	}
}

func TestANewPackagingProgramFailsAtItsLine(t *testing.T) {
	files := fixture()
	files["app/units/new.service"] = "[Service]\nExecStart=/usr/bin/curl example.org\n"
	want := []string{`app/units/new.service:2: ExecStart runs (file app/units/new.service, program "curl") has no packaging entry in allow.json`}
	if got := problemsOf(t, files, fixtureAllowlist); !slices.Equal(got, want) {
		t.Fatalf("problems = %q, want %q", got, want)
	}
}

func TestAStaleEntryFailsAtItsLine(t *testing.T) {
	files := fixture()
	files["app/internal/run/run.go"] = "package run\n\nimport \"os/exec\"\n\nvar lookPath = exec.LookPath\n"
	want := []string{`allow.json:12: the go entry (package app/internal/run, function Program, program "sh") matches nothing in the tree: delete it`}
	if got := problemsOf(t, files, fixtureAllowlist); !slices.Equal(got, want) {
		t.Fatalf("problems = %q, want %q", got, want)
	}
}

func TestADynamicEntryCoversOnlyItsOwnFunction(t *testing.T) {
	files := fixture()
	files["app/internal/run/other.go"] = "package run\n\nimport \"os/exec\"\n\nfunc other(name string) { _ = exec.Command(name) }\n"
	want := []string{`app/internal/run/other.go:5: os/exec.Command (package app/internal/run, function other, program "dynamic") has no go entry in allow.json`}
	if got := problemsOf(t, files, fixtureAllowlist); !slices.Equal(got, want) {
		t.Fatalf("problems = %q, want %q", got, want)
	}
}

func TestALiteralProgramDoesNotMatchADynamicEntry(t *testing.T) {
	files := fixture()
	files["app/internal/run/run.go"] = strings.Replace(files["app/internal/run/run.go"], `exec.Command("sh", "-c", "true")`, `exec.Command("bash", "-c", "true")`, 1)
	got := problemsOf(t, files, fixtureAllowlist)
	want := []string{
		`allow.json:12: the go entry (package app/internal/run, function Program, program "sh") matches nothing in the tree: delete it`,
		`app/internal/run/run.go:12: os/exec.Command (package app/internal/run, function Program, program "bash") has no go entry in allow.json`,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("problems = %q, want %q", got, want)
	}
}

func TestADotImportOfAProcessPackageIsRejected(t *testing.T) {
	files := fixture()
	files["app/internal/run/dot.go"] = "package run\n\nimport . \"os/exec\"\n\nfunc dot() { _ = Command(\"x\") }\n"
	_, err := scanFixture(t, files)
	if err == nil || !strings.Contains(err.Error(), "app/internal/run/dot.go:3:8: a dot import of os/exec hides its process starts") {
		t.Fatalf("scan error = %v, want the dot import refused", err)
	}
}

func TestAMainTheFlakeDoesNotShipIsNotScanned(t *testing.T) {
	found := mustScanFixture(t, fixture())
	for _, s := range found.goSites {
		if s.key.pkg == "app/internal/devonly" {
			t.Fatalf("scanned %v, which only a check's package imports", s)
		}
	}
}

func TestAnUnlistedSubPackageFails(t *testing.T) {
	files := fixture()
	files["app/nix/package.nix"] = strings.Replace(files["app/nix/package.nix"], `["app/cmd/lys-app"]`, `["app/cmd/lys-app" "app/cmd/missing"]`, 1)
	_, err := scanFixture(t, files)
	if err == nil || !strings.Contains(err.Error(), "nix/go-packages.json names no package app/cmd/missing") {
		t.Fatalf("scan error = %v, want the missing package named", err)
	}
}

func TestTheGoPackageGraphIsTheOneTheComponentsName(t *testing.T) {
	files := fixture()
	files["ci/components.json"] = `{"components": {"repository": {"go-packages": "nix/graph.json"}}}`
	files["nix/graph.json"] = files["nix/go-packages.json"]
	delete(files, "nix/go-packages.json")
	mustScanFixture(t, files)
	for _, components := range []string{
		`{"components": {"app": {}}}`,
		`{"components": {"repository": {"go-packages": "nix/graph.json"}, "app": {"go-packages": "nix/other.json"}}}`,
	} {
		files["ci/components.json"] = components
		if _, err := scanFixture(t, files); err == nil || !strings.Contains(err.Error(), "want one") {
			t.Errorf("components %s: scan error = %v, want one graph asked for", components, err)
		}
	}
}

func TestEntriesAreRejectedWithTheirLine(t *testing.T) {
	cases := map[string]struct {
		entry string
		want  string
	}{
		"no reason": {
			`{"package": "p", "function": "f", "program": "x", "class": "own", "reason": " "}`,
			"allow.json:3: the entry gives no reason",
		},
		"replace without an issue": {
			`{"package": "p", "function": "f", "program": "x", "class": "replace", "reason": "r"}`,
			"allow.json:3: a replace entry names the issue that replaces it",
		},
		"an issue on an own entry": {
			`{"package": "p", "function": "f", "program": "x", "class": "own", "issue": 4, "reason": "r"}`,
			"allow.json:3: the entry is class own and names issue 4, which only a replace entry does",
		},
		"an unknown class": {
			`{"package": "p", "function": "f", "program": "x", "class": "dev", "reason": "r"}`,
			`class "dev" is none of integration, own, replace`,
		},
		"no class": {
			`{"package": "p", "function": "f", "program": "x", "reason": "r"}`,
			"allow.json:3: the entry gives no class",
		},
		"no function": {
			`{"package": "p", "program": "x", "class": "own", "reason": "r"}`,
			"allow.json:3: a go entry names its package, function and program",
		},
		"an unknown field": {
			`{"package": "p", "function": "f", "program": "x", "class": "own", "reason": "r", "note": "n"}`,
			`unknown field "note"`,
		},
		"a repeated entry": {
			"{\"package\": \"p\", \"function\": \"f\", \"program\": \"x\", \"class\": \"own\", \"reason\": \"r\"},\n" +
				`    {"package": "p", "function": "f", "program": "x", "class": "own", "reason": "again"}`,
			`allow.json:4: package p, function f, program "x" repeats the entry on line 3`,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseAllowlist("allow.json", []byte("{\n  \"go\": [\n    "+c.entry+"\n  ]\n}\n"))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("parse error = %v, want %q", err, c.want)
			}
		})
	}
}

func TestAPackagingEntryNeedsItsFile(t *testing.T) {
	_, err := parseAllowlist("allow.json", []byte(`{"packaging": [{"program": "x", "class": "own", "reason": "r"}]}`))
	if err == nil || !strings.Contains(err.Error(), "a packaging entry names its file and program") {
		t.Fatalf("parse error = %v, want the missing file named", err)
	}
}
