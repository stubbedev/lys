package settings

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/network"
	"github.com/stubbedev/lys/desktop/shell/treetest"

	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/proxyenv"
)

func proxiedNets(proxy network.Proxy) *fakeNet {
	f := homeNets()
	f.state.proxy = &proxyState{profile: network.Profile{Path: "/p/home", UUID: "u-home", ID: "Home"}, proxy: proxy}
	return f
}

func TestWithoutAConnectionTheProxySaysToConnectFirst(t *testing.T) {
	f := homeNets()
	_, p := netPanel(t, f)
	if !p.proxySec.Visible() || !p.proxy.none.Visible() || p.proxy.row.Visible() {
		t.Fatalf("without a primary connection the proxy section shows %v, its note %v, its picker %v", p.proxySec.Visible(), p.proxy.none.Visible(), p.proxy.row.Visible())
	}
	p.proxy.choose(proxyChoiceNone)
	if len(f.proxies) != 0 {
		t.Errorf("a proxy was written without a connection: %v", f.proxies)
	}
	if newNetworkPanel(testKit(t, "")).proxySec.Visible() {
		t.Error("the proxy section shows without NetworkManager")
	}
}

func TestTheProxyOfTheNetworkInUseIsPickedAndReachesNetworkManager(t *testing.T) {
	f := proxiedNets(network.NoProxy())
	_, p := netPanel(t, f)
	title := locale.T().Get("settings-proxy-for", i18n.Str("name", "Home"))
	if labels := strings.Join(treetest.Labels(p.proxySec), "|"); !strings.Contains(labels, title) {
		t.Fatalf("the proxy section reads %q, want %q", labels, title)
	}
	if p.proxy.methods.Active() != 0 || p.proxy.auto.Visible() {
		t.Fatalf("no proxy shows method %d, the URL field %v", p.proxy.methods.Active(), p.proxy.auto.Visible())
	}
	p.proxy.methods.SetActive(1)
	discover, _ := network.AutoProxy("")
	if !slices.Equal(f.proxies, []proxyWrite{{"/p/home", discover}}) {
		t.Fatalf("choosing Automatic wrote %v", f.proxies)
	}
	p.proxy.pac.SetText("https://wpad.home/proxy.pac")
	clickText(t, p.proxy, locale.T().Get("settings-proxy-apply"), 0)
	pac, _ := network.AutoProxy("https://wpad.home/proxy.pac")
	if got := f.proxies[len(f.proxies)-1]; got != (proxyWrite{"/p/home", pac}) {
		t.Fatalf("applying the URL wrote %v", got)
	}
	f.state.proxy.proxy = pac
	_, p = netPanel(t, f)
	if p.proxy.methods.Active() != 1 || !p.proxy.auto.Visible() || p.proxy.pac.Text() != "https://wpad.home/proxy.pac" {
		t.Fatalf("a saved PAC URL shows method %d, field %v reading %q", p.proxy.methods.Active(), p.proxy.auto.Visible(), p.proxy.pac.Text())
	}
	p.proxy.methods.SetActive(0)
	if got := f.proxies[len(f.proxies)-1]; got != (proxyWrite{"/p/home", network.NoProxy()}) {
		t.Errorf("choosing None wrote %v", got)
	}
}

func TestABadPACURLIsSaidAndNotWritten(t *testing.T) {
	discover, _ := network.AutoProxy("")
	for in, key := range map[string]string{
		"proxy.home/proxy.pac":  "settings-proxy-pac-invalid",
		"http:///proxy.pac":     "settings-proxy-pac-host",
		"file://host/proxy.pac": "settings-proxy-pac-path",
	} {
		f := proxiedNets(discover)
		_, p := netPanel(t, f)
		p.proxy.pac.SetText(in)
		p.proxy.pac.OnActivate(in)
		if len(f.proxies) != 0 || p.proxy.field.Error() != locale.T().Get(key) {
			t.Errorf("%q wrote %v and said %q, want %s", in, f.proxies, p.proxy.field.Error(), key)
		}
		p.proxy.pac.SetText("http://wpad.home/proxy.pac")
		if p.proxy.field.Error() != "" {
			t.Errorf("typing after %q left the error %q", in, p.proxy.field.Error())
		}
	}
}

func wmNetPanel(t *testing.T, f *fakeNet, config, runtime string) (*networkPanel, *wmBackend, wmFiles) {
	t.Helper()
	k, w, _, files := wmKitWith(t, config, runtime)
	k.network = func() (netControl, error) { return f, nil }
	k.pickers = &fakePickers{}
	return newNetworkPanel(k), w, files
}

