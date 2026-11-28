package portal

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/stubbedev/lys/desktop/desktopentry"
	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/fileuri"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/shell/service/launcher"
)

// email is org.freedesktop.impl.portal.Email: the user's default mail
// client, the x-scheme-handler/mailto default of the mimeapps.list
// files, opens a pre-filled compose window, on an RFC 6068 mailto: URI
// when there are no attachments and through mailClients when there are.
// The frontend turns the app's attachment_fds into the attachments
// list of paths it passes here; a mail client that cannot take them is
// a failure, never a compose window silently missing the files.
type email struct{ conn *dbus.Conn }

func emailIface(conn *dbus.Conn) dbusx.Interface {
	return dbusx.Interface{
		Name:    "org.freedesktop.impl.portal.Email",
		Methods: email{conn},
	}
}

// ComposeEmail opens the compose window.
func (e email) ComposeEmail(handle dbus.ObjectPath, appID, parent string, options Vardict) (uint32, Vardict, *dbus.Error) {
	c, ok := beginOr(e.conn, handle, appID, parent, nil)
	if !ok {
		return ResponseOther, Vardict{}, nil
	}
	defer c.end()
	token, _ := optString(options, "activation_token")
	if err := composeEmail(options, token); err != nil {
		warnf("email: %v", err)
		return ResponseOther, Vardict{}, nil
	}
	return ResponseSuccess, Vardict{}, nil
}

const mailtoType = "x-scheme-handler/mailto"

// errNoMailClient is a session with no default app for mailto: URIs.
var errNoMailClient = errors.New("no application is the default for " + mailtoType)

// composeEmail opens the request's compose window in the default mail
// client, raised by token ("" for none).
func composeEmail(options Vardict, token string) error {
	m, err := parseMessage(options)
	if err != nil {
		return err
	}
	app, ok := desktopentry.Default(desktopentry.ApplicationDirs(), desktopentry.MimeAppsLists(), mailtoType)
	if !ok {
		return errNoMailClient
	}
	l, err := composeLaunch(app, m)
	if err != nil {
		return fmt.Errorf("compose with %s: %w", app.ID, err)
	}
	ctx := desktopentry.LaunchContext{
		ActivationToken: token,
		Terminal:        func() (string, error) { return launcher.DetectTerminal("") },
	}
	if err := l.start(app, ctx); err != nil {
		return fmt.Errorf("compose with %s: %w", app.ID, err)
	}
	return nil
}

// composeLaunch is how app opens m: on its mailto: URI without
// attachments, else as the mailClients entry for the program its Exec
// runs.
func composeLaunch(app desktopentry.App, m message) (launch, error) {
	if len(m.attachments) == 0 {
		return uriLaunch(m.mailto()), nil
	}
	argv, err := app.ExpandExec(app.Exec())
	if err != nil {
		return nil, err
	}
	return attachLaunch(filepath.Base(argv[0]), m)
}

// message is a ComposeEmail request's options, read once.
type message struct {
	to, cc, bcc   []string
	subject, body string
	// attachments are absolute paths.
	attachments []string
}

// parseMessage reads the options: the addresses list and the single
// address are the recipients, and each attachment is an absolute path
// or a file:// URI, as the interface describes them.
func parseMessage(options Vardict) (message, error) {
	m := message{
		to:  stringList(options, "addresses"),
		cc:  stringList(options, "cc"),
		bcc: stringList(options, "bcc"),
	}
	if s, ok := optString(options, "address"); ok {
		m.to = append(m.to, s)
	}
	m.subject, _ = optString(options, "subject")
	m.body, _ = optString(options, "body")
	for _, a := range stringList(options, "attachments") {
		path := a
		if p, ok := fileuri.Path(a); ok {
			path = p
		}
		if !filepath.IsAbs(path) {
			return message{}, fmt.Errorf("attachment %q is neither an absolute path nor a file URI", a)
		}
		m.attachments = append(m.attachments, path)
	}
	return m, nil
}

// mailto is m's RFC 6068 URI, without its attachments.
func (m message) mailto() string { return mailtoURI(m.to, m.fields()) }

// fields are m's hfields after the recipients: cc, bcc, subject and
// body, each value percent-encoded.
func (m message) fields() []string {
	var fields []string
	for _, f := range []struct {
		name  string
		addrs []string
	}{{"cc", m.cc}, {"bcc", m.bcc}} {
		if len(f.addrs) > 0 {
			fields = append(fields, f.name+"="+encodeAddresses(f.addrs))
		}
	}
	return append(fields, m.textFields()...)
}

// textFields are the subject and body hfields, line breaks in the body
// as the CRLF RFC 6068 requires.
func (m message) textFields() []string {
	var fields []string
	if m.subject != "" {
		fields = append(fields, "subject="+fileuri.Encode(m.subject, ""))
	}
	if m.body != "" {
		crlf := strings.ReplaceAll(strings.ReplaceAll(m.body, "\r\n", "\n"), "\n", "\r\n")
		fields = append(fields, "body="+fileuri.Encode(crlf, ""))
	}
	return fields
}

// mailtoURI is "mailto:" with the recipients, then the hfields as its
// query.
func mailtoURI(to, fields []string) string {
	uri := "mailto:" + encodeAddresses(to)
	if len(fields) > 0 {
		uri += "?" + strings.Join(fields, "&")
	}
	return uri
}

