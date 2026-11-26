package portal

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stubbedev/lys/desktop/opener/openertest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

const emailIfaceName = "org.freedesktop.impl.portal.Email.ComposeEmail"

func TestMailtoIsAnRFC6068URI(t *testing.T) {
	for _, tc := range []struct {
		name string
		m    message
		want string
	}{
		{"empty", message{}, "mailto:"},
		{
			"several recipients",
			message{to: []string{"x@y.com", "z@w.com"}, cc: []string{"c@d.com", "e@f.com"}, bcc: []string{"g@h.com"}},
			"mailto:x@y.com,z@w.com?cc=c@d.com,e@f.com&bcc=g@h.com",
		},
		{
			"delimiters in an address are escaped",
			message{to: []string{"a?b&c%d@e.com"}},
			"mailto:a%3Fb%26c%25d@e.com",
		},
		{
			"subject and body",
			message{subject: "Hi there & #1?", body: "Line & stuff~\nnext\r\nlast"},
			"mailto:?subject=Hi%20there%20%26%20%231%3F&body=Line%20%26%20stuff~%0D%0Anext%0D%0Alast",
		},
		{
			"UTF-8 as its bytes",
			message{subject: "Grüße"},
			"mailto:?subject=Gr%C3%BC%C3%9Fe",
		},
	} {
		if got := tc.m.mailto(); got != tc.want {
			t.Errorf("%s: mailto = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestParseMessageReadsRecipientsAndAttachments(t *testing.T) {
	m, err := parseMessage(Vardict{
		"addresses":   dbus.MakeVariant([]string{"x@y.com"}),
		"address":     dbus.MakeVariant("a@b.com"),
		"subject":     dbus.MakeVariant("S"),
		"attachments": dbus.MakeVariant([]string{"/doc/a.txt", "file:///doc/b%20c.pdf"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	want := message{to: []string{"x@y.com", "a@b.com"}, subject: "S", attachments: []string{"/doc/a.txt", "/doc/b c.pdf"}}
	if !reflect.DeepEqual(m, want) {
		t.Errorf("message = %+v, want %+v", m, want)
	}
	for _, bad := range []string{"doc/a.txt", "https://example.com/a.txt"} {
		if _, err := parseMessage(Vardict{"attachments": dbus.MakeVariant([]string{bad})}); err == nil {
			t.Errorf("attachment %q was taken as a file", bad)
		}
	}
}

var attaching = message{
	to:          []string{"a@b.com", "c@d.com"},
	cc:          []string{"e@f.com"},
	bcc:         []string{"g@h.com"},
	subject:     "Hi, there",
	body:        "One\ntwo",
	attachments: []string{"/doc/a b.txt", "/doc/x,y'z.pdf"},
}

func TestEachMailClientsAttachmentCommandLine(t *testing.T) {
	thunderbird := argsLaunch{"-compose", "to='a@b.com,c@d.com',cc='e@f.com',bcc='g@h.com',subject=Hi%2C%20there,body=One%0D%0Atwo,attachment='file:///doc/a%20b.txt,file:///doc/x%2Cy%27z.pdf'"}
	for _, tc := range []struct {
		program string
		want    launch
	}{
		{"thunderbird", thunderbird},
		{"thunderbird-esr", thunderbird},
		{"icedove", thunderbird},
		{"evolution", uriLaunch("mailto:a@b.com,c@d.com?cc=e@f.com&bcc=g@h.com&subject=Hi%2C%20there&body=One%0D%0Atwo&attach=file:///doc/a%2520b.txt&attach=file:///doc/x%2Cy%27z.pdf")},
		{"kmail", argsLaunch{"--composer", "--subject", "Hi, there", "--cc", "e@f.com", "--bcc", "g@h.com", "--body", "One\ntwo", "--attach", "file:///doc/a%20b.txt", "--attach", "file:///doc/x,y'z.pdf", "--", "a@b.com", "c@d.com"}},
	} {
		got, err := attachLaunch(tc.program, attaching)
		if err != nil {
			t.Errorf("%s: %v", tc.program, err)
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: launch = %q, want %q", tc.program, got, tc.want)
		}
	}
}

func TestAttachmentsAreRefusedByAClientWithoutAWayToTakeThem(t *testing.T) {
	for _, program := range []string{"claws-mail", "geary", "mutt", "kmailservice", "evolution-alarm-notify"} {
		_, err := attachLaunch(program, attaching)
		var unsupported *attachmentsUnsupportedError
		if !errors.As(err, &unsupported) || unsupported.program != program || unsupported.attachments != 2 {
			t.Errorf("%s: err = %v, want attachments refused", program, err)
		}
	}
	quoted := attaching
	quoted.to = []string{"o'brien@example.com"}
	if _, err := attachLaunch("thunderbird", quoted); err == nil {
		t.Error("thunderbird -compose took an address it cannot quote")
	}
}

// The default the mimeapps.list names opens the compose window, raised by
// the request's token, even when another app also declares mailto.
func TestEmailOpensTheDefaultMailtoHandler(t *testing.T) {
	desktop := openertest.New(t)
	other := desktop.Handler(t, "othermail", mailtoType)
	mail := desktop.Handler(t, "mail", mailtoType)
	desktop.Write(t, "config/mimeapps.list", "[Default Applications]\n"+mailtoType+"=mail.desktop;\n")
	r := newRig(t, nil)
	opts := Vardict{
		"address":          dbus.MakeVariant("a@b.com"),
		"subject":          dbus.MakeVariant("Hi there"),
		"activation_token": dbus.MakeVariant("tok"),
	}
	if code, _ := r.interactive(t, emailIfaceName, handle, "org.app", "", opts); code != ResponseSuccess {
		t.Errorf("code = %d", code)
	}
	if got := mail.Ran(t); got != "mailto:a@b.com?subject=Hi%20there|tok" {
		t.Errorf("the default ran with %q", got)
	}
	if other.HasRun() {
		t.Error("an app that is not the default composed the mail")
	}
}

// attachment_fds never reaches a backend through the frontend; on its
// own it attaches nothing.
func TestEmailIgnoresAttachmentFds(t *testing.T) {
	desktop := openertest.New(t)
	mail := desktop.Handler(t, "mail", mailtoType)
	r := newRig(t, nil)
	opts := Vardict{
		"address":        dbus.MakeVariant("a@b.com"),
		"attachment_fds": dbus.MakeVariant([]dbus.UnixFD{}),
	}
	if code, _ := r.interactive(t, emailIfaceName, handle, "org.app", "", opts); code != ResponseSuccess {
		t.Errorf("code = %d", code)
	}
	if got := mail.Ran(t); got != "mailto:a@b.com|" {
		t.Errorf("the client ran with %q, want the plain mailto", got)
	}
}

func TestEmailAttachesThroughTheClientsCommandLine(t *testing.T) {
	desktop := openertest.New(t)
	thunderbird := desktop.Handler(t, "thunderbird", mailtoType)
	r := newRig(t, nil)
	opts := Vardict{
		"address":          dbus.MakeVariant("a@b.com"),
		"attachments":      dbus.MakeVariant([]string{"/doc/a.txt"}),
		"activation_token": dbus.MakeVariant("tok"),
	}
	if code, _ := r.interactive(t, emailIfaceName, handle, "org.app", "", opts); code != ResponseSuccess {
		t.Errorf("code = %d", code)
	}
	if got := thunderbird.Ran(t); got != "-compose to='a@b.com',attachment='file:///doc/a.txt'|tok" {
		t.Errorf("thunderbird ran with %q", got)
	}
}

func TestEmailFailsForAClientWithoutAttachments(t *testing.T) {
	desktop := openertest.New(t)
	mail := desktop.Handler(t, "mail", mailtoType)
	r := newRig(t, nil)
	opts := Vardict{
		"address":     dbus.MakeVariant("a@b.com"),
		"attachments": dbus.MakeVariant([]string{"/doc/a.txt"}),
	}
	if code, _ := r.interactive(t, emailIfaceName, handle, "org.app", "", opts); code != ResponseOther {
		t.Errorf("code = %d, want other", code)
	}
	if mail.HasRun() {
		t.Error("the client opened a compose window without the attachments")
	}
}

func TestEmailFailsWithoutAHandler(t *testing.T) {
	openertest.New(t)
	if err := composeEmail(Vardict{"address": dbus.MakeVariant("a@b.com")}, ""); !errors.Is(err, errNoMailClient) {
		t.Errorf("err = %v, want %v", err, errNoMailClient)
	}
	r := newRig(t, nil)
	if code, _ := r.interactive(t, emailIfaceName, handle, "org.app", "", Vardict{}); code != ResponseOther {
		t.Errorf("code = %d, want other", code)
	}
}