func manualIndex(t *testing.T, p *networkPanel) int {
	t.Helper()
	i := slices.Index(p.proxy.choices, proxyChoiceManual)
	if i < 0 || p.proxy.methods.Len() != len(p.proxy.choices) {
		t.Fatalf("Manual is not offered among %v", p.proxy.choices)
	}
	return i
}

func typeServer(p *networkPanel, kind proxyenv.Kind, host string, port uint16) {
	in := p.proxy.servers[kind]
	in.host.SetText(host)
	in.port.SetValue(float64(port))
}

const proxiedEnv = "env:\n  EDITOR: nvim\n  http_proxy: http://proxy.corp.example:3128\n  HTTP_PROXY: http://proxy.corp.example:3128\n  all_proxy: socks5://10.0.0.1:1080\n  ALL_PROXY: socks5://10.0.0.1:1080\n  no_proxy: localhost,.corp.example\n  NO_PROXY: localhost,.corp.example\n"

func TestAManualProxyIsWrittenToLysWMsEnvironment(t *testing.T) {
	discover, _ := network.AutoProxy("")
	f := proxiedNets(discover)
	p, w, files := wmNetPanel(t, f, "env:\n  EDITOR: nvim\n", "")
	p.proxy.methods.SetActive(manualIndex(t, p))
	if !p.proxy.manual.Visible() || p.proxy.auto.Visible() || len(f.proxies) != 0 {
		t.Fatalf("picking Manual shows the servers %v, the URL %v, and wrote %v", p.proxy.manual.Visible(), p.proxy.auto.Visible(), f.proxies)
	}
	if labels := strings.Join(treetest.Labels(p.proxy.manual), "|"); !strings.Contains(labels, locale.T().Get("settings-proxy-manual-note")) {
		t.Fatalf("the manual form reads %q, without saying when it applies", labels)
	}
	typeServer(p, proxyenv.HTTP, "proxy.corp.example", 3128)
	typeServer(p, proxyenv.SOCKS, "10.0.0.1", 1080)
	p.proxy.chips.toggle("localhost", true)
	p.proxy.chips.entry.SetText(".corp.example")
	p.proxy.chips.add()
	clickText(t, p.proxy.manual, locale.T().Get("settings-proxy-apply"), 0)
	want := map[string]any{
		"EDITOR":     "nvim",
		"http_proxy": "http://proxy.corp.example:3128", "HTTP_PROXY": "http://proxy.corp.example:3128",
		"all_proxy": "socks5://10.0.0.1:1080", "ALL_PROXY": "socks5://10.0.0.1:1080",
		"no_proxy": "localhost,.corp.example", "NO_PROXY": "localhost,.corp.example",
	}
	if got := w.value(wmEnvPath); !reflect.DeepEqual(got, want) {
		t.Fatalf("lys-wm's env is %v, want %v", got, want)
	}
	if !strings.Contains(fileText(t, files.runtime), "socks5://10.0.0.1:1080") {
		t.Fatalf("the runtime file reads %q", fileText(t, files.runtime))
	}
	if !slices.Equal(f.proxies, []proxyWrite{{"/p/home", network.NoProxy()}}) {
		t.Fatalf("NetworkManager got %v, want the connection's own proxy off", f.proxies)
	}
}

func TestASavedManualProxyIsShownAsManual(t *testing.T) {
	p, _, _ := wmNetPanel(t, proxiedNets(network.NoProxy()), "", proxiedEnv)
	if p.proxy.methods.Active() != manualIndex(t, p) || !p.proxy.manual.Visible() {
		t.Fatalf("a saved manual proxy shows method %d, the servers %v", p.proxy.methods.Active(), p.proxy.manual.Visible())
	}
	if in := p.proxy.servers[proxyenv.HTTP]; in.host.Text() != "proxy.corp.example" || in.port.Value() != 3128 {
		t.Errorf("HTTP reads %q:%v", in.host.Text(), in.port.Value())
	}
	if in := p.proxy.servers[proxyenv.HTTPS]; in.host.Text() != "" {
		t.Errorf("HTTPS reads %q", in.host.Text())
	}
	if !slices.Equal(p.proxy.chips.names(), []string{"localhost", ".corp.example"}) {
		t.Errorf("the exclusions read %v", p.proxy.chips.names())
	}
	if p.proxy.problem.Visible() {
		t.Errorf("a proxy this page wrote shows %q", p.proxy.problem.Text())
	}
}