// encodeAddresses is a comma-separated address list, each address
// percent-encoded but for its @.
func encodeAddresses(addrs []string) string {
	encoded := make([]string, len(addrs))
	for i, a := range addrs {
		encoded[i] = fileuri.Encode(a, "@")
	}
	return strings.Join(encoded, ",")
}

// launch is how a mail client is started on a message.
type launch interface {
	start(app desktopentry.App, ctx desktopentry.LaunchContext) error
}

// uriLaunch opens the client on a mailto: URI.
type uriLaunch string

func (u uriLaunch) start(app desktopentry.App, ctx desktopentry.LaunchContext) error {
	return app.Open(string(u), ctx)
}

// argsLaunch runs the client's Exec with these arguments after it.
type argsLaunch []string

func (a argsLaunch) start(app desktopentry.App, ctx desktopentry.LaunchContext) error {
	return app.LaunchArgs(a, ctx)
}

// mailClient is a mail program lys can hand attachments to: the
// programs it is, and the launch that composes a message with them.
type mailClient struct {
	is      func(program string) bool
	compose func(m message) (launch, error)
}

// mailClients is xdg-email's per-client table (xdg-utils v1.2.1,
// scripts/xdg-email.in). Thunderbird and Icedove, matched as its
// *thunderbird*|*icedove* glob, take run_thunderbird's -compose line;
// Evolution takes the attach= hfields xdg-email puts in the mailto: URI
// it hands every other client; KMail, which xdg-email reaches through
// kmailservice or kde-open with that URI, takes its own --attach
// options. No other client has a way xdg-email knows of to receive
// files.
var mailClients = []mailClient{
	{
		is:      func(p string) bool { return strings.Contains(p, "thunderbird") || strings.Contains(p, "icedove") },
		compose: thunderbirdCompose,
	},
	{is: func(p string) bool { return p == "evolution" }, compose: evolutionCompose},
	{is: func(p string) bool { return p == "kmail" }, compose: kmailCompose},
}

// attachmentsUnsupportedError is a default mail client lys knows no way
// to hand files to: the request fails rather than open a compose window
// without them.
type attachmentsUnsupportedError struct {
	program     string
	attachments int
}

func (e *attachmentsUnsupportedError) Error() string {
	return fmt.Sprintf("%s takes no attachments; refusing to compose without the %d requested", e.program, e.attachments)
}

// attachLaunch composes m, attachments and all, in the mail client
// program.
func attachLaunch(program string, m message) (launch, error) {
	for _, c := range mailClients {
		if c.is(program) {
			return c.compose(m)
		}
	}
	return nil, &attachmentsUnsupportedError{program: program, attachments: len(m.attachments)}
}

// thunderbirdCompose is run_thunderbird's `-compose` argument: the
// recipient lists quoted, the subject and body as the mailto: URI
// encodes them, the attachments one quoted list of file URIs, with the
// separators "," and "'" escaped.
func thunderbirdCompose(m message) (launch, error) {
	var fields []string
	for _, f := range []struct {
		name  string
		addrs []string
	}{{"to", m.to}, {"cc", m.cc}, {"bcc", m.bcc}} {
		if len(f.addrs) == 0 {
			continue
		}
		for _, a := range f.addrs {
			if strings.ContainsRune(a, '\'') {
				return nil, fmt.Errorf("thunderbird -compose cannot quote the address %q", a)
			}
		}
		fields = append(fields, f.name+"='"+strings.Join(f.addrs, ",")+"'")
	}
	fields = append(fields, m.textFields()...)
	uris := make([]string, len(m.attachments))
	for i, path := range m.attachments {
		uris[i] = "file://" + fileuri.Encode(path, "/")
	}
	fields = append(fields, "attachment='"+strings.Join(uris, ",")+"'")
	return argsLaunch{"-compose", strings.Join(fields, ",")}, nil
}

// evolutionCompose is the mailto: URI with an attach= hfield per file,
// its file URI percent-encoded as a value.
func evolutionCompose(m message) (launch, error) {
	fields := m.fields()
	for _, path := range m.attachments {
		fields = append(fields, "attach="+fileuri.Encode(fileuri.FromPath(path), "/:@"))
	}
	return uriLaunch(mailtoURI(m.to, fields)), nil
}

// kmailCompose is KMail's composer command line: --cc and --bcc as
// address lists, an --attach URL per file, and the recipients after
// "--".
func kmailCompose(m message) (launch, error) {
	args := argsLaunch{"--composer"}
	if m.subject != "" {
		args = append(args, "--subject", m.subject)
	}
	if len(m.cc) > 0 {
		args = append(args, "--cc", strings.Join(m.cc, ","))
	}
	if len(m.bcc) > 0 {
		args = append(args, "--bcc", strings.Join(m.bcc, ","))
	}
	if m.body != "" {
		args = append(args, "--body", m.body)
	}
	for _, path := range m.attachments {
		args = append(args, "--attach", fileuri.FromPath(path))
	}
	return append(append(args, "--"), m.to...), nil
}

// stringList reads an `as` option.
func stringList(options Vardict, key string) []string {
	list, _ := options[key].Value().([]string)
	return list
}
