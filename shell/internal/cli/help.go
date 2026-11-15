package cli

import (
	"strings"
)

// Help layout constants. Help is never wrapped to the terminal, so the
// width is always 100 and only the next-line heuristic consults it.
const (
	tab            = "  "
	tabWidth       = 2
	nextLineIndent = "        "
	shortSize      = 4
	termWidth      = 100
)

// String is the arg's unstyled usage form, e.g.
// "--icon <ICON>", "<ID>", "[MINUTES]".
func (a *arg) String() string { return strip(a.stylized(nil)) }

// stylized is the flag name and its value suffix.
// required overrides the arg's own requiredness for the brackets.
func (a *arg) stylized(required *bool) string {
	var b strings.Builder
	switch {
	case a.Long != "":
		b.WriteString(styleLiteral.wrap("--" + a.Long))
	case a.Short != 0:
		b.WriteString(styleLiteral.wrap("-" + string(a.Short)))
	}
	b.WriteString(a.suffix(required))
	return b.String()
}

// suffix is the styled value placeholder after a flag name.
func (a *arg) suffix(required *bool) string {
	var b strings.Builder
	if a.takesValue() && !a.positional() {
		b.WriteString(stylePlaceholder.wrap(" "))
	}
	if a.takesValue() {
		req := a.Required
		if required != nil {
			req = *required
		}
		b.WriteString(stylePlaceholder.wrap(a.renderValue(req)))
	}
	return b.String()
}

// renderValue is the value placeholder of a single-name arg.
func (a *arg) renderValue(required bool) string {
	name := a.valueName()
	var v string
	if a.positional() && !required {
		v = "[" + name + "]"
	} else {
		v = "<" + name + ">"
	}
	if a.multiple() {
		v += "..."
	}
	return v
}

// usage is the usage line without its title. With used args it has
// no [OPTIONS] tag and shows the used args as required.
func usage(c *cmd, used []*arg) string {
	var b strings.Builder
	if len(used) == 0 {
		c.writeArgUsage(&b, nil)
		if len(c.subs) > 0 {
			if c.subRequired {
				b.WriteString(stylePlaceholder.wrap("<COMMAND>"))
			} else {
				b.WriteString(stylePlaceholder.wrap("[COMMAND]"))
			}
		}
	} else {
		c.writeArgUsage(&b, used)
		if c.subRequired {
			b.WriteString(stylePlaceholder.wrap("<COMMAND>"))
		}
	}
	return strings.TrimRight(b.String(), " \n")
}

// usageWithTitle is the usage line with its "Usage:" title.
func usageWithTitle(c *cmd, used []*arg) string {
	return styleUsage.wrap("Usage:") + " " + usage(c, used)
}

// writeArgUsage writes the args part of a usage line, required args
// included.
func (c *cmd) writeArgUsage(b *strings.Builder, used []*arg) {
	b.WriteString(styleLiteral.wrap(c.bin))
	b.WriteString(" ")
	if len(used) == 0 && c.needsOptionsTag() {
		b.WriteString(stylePlaceholder.wrap("[OPTIONS]"))
		b.WriteString(" ")
	}
	yes, no := true, false
	var opts []string
	seen := map[string]bool{}
	positionals := map[int]string{}
	for _, a := range append(c.requiredArgs(), used...) {
		s := a.stylized(&yes)
		if a.positional() {
			positionals[a.index] = s
		} else if !seen[s] {
			seen[s] = true
			opts = append(opts, s)
		}
	}
	for _, a := range c.args {
		if a.positional() {
			if _, ok := positionals[a.index]; !ok {
				positionals[a.index] = a.stylized(&no)
			}
		}
	}
	for _, s := range opts {
		b.WriteString(s + " ")
	}
	for i := 1; i <= len(positionals); i++ {
		b.WriteString(positionals[i] + " ")
	}
}

func (c *cmd) requiredArgs() []*arg {
	var req []*arg
	for _, a := range c.args {
		if a.Required {
			req = append(req, a)
		}
	}
	return req
}