func TestNoneAndAutomaticTakeTheManualProxyOutOfLysWMsEnvironment(t *testing.T) {
	discover, _ := network.AutoProxy("")
	for _, c := range []struct {
		choice proxyChoice
		want   network.Proxy
	}{
		{proxyChoiceNone, network.NoProxy()},
		{proxyChoiceAuto, discover},
	} {
		f := proxiedNets(network.NoProxy())
		p, w, _ := wmNetPanel(t, f, "", proxiedEnv)
		p.proxy.methods.SetActive(slices.Index(p.proxy.choices, c.choice))
		if got := w.value(wmEnvPath); !reflect.DeepEqual(got, map[string]any{"EDITOR": "nvim"}) {
			t.Errorf("choice %d left lys-wm's env %v", c.choice, got)
		}
		if !slices.Equal(f.proxies, []proxyWrite{{"/p/home", c.want}}) {
			t.Errorf("choice %d wrote %v", c.choice, f.proxies)
		}
		if p.proxy.manual.Visible() {
			t.Errorf("choice %d still shows the servers", c.choice)
		}
	}
}

func TestAProxyInLysWMsConfigFileIsNotTakenOutBehindIt(t *testing.T) {
	f := proxiedNets(network.NoProxy())
	p, w, files := wmNetPanel(t, f, "env:\n  EDITOR: nvim\n  http_proxy: http://proxy:3128\n  no_proxy: localhost\n", "")
	p.proxy.methods.SetActive(0)
	want := locale.T().Get("settings-proxy-env-config", i18n.Str("names", "http_proxy, no_proxy"))
	if !p.proxy.problem.Visible() || p.proxy.problem.Text() != want {
		t.Fatalf("None says %v %q, want %q", p.proxy.problem.Visible(), p.proxy.problem.Text(), want)
	}
	if len(f.proxies) != 0 || p.proxy.methods.Active() != manualIndex(t, p) {
		t.Fatalf("None wrote %v and shows method %d", f.proxies, p.proxy.methods.Active())
	}
	noFile(t, files.runtime)
	typeServer(p, proxyenv.HTTP, "other", 8080)
	p.proxy.chips.toggle("localhost", false)
	p.proxy.apply()
	want = locale.T().Get("settings-proxy-env-config", i18n.Str("names", "no_proxy"))
	if p.proxy.problem.Text() != want || len(f.proxies) != 0 {
		t.Fatalf("dropping the config's exclusion says %q and wrote %v, want %q", p.proxy.problem.Text(), f.proxies, want)
	}
	p.proxy.chips.toggle("localhost", true)
	p.proxy.apply()
	if got, _ := w.value(wmEnvPath).(map[string]any); got["http_proxy"] != "http://other:8080" || got["HTTP_PROXY"] != "http://other:8080" || p.proxy.problem.Visible() {
		t.Fatalf("replacing the config's proxy left %v, problem %q", got, p.proxy.problem.Text())
	}
}

func TestNoneWithoutAManualProxyLeavesLysWMsConfigAlone(t *testing.T) {
	discover, _ := network.AutoProxy("")
	f := proxiedNets(discover)
	p, _, files := wmNetPanel(t, f, "", "")
	p.proxy.methods.SetActive(0)
	noFile(t, files.runtime)
	if !slices.Equal(f.proxies, []proxyWrite{{"/p/home", network.NoProxy()}}) {
		t.Fatalf("None wrote %v", f.proxies)
	}
}

