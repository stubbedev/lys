// Package launcher is the launcher surface: a layer-shell overlay per `lys-shell launcher` session,
// hosting the service/launcher engine. Sessions arrive over the
// launcher socket; the engine runs on its own goroutine and this
// package only renders and forwards input.
package launcher

import (
	"log"
	"os"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/launchermatch"
	"github.com/stubbedev/lys/desktop/shell/launcherview"

	"github.com/stubbedev/lys/shell/internal/launcheripc"
	engine "github.com/stubbedev/lys/shell/service/launcher"
	"github.com/stubbedev/lys/shell/service/launcher/modes"
	"github.com/stubbedev/lys/shell/service/launcher/search"
)

// uiSettings are one session's resolved UI knobs.
type uiSettings struct {
	look launcherview.Look
	// widthOverride is -width, resolved late because percent and chars
	// need the monitor and the font.
	widthOverride *launcheripc.Width
	password      bool
	mesg          *string
	// errorMessage is -e: a message-only dialog.
	errorMessage *string
	filter       *string
	// prompt is -p; modes supply their own otherwise.
	prompt           *string
	displayNames     map[string]string
	keybindings      []config.LauncherBinding
	mouseBindings    []config.LauncherBinding
	hooks            engine.Hooks
	previewCmd       string
	completer        string
	cycle            bool
	autoSelect       bool
	selectRow        *string
	selectedRow      *uint32
	display          launcherview.RowDisplay
	ballotSelected   string
	ballotUnselected string
	dump             bool
}

// sessionSetup is everything the surface needs to run one session.
type sessionSetup struct {
	modes       []engine.Mode
	initialMode int
	matcher     launchermatch.Options
	ui          uiSettings
	history     *engine.History
}

// VaultModeName is the vault mode's name, which Deps.Vault builds.
const VaultModeName = "vault"

// setupDeps are the shell services a session's modes draw on.
type setupDeps struct {
	clipboard modes.ClipboardHistory
	// vault builds the vault mode (nil: none, so the mode is unknown).
	vault func() engine.Mode
	// openHistory opens the history database (a test seam).
	openHistory func() (*engine.History, error)
}

// deref returns *p or fallback.
func deref[T any](p *T, fallback T) T {
	if p == nil {
		return fallback
	}
	return *p
}

// buildSession resolves a session from the CLI's options merged over
// the live config. rows is the dmenu row stream.
func buildSession(opts launcheripc.SessionOptions, cfg *config.Config, rows <-chan []string, deps setupDeps) sessionSetup {
	l := cfg.Launcher
	var history *engine.History
	if l.History.Enable && deps.openHistory != nil {
		history = engine.DeferHistory(deps.openHistory)
	}
	bindings := engine.EffectiveKeybindings(engine.MergeOverrides(l.Keybindings, opts.KbOverrides))

	var ms []engine.Mode
	if opts.Dmenu {
		ms = append(ms, modes.NewDmenu(dmenuConfig(opts), rows))
	} else {
		for _, name := range requestedModes(opts, l) {
			if m := buildMode(name, opts, cfg, history, bindings, deps); m != nil {
				ms = append(ms, m)
			} else {
				log.Printf("launcher: mode %q not available; skipped", name)
			}
		}
	}
	initial := 0
	if opts.Mode != nil {
		for i, m := range ms {
			if m.Name() == *opts.Mode {
				initial = i
				break
			}
		}
	}
	look := launcherview.LookOf(cfg)
	look.OffsetX = int(deref(opts.XOffset, 0))
	look.OffsetY = int(deref(opts.YOffset, 0))
	look.Lines = int(deref(opts.Lines, l.Lines))
	look.Fixed = look.Fixed && !opts.NoFixedNumLines
	if opts.Location != nil {
		if loc, ok := config.LauncherLocationFromRofi(int(*opts.Location)); ok {
			look.Location = loc
		}
	}
	look.ShowIcons = deref(opts.ShowIcons, look.ShowIcons)
	look.Sidebar = deref(opts.SidebarMode, look.Sidebar)
	look.Font = deref(opts.Font, look.Font)
	look.Style = stylePreset(opts, l)
	return sessionSetup{
		modes:       ms,
		initialMode: initial,
		matcher:     matcherOptions(opts, l),
		history:     history,
		ui: uiSettings{
			look:             look,
			widthOverride:    opts.Width,
			password:         opts.Password,
			mesg:             opts.Mesg,
			errorMessage:     opts.ErrorMessage,
			filter:           opts.Filter,
			prompt:           opts.Prompt,
			displayNames:     engine.MergeOverrides(l.DisplayNames, opts.DisplayNames),
			keybindings:      bindings,
			mouseBindings:    engine.EffectiveMouseBindings(engine.MergeOverrides(l.MouseBindings, opts.MouseOverrides)),
			hooks:            hooksOf(opts),
			previewCmd:       previewCmd(opts, l),
			completer:        deref(opts.CompleterMode, ""),
			cycle:            deref(opts.Cycle, l.Cycle),
			autoSelect:       deref(opts.AutoSelect, l.AutoSelect),
			selectRow:        opts.Select,
			selectedRow:      opts.SelectedRow,
			display:          launcherview.RowDisplay{Columns: opts.DisplayColumns, Separator: deref(opts.DisplayColumnSeparator, "\t"), Ellipsize: ellipsize(opts)},
			ballotSelected:   deref(opts.BallotSelected, "☑ "),
			ballotUnselected: deref(opts.BallotUnselected, "☐ "),
			dump:             opts.Dump,
		},
	}
}