// needsOptionsTag reports whether there is any optional named arg
// other than help and version.
func (c *cmd) needsOptionsTag() bool {
	for _, a := range c.args {
		if a.positional() || a.action == actionHelp || a.action == actionVersion || a.Required {
			continue
		}
		return true
	}
	return false
}

// renderHelp is the help template, then the leading blank line and
// trailing whitespace trimmed and a single newline restored.
func renderHelp(c *cmd, useLong bool) string {
	useLong = useLong && c.longHelpExists
	h := helpWriter{c: c, useLong: useLong}
	var b strings.Builder
	about := c.about
	if useLong && c.longAbout != "" {
		about = c.longAbout
	}
	if about != "" {
		b.WriteString(about + "\n")
	}
	b.WriteString("\n")
	b.WriteString(styleUsage.wrap("Usage:") + " " + usage(c, nil))
	if len(c.args) > 0 || len(c.subs) > 0 {
		b.WriteString("\n\n")
		h.writeAllArgs(&b)
	}
	if useLong && c.afterLongHelp != "" {
		b.WriteString("\n\n" + c.afterLongHelp)
	}
	out := b.String()
	if i := strings.IndexByte(out, '\n'); i >= 0 && strings.TrimSpace(out[:i]) == "" {
		out = out[i+1:]
	}
	return strings.TrimRight(out, " \t\n") + "\n"
}

type helpWriter struct {
	c       *cmd
	useLong bool
}

func (h helpWriter) writeAllArgs(b *strings.Builder) {
	var pos, named []*arg
	for _, a := range h.c.args {
		if a.positional() {
			pos = append(pos, a)
		} else {
			named = append(named, a)
		}
	}
	first := true
	section := func(title string) {
		if !first {
			b.WriteString("\n\n")
		}
		first = false
		b.WriteString(styleHeader.wrap(title+":") + "\n")
	}
	if len(h.c.subs) > 0 {
		section("Commands")
		h.writeSubcommands(b)
	}
	if len(pos) > 0 {
		section("Arguments")
		h.writeArgs(b, pos)
	}
	if len(named) > 0 {
		section("Options")
		h.writeArgs(b, named)
	}
}

// writeArgs writes an args section. Named args keep declaration
// order with help and version last; positionals are in index order, which is declaration order too.
func (h helpWriter) writeArgs(b *strings.Builder, args []*arg) {
	longest := 2
	for _, a := range args {
		w := displayWidth(a.String())
		if a.Long != "" {
			w += shortSize
		}
		longest = max(longest, w)
	}
	nextLine := false
	for _, a := range args {
		if h.argNextLineHelp(a, longest) {
			nextLine = true
		}
	}
	for i, a := range args {
		if i != 0 {
			b.WriteString("\n")
			if nextLine && h.useLong {
				b.WriteString("\n")
			}
		}
		h.writeArg(b, a, nextLine, longest)
	}
}

func (h helpWriter) argNextLineHelp(a *arg, longest int) bool {
	if h.useLong {
		return true
	}
	return forceNextLine(displayWidth(a.help(false))+displayWidth(h.specVals(a)), longest)
}

// forceNextLine reports whether the help column is too narrow to hold
// the text.
func forceNextLine(helpWidth, longest int) bool {
	taken := longest + tabWidth*2
	return termWidth >= taken && float32(taken)/float32(termWidth) > 0.40 && helpWidth > termWidth-taken
}

func (a *arg) help(useLong bool) string {
	if useLong && a.LongHelp != "" {
		return a.LongHelp
	}
	if a.Help != "" {
		return a.Help
	}
	return a.LongHelp
}

func (h helpWriter) writeArg(b *strings.Builder, a *arg, nextLine bool, longest int) {
	b.WriteString(tab)
	if a.Short != 0 {
		b.WriteString(styleLiteral.wrap("-" + string(a.Short)))
	} else if a.Long != "" {
		b.WriteString("    ")
	}
	if a.Long != "" {
		if a.Short != 0 {
			b.WriteString(", ")
		}
		b.WriteString(styleLiteral.wrap("--" + a.Long))
	}
	b.WriteString(a.suffix(nil))
	if !h.useLong && !nextLine {
		selfLen := displayWidth(a.String())
		padding := tabWidth
		if !a.positional() {
			selfLen += shortSize
			if a.Long == "" {
				padding += 4
			}
		}
		b.WriteString(strings.Repeat(" ", longest+padding-selfLen))
	}
	h.writeHelp(b, a, a.help(h.useLong), h.specVals(a), nextLine, longest)
}

