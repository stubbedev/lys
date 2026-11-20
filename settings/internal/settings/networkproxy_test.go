package settings

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/network"
	"github.com/stubbedev/lys/desktop/shell/treetest"

	"github.com/stubbedev/lys/settings/internal/locale"
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
	p.proxy.choose(network.ProxyNone)
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