// ellipsize is -ellipsize-mode: start, middle, or anything else for end.
// -keep-right is start by another name.
func ellipsize(opts launcheripc.SessionOptions) launcherview.Ellipsize {
	if opts.KeepRight {
		return launcherview.EllipsizeStart
	}
	switch deref(opts.EllipsizeMode, "end") {
	case "start":
		return launcherview.EllipsizeStart
	case "middle":
		return launcherview.EllipsizeMiddle
	}
	return launcherview.EllipsizeEnd
}

// requestedModes is -modes, else [launcher] modes, with -show's mode
// guaranteed present (first) and the -completer-mode appended, so the
// key can reach it without it being the mode the session opens on.
func requestedModes(opts launcheripc.SessionOptions, l config.LauncherConfig) []string {
	names := append([]string(nil), l.Modes...)
	if opts.Modes != nil {
		names = append([]string(nil), opts.Modes...)
	}
	if opts.Mode != nil && !slices.Contains(names, *opts.Mode) {
		names = append([]string{*opts.Mode}, names...)
	}
	if c := deref(opts.CompleterMode, ""); c != "" && !slices.Contains(names, c) {
		names = append(names, c)
	}
	return names
}

func buildMode(name string, opts launcheripc.SessionOptions, cfg *config.Config, history *engine.History, bindings []config.LauncherBinding, deps setupDeps) engine.Mode {
	l := cfg.Launcher
	switch name {
	case "drun":
		return modes.NewDrun(drunConfig(opts, l), history)
	case "run":
		return modes.NewRun(runConfig(opts, l), history)
	case "window":
		return modes.NewWindow(windowConfig(opts, l, false))
	case "windowcd":
		return modes.NewWindow(windowConfig(opts, l, true))
	case "ssh":
		return modes.NewSSH(sshConfig(opts, l), history)
	case "filebrowser":
		return modes.NewFileBrowser(fileBrowserConfig(l, false))
	case "recursivebrowser":
		return modes.NewFileBrowser(fileBrowserConfig(l, true))
	case "keys":
		return modes.NewKeys(bindings)
	case "calc":
		return modes.NewCalc()
	case "clipboard":
		return modes.NewClipboard(deps.clipboard)
	case "emoji":
		return modes.NewEmoji(search.SkinToneOf(l.Emoji.SkinTone))
	case VaultModeName:
		if deps.vault == nil {
			return nil
		}
		return deps.vault()
	case "combi":
		var children []engine.Mode
		for _, child := range l.Combi.Modes {
			if child == "combi" {
				continue // no recursion
			}
			if m := buildMode(child, opts, cfg, history, bindings, deps); m != nil {
				children = append(children, m)
			}
		}
		if len(children) == 0 {
			return nil
		}
		return modes.NewCombi(children, deref(opts.CombiDisplayFormat, l.Combi.DisplayFormat))
	}
	// Custom script modes: name:script inline, or a [launcher.scripts] key.
	if n, script, ok := strings.Cut(name, ":"); ok {
		return modes.NewScript(n, expandHome(script))
	}
	if script, ok := l.Scripts[name]; ok {
		return modes.NewScript(name, expandHome(script))
	}
	return nil
}