// writeHelp writes one arg's help: the text, the possible-values block
// in long help, and the [default: ...]/[possible values: ...] specs.
func (h helpWriter) writeHelp(b *strings.Builder, a *arg, about, specVals string, nextLine bool, longest int) {
	if nextLine {
		b.WriteString("\n" + tab + nextLineIndent)
	}
	spaces := longest + tabWidth*2
	if nextLine {
		spaces = len(tab) + len(nextLineIndent)
	}
	trailing := strings.Repeat(" ", spaces)
	help := about
	helpEmpty := help == ""
	nextLineSpecs := h.useLong && a != nil
	if specVals != "" && !nextLineSpecs {
		if !helpEmpty {
			help += " "
		}
		help += specVals
		helpEmpty = help == ""
	}
	b.WriteString(strings.ReplaceAll(help, "\n", "\n"+trailing))

	hasPVs := false
	if a != nil && h.useLongPV(a) {
		pvs := a.possibleValues()
		hasPVs = true
		longestPV := 0
		for _, pv := range pvs {
			longestPV = max(longestPV, displayWidth(pv.Name))
		}
		pvSpaces := strings.Repeat(" ", spaces+tabWidth-2)
		pvTrailing := strings.Repeat(" ", spaces+tabWidth)
		if !helpEmpty {
			b.WriteString("\n\n" + pvSpaces)
		}
		b.WriteString("Possible values:")
		for _, pv := range pvs {
			descr := styleLiteral.wrap(pv.Name)
			if pv.Help != "" {
				descr += ": " + strings.Repeat(" ", longestPV-displayWidth(pv.Name)) + pv.Help
			}
			b.WriteString("\n" + pvSpaces + "- " + strings.ReplaceAll(descr, "\n", "\n"+pvTrailing))
		}
	}
	if specVals != "" && nextLineSpecs {
		spec := specVals
		if !helpEmpty || hasPVs {
			spec = "\n\n" + spec
		}
		b.WriteString(strings.ReplaceAll(spec, "\n", "\n"+trailing))
	}
}

func (h helpWriter) useLongPV(a *arg) bool {
	return h.useLong && pvsHaveHelp(a.possibleValues())
}

// specVals renders the [default: ...] and [possible values: ...] specs.
func (h helpWriter) specVals(a *arg) string {
	var specs []string
	if a.takesValue() && len(a.Defaults) > 0 {
		dvs := make([]string, len(a.Defaults))
		for i, d := range a.Defaults {
			dvs[i] = quoteIfSpaced(d)
		}
		specs = append(specs, "[default: "+strings.Join(dvs, " ")+"]")
	}
	if pvs := a.possibleValues(); len(pvs) > 0 && !h.useLongPV(a) {
		names := make([]string, len(pvs))
		for i, pv := range pvs {
			names[i] = quoteIfSpaced(pv.Name)
		}
		specs = append(specs, "[possible values: "+strings.Join(names, ", ")+"]")
	}
	sep := " "
	if h.useLong {
		sep = "\n"
	}
	return strings.Join(specs, sep)
}

// writeSubcommands writes the Commands section.
func (h helpWriter) writeSubcommands(b *strings.Builder) {
	longest := 2
	for _, s := range h.c.subs {
		longest = max(longest, displayWidth(s.name))
	}
	nextLine := false
	for _, s := range h.c.subs {
		if forceNextLine(displayWidth(s.about), longest) {
			nextLine = true
		}
	}
	for i, s := range h.c.subs {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(tab + styleLiteral.wrap(s.name))
		if !nextLine {
			b.WriteString(strings.Repeat(" ", longest+tabWidth-displayWidth(s.name)))
		}
		h.writeHelp(b, nil, s.about, "", nextLine, longest)
	}
}