func TestAManualProxyThatCannotBeOneIsSaidAndNotWritten(t *testing.T) {
	t.Run("server", func(t *testing.T) {
		f := proxiedNets(network.NoProxy())
		p, w, files := wmNetPanel(t, f, "", "")
		p.proxy.methods.SetActive(manualIndex(t, p))
		typeServer(p, proxyenv.HTTPS, "http://proxy", 8080)
		p.proxy.apply()
		if got := p.proxy.servers[proxyenv.HTTPS].field.Error(); got != locale.T().Get("settings-proxy-host-invalid") {
			t.Errorf("the HTTPS field says %q", got)
		}
		if w.value(wmEnvPath) != nil || len(f.proxies) != 0 {
			t.Errorf("a bad server wrote %v and %v", w.value(wmEnvPath), f.proxies)
		}
		noFile(t, files.runtime)
		p.proxy.servers[proxyenv.HTTPS].host.SetText("proxy")
		if got := p.proxy.servers[proxyenv.HTTPS].field.Error(); got != "" {
			t.Errorf("typing left %q", got)
		}
	})
	t.Run("no server", func(t *testing.T) {
		f := proxiedNets(network.NoProxy())
		p, w, _ := wmNetPanel(t, f, "", "")
		p.proxy.methods.SetActive(manualIndex(t, p))
		p.proxy.chips.toggle("localhost", true)
		p.proxy.apply()
		if !p.proxy.problem.Visible() || p.proxy.problem.Text() != locale.T().Get("settings-proxy-no-server") {
			t.Errorf("no server says %v %q", p.proxy.problem.Visible(), p.proxy.problem.Text())
		}
		if w.value(wmEnvPath) != nil || len(f.proxies) != 0 {
			t.Errorf("no server wrote %v and %v", w.value(wmEnvPath), f.proxies)
		}
		if p.proxy.methods.Active() != manualIndex(t, p) {
			t.Errorf("the picker fell back to %d", p.proxy.methods.Active())
		}
	})
	t.Run("exclusion", func(t *testing.T) {
		p, _, _ := wmNetPanel(t, proxiedNets(network.NoProxy()), "", "")
		p.proxy.methods.SetActive(manualIndex(t, p))
		p.proxy.chips.toggle("localhost", true)
		for in, key := range map[string]string{"corp.example:443": "settings-proxy-exclude-invalid", "localhost": "settings-proxy-exclude-listed"} {
			p.proxy.chips.entry.SetText(in)
			p.proxy.chips.add()
			if p.proxy.chips.problem.Text() != locale.T().Get(key) || !slices.Equal(p.proxy.chips.names(), []string{"localhost"}) {
				t.Errorf("%q says %q and lists %v, want %s", in, p.proxy.chips.problem.Text(), p.proxy.chips.names(), key)
			}
		}
	})
}

func TestAProxyInLysWMsEnvironmentThisPageDoesNotWriteIsSaid(t *testing.T) {
	p, _, _ := wmNetPanel(t, proxiedNets(network.NoProxy()), "env:\n  https_proxy: https://proxy:443\n", "")
	if p.proxy.methods.Active() != manualIndex(t, p) || !p.proxy.problem.Visible() {
		t.Fatalf("an unwritten proxy shows method %d, problem %v", p.proxy.methods.Active(), p.proxy.problem.Visible())
	}
	want := locale.T().Get("settings-proxy-env-unread", i18n.Str("reason", locale.T().Get("settings-proxy-env-scheme")))
	if p.proxy.problem.Text() != want {
		t.Fatalf("it says %q, want %q", p.proxy.problem.Text(), want)
	}
}

func TestWithoutLysWMManualIsNotOfferedAndTheReasonIsSaid(t *testing.T) {
	t.Run("not installed", func(t *testing.T) {
		k := testKit(t, "")
		k.parts.wm = false
		k.network = func() (netControl, error) { return proxiedNets(network.NoProxy()), nil }
		p := newNetworkPanel(k)
		if slices.Contains(p.proxy.choices, proxyChoiceManual) || p.proxy.methods.Len() != 2 || p.proxy.manual != nil {
			t.Fatalf("without lys-wm the choices are %v", p.proxy.choices)
		}
		if !p.proxy.noManual.Visible() || p.proxy.noManual.Text() != locale.T().Get("settings-proxy-manual-missing") {
			t.Fatalf("the note shows %v reading %q", p.proxy.noManual.Visible(), p.proxy.noManual.Text())
		}
	})
	t.Run("unreadable", func(t *testing.T) {
		k := testKit(t, "")
		t.Setenv("PATH", t.TempDir())
		k.network = func() (netControl, error) { return proxiedNets(network.NoProxy()), nil }
		p := newNetworkPanel(k)
		if slices.Contains(p.proxy.choices, proxyChoiceManual) {
			t.Fatalf("an unreadable lys-wm offers %v", p.proxy.choices)
		}
		prefix, _, _ := strings.Cut(locale.T().Get("settings-proxy-manual-unread", i18n.Str("reason", "\x00")), "\x00")
		if !p.proxy.noManual.Visible() || !strings.HasPrefix(p.proxy.noManual.Text(), prefix) || len(p.proxy.noManual.Text()) == len(prefix) {
			t.Fatalf("the note shows %v reading %q", p.proxy.noManual.Visible(), p.proxy.noManual.Text())
		}
	})
	t.Run("not connected", func(t *testing.T) {
		k := testKit(t, "")
		k.parts.wm = false
		k.network = func() (netControl, error) { return homeNets(), nil }
		if p := newNetworkPanel(k); p.proxy.noManual.Visible() {
			t.Fatal("the note shows without a connection to set a proxy for")
		}
	})
}