func expandHome(path string) string {
	if rest, ok := strings.CutPrefix(path, "~"); ok {
		return os.Getenv("HOME") + rest
	}
	return path
}

func dmenuConfig(opts launcheripc.SessionOptions) modes.DmenuConfig {
	return modes.DmenuConfig{
		Prompt:      opts.Prompt,
		Message:     opts.Mesg,
		MarkupRows:  opts.MarkupRows,
		MultiSelect: opts.MultiSelect,
		NoCustom:    opts.NoCustom || opts.OnlyMatch,
		Urgent:      opts.Urgent,
		Active:      opts.Active,
	}
}

var windowFieldNames = map[string][]modes.WindowField{
	"title":   {modes.WindowFieldTitle},
	"class":   {modes.WindowFieldClass},
	"name":    {modes.WindowFieldName},
	"role":    {modes.WindowFieldRole},
	"desktop": {modes.WindowFieldDesktop},
	"all":     {modes.WindowFieldTitle, modes.WindowFieldClass, modes.WindowFieldName, modes.WindowFieldRole, modes.WindowFieldDesktop},
}

func windowConfig(opts launcheripc.SessionOptions, l config.LauncherConfig, currentOnly bool) modes.WindowConfig {
	var fields []modes.WindowField
	if opts.WindowMatchFields != nil {
		for _, raw := range opts.WindowMatchFields {
			fields = append(fields, windowFieldNames[raw]...)
		}
	} else {
		for _, f := range l.Window.MatchFields {
			fields = append(fields, windowFieldNames[string(f)]...)
		}
	}
	return modes.WindowConfig{
		Format:             deref(opts.WindowFormat, l.Window.Format),
		MatchFields:        fields,
		HideActive:         deref(opts.HideActiveWindow, l.Window.HideActive),
		CloseOnDelete:      l.Window.CloseOnDelete,
		WindowCommand:      deref(opts.WindowCommand, ""),
		CurrentDesktopOnly: currentOnly,
	}
}

func sshConfig(opts launcheripc.SessionOptions, l config.LauncherConfig) modes.SSHConfig {
	return modes.SSHConfig{
		Client:          deref(opts.SSHClient, l.SSH.Client),
		Command:         deref(opts.SSHCommand, l.SSH.Command),
		ParseHosts:      deref(opts.ParseHosts, l.SSH.ParseHosts),
		ParseKnownHosts: deref(opts.ParseKnownHosts, l.SSH.ParseKnownHosts),
		Terminal:        deref(opts.Terminal, l.Terminal),
		MaxHistory:      l.History.MaxSize,
	}
}

var fileSorts = map[config.LauncherFileSort]modes.FileSort{
	config.FileSortName:  modes.FileSortName,
	config.FileSortMtime: modes.FileSortMtime,
	config.FileSortAtime: modes.FileSortAtime,
	config.FileSortCtime: modes.FileSortCtime,
}

func fileBrowserConfig(l config.LauncherConfig, recursive bool) modes.FileBrowserConfig {
	return modes.FileBrowserConfig{
		Directory:        l.Filebrowser.Directory,
		Sorting:          fileSorts[l.Filebrowser.SortingMethod],
		DirectoriesFirst: l.Filebrowser.DirectoriesFirst,
		ShowHidden:       l.Filebrowser.ShowHidden,
		Command:          l.Filebrowser.Command,
		Terminal:         l.Terminal,
		Recursive:        recursive,
	}
}

var drunFieldNames = map[string]modes.DrunField{
	"name":       modes.DrunFieldName,
	"generic":    modes.DrunFieldGeneric,
	"exec":       modes.DrunFieldExec,
	"categories": modes.DrunFieldCategories,
	"comment":    modes.DrunFieldComment,
	"keywords":   modes.DrunFieldKeywords,
}

func drunConfig(opts launcheripc.SessionOptions, l config.LauncherConfig) modes.DrunConfig {
	var fields []modes.DrunField
	switch {
	case opts.DrunMatchFields != nil && slices.Contains(opts.DrunMatchFields, "all"):
		fields = modes.AllDrunFields
	case opts.DrunMatchFields != nil:
		for _, raw := range opts.DrunMatchFields {
			if f, ok := drunFieldNames[raw]; ok {
				fields = append(fields, f)
			}
		}
	default:
		for _, f := range l.Drun.MatchFields {
			fields = append(fields, drunFieldNames[string(f)])
		}
	}
	categories := l.Drun.Categories
	if opts.DrunCategories != nil {
		categories = opts.DrunCategories
	}
	exclude := l.Drun.ExcludeCategories
	if opts.DrunExcludeCategories != nil {
		exclude = opts.DrunExcludeCategories
	}
	return modes.DrunConfig{
		Categories:        categories,
		ExcludeCategories: exclude,
		MatchFields:       fields,
		DisplayFormat:     deref(opts.DrunDisplayFormat, l.Drun.DisplayFormat),
		ShowActions:       deref(opts.DrunShowActions, l.Drun.ShowActions),
		URLLauncher:       deref(opts.DrunURLLauncher, l.Drun.URLLauncher),
		Terminal:          deref(opts.Terminal, l.Terminal),
		MaxHistory:        l.History.MaxSize,
		FallbackIcon:      deref(opts.ApplicationFallbackIcon, ""),
		IgnoredPrefixes:   opts.IgnoredPrefixes,
	}
}

func runConfig(opts launcheripc.SessionOptions, l config.LauncherConfig) modes.RunConfig {
	return modes.RunConfig{
		RunCommand:      deref(opts.RunCommand, l.Run.RunCommand),
		ShellCommand:    deref(opts.RunShellCommand, l.Run.ShellCommand),
		ListCommand:     deref(opts.RunListCommand, l.Run.ListCommand),
		Terminal:        deref(opts.Terminal, l.Terminal),
		MaxHistory:      l.History.MaxSize,
		IgnoredPrefixes: opts.IgnoredPrefixes,
	}
}

// matcherOptions merges the matching flags over [launcher]. An unknown
// -matching is normal, an unknown -sorting-method levenshtein.
func matcherOptions(opts launcheripc.SessionOptions, l config.LauncherConfig) launchermatch.Options {
	o := launchermatch.OptionsOf(l)
	if opts.Matching != nil {
		o.Method = launchermatch.MethodOf(config.LauncherMatching(*opts.Matching))
	}
	switch {
	case deref(opts.CaseInsensitive, false):
		o.Case = launchermatch.CaseModeInsensitive
	case deref(opts.CaseSmart, false):
		o.Case = launchermatch.CaseModeSmart
	case deref(opts.CaseSensitive, false):
		o.Case = launchermatch.CaseModeSensitive
	}
	if opts.SortingMethod != nil {
		o.SortMethod = launchermatch.SortMethodOf(*opts.SortingMethod)
	}
	if opts.NegateChar != nil {
		o.NegationChar = launchermatch.NegationCharOf(*opts.NegateChar, o.NegationChar)
	}
	o.Tokenize = deref(opts.Tokenize, o.Tokenize)
	o.Normalize = deref(opts.NormalizeMatch, o.Normalize)
	o.Sort = deref(opts.Sort, o.Sort)
	return o
}

func hooksOf(opts launcheripc.SessionOptions) engine.Hooks {
	return engine.Hooks{
		SelectionChanged: deref(opts.OnSelectionChanged, ""),
		EntryAccepted:    deref(opts.OnEntryAccepted, ""),
		ModeChanged:      deref(opts.OnModeChanged, ""),
		MenuCanceled:     deref(opts.OnMenuCanceled, ""),
		MenuError:        deref(opts.OnMenuError, ""),
	}
}

// previewCmd is -preview-cmd, else the configured one; blank is none.
func previewCmd(opts launcheripc.SessionOptions, l config.LauncherConfig) string {
	cmd := deref(opts.PreviewCmd, l.PreviewCmd)
	if strings.TrimSpace(cmd) == "" {
		return ""
	}
	return cmd
}

// stylePreset is the CSS of the -style preset, "" without one. A preset
// name that does not exist is a typo in a bind, logged rather than
// silently rendering the default look.
func stylePreset(opts launcheripc.SessionOptions, l config.LauncherConfig) string {
	if opts.Style == nil {
		return ""
	}
	preset, ok := l.Styles[*opts.Style]
	if !ok {
		log.Printf("launcher: no such [launcher.styles] preset %q", *opts.Style)
	}
	return preset
}
