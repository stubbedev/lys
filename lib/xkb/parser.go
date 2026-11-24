package xkb

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/stubbedev/memory-go/heapslab"
)

type fileType uint8

const (
	fileKeycodes fileType = iota
	fileTypes
	fileCompat
	fileSymbols
	fileGeometry
	fileKeymap
	fileRules
)

var includeDirs = [...]string{
	fileKeycodes: "keycodes",
	fileTypes:    "types",
	fileCompat:   "compat",
	fileSymbols:  "symbols",
	fileGeometry: "geometry",
	fileKeymap:   "keymap",
	fileRules:    "rules",
}

// dir is the directory of an include path that holds files of the type,
// "" for a type that has none.
func (t fileType) dir() string {
	if int(t) >= len(includeDirs) {
		return ""
	}
	return includeDirs[t]
}

type mergeMode uint8

const (
	mergeDefault mergeMode = iota
	mergeAugment
	mergeOverride
	mergeReplace
)

type mapFlags uint8

const mapIsDefault mapFlags = 1

type xkbFile struct {
	typ      fileType
	name     string
	flags    mapFlags
	decls    []decl
	sections []*xkbFile
}

type decl interface{ isDecl() }

type varDecl struct {
	merge mergeMode
	name  expr
	value expr
}

type includeDecl struct {
	stmts []includeStmt
	text  string
}

type includeStmt struct {
	merge    mergeMode
	file     string
	mapName  string
	modifier string
}

type keycodeDecl struct {
	merge mergeMode
	name  string
	value int64
}

type aliasDecl struct {
	merge       mergeMode
	alias, real string
}

type vmodDecl struct {
	merge mergeMode
	name  string
	value expr
}

type interpDecl struct {
	merge mergeMode
	sym   Keysym
	match expr
	body  []varDecl
}

type typeDecl struct {
	merge mergeMode
	name  string
	body  []varDecl
}

type symbolsDecl struct {
	merge mergeMode
	name  string
	body  []varDecl
}

type modMapDecl struct {
	merge    mergeMode
	modifier string
	keys     []expr
}

type groupCompatDecl struct{}

type ledMapDecl struct {
	merge mergeMode
	name  string
	body  []varDecl
}

type ledNameDecl struct {
	merge mergeMode
	index int64
	name  expr
}

func (varDecl) isDecl()         {}
func (includeDecl) isDecl()     {}
func (*keycodeDecl) isDecl()    {}
func (aliasDecl) isDecl()       {}
func (vmodDecl) isDecl()        {}
func (*interpDecl) isDecl()     {}
func (typeDecl) isDecl()        {}
func (*symbolsDecl) isDecl()    {}
func (modMapDecl) isDecl()      {}
func (groupCompatDecl) isDecl() {}
func (ledMapDecl) isDecl()      {}
func (ledNameDecl) isDecl()     {}

type expr any

type (
	exprString  struct{ value string }
	exprInt     struct{ value int64 }
	exprFloat   struct{}
	exprBool    struct{ value bool }
	exprKeyName struct{ name string }
	exprKeysym  struct{ sym Keysym }
	exprIdent   struct{ name string }
	exprField   struct{ elem, field string }
	exprArray   struct {
		elem, field string
		index       expr
	}
	exprAction struct {
		name string
		args []expr
	}
	exprActionList struct{ actions []exprAction }
	exprKeysymList struct{ syms []Keysym }
	exprEmptyList  struct{}
	exprSymsLevels struct{ levels []exprKeysymList }
	exprActsLevels struct{ levels []exprActionList }
	exprBinary     struct {
		op          tokenKind
		left, right expr
	}
	exprAssign struct{ left, right expr }
	exprUnary  struct {
		op    tokenKind
		child expr
	}
)

type parser struct {
	declScratch  [1]decl
	levelScratch []levelItem
	varScratch   []varDecl
	argScratch   []expr
	ast          *astArenas
	ctx          *Context
	exprDepth    int
	lex          *lexer
	tok          token
	next         token
}

func newParser(c *Context, ast *astArenas, name, src string) (*parser, error) {
	src = strings.TrimPrefix(src, "\xef\xbb\xbf")
	if len(src) > 0 && (src[0] >= 0x80 || src[0] == 0) && !strings.HasPrefix(src, "\u200e") && !strings.HasPrefix(src, "\u200f") {
		return nil, fmt.Errorf("%s: the file encoding is not backward compatible with ASCII", name)
	}
	p := &parser{ctx: c, ast: ast, lex: newLexer(c, name, src)}
	var err error
	if p.tok, err = p.lex.next(); err != nil {
		return nil, err
	}
	if p.next, err = p.lex.next(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *parser) advance() error {
	p.tok = p.next
	var err error
	p.next, err = p.lex.next()
	return err
}

func (p *parser) errorf(format string, args ...any) error {
	return fmt.Errorf("%s:%d: %s", p.lex.name, p.tok.line, fmt.Sprintf(format, args...))
}

func (p *parser) expect(kind tokenKind, what string) (token, error) {
	if p.tok.kind != kind {
		return token{}, p.errorf("syntax error: expected %s", what)
	}
	t := p.tok
	if err := p.advance(); err != nil {
		return token{}, err
	}
	return t, nil
}

func (p *parser) acceptComma() (bool, error) {
	if p.tok.kind != tokComma {
		return false, nil
	}
	return true, p.advance()
}

type parsedFile struct {
	ctx      *Context
	name     string
	parser   *parser
	err      error
	first    *xkbFile
	def      *xkbFile
	maps     map[string]*xkbFile
	finished bool
}

func newParsedFile(c *Context, ast *astArenas, name, src string) *parsedFile {
	pf := &parsedFile{ctx: c, name: name, maps: map[string]*xkbFile{}}
	pf.parser, pf.err = newParser(c, ast, name, src)
	if pf.err != nil {
		c.log(LogError, 0, invalidSyntaxCode, "%v\n", pf.err)
	}
	return pf
}

func (pf *parsedFile) step() bool {
	if pf.err != nil || pf.finished {
		return false
	}
	if pf.parser.tok.kind == tokEOF {
		pf.finished = true
		return false
	}
	f, err := pf.parser.parseMap()
	if err != nil {
		pf.ctx.log(LogError, 0, invalidSyntaxCode, "%v\n", err)
		pf.err = err
		return false
	}
	if pf.first == nil {
		pf.first = f
	}
	if pf.def == nil && f.flags&mapIsDefault != 0 {
		pf.def = f
	}
	if _, seen := pf.maps[f.name]; !seen {
		pf.maps[f.name] = f
	}
	return true
}

func (pf *parsedFile) named(mapName string) (*xkbFile, error) {
	for {
		if f, ok := pf.maps[mapName]; ok {
			return f, nil
		}
		if !pf.step() {
			break
		}
	}
	if pf.err != nil {
		return nil, pf.err
	}
	return nil, fmt.Errorf("%s: no map named %q", pf.name, mapName)
}

func (pf *parsedFile) defaultMap() (*xkbFile, error) {
	for pf.def == nil {
		if !pf.step() {
			break
		}
	}
	if pf.err != nil {
		return nil, pf.err
	}
	if pf.def != nil {
		return pf.def, nil
	}
	if pf.first != nil {
		pf.ctx.verbose(verbosityDetailed, 433, "No map in include statement, but \"%s\" contains several; Using first defined map, \"%s\"\n", pf.name, mapDisplayName(pf.first))
		return pf.first, nil
	}
	return nil, fmt.Errorf("%s: no map", pf.name)
}

func (p *parser) parseFlags() (mapFlags, error) {
	var flags mapFlags
	for {
		switch p.tok.kind {
		case tokDefault:
			flags |= mapIsDefault
		case tokPartial, tokHidden, tokAlphanumericKeys, tokModifierKeys, tokKeypadKeys, tokFunctionKeys, tokAlternateGroup:
		default:
			return flags, nil
		}
		if err := p.advance(); err != nil {
			return 0, err
		}
	}
}

func (p *parser) parseMapName() (string, error) {
	if p.tok.kind != tokString {
		return "", nil
	}
	name := p.tok.text
	return name, p.advance()
}

func (p *parser) parseMap() (*xkbFile, error) {
	flags, err := p.parseFlags()
	if err != nil {
		return nil, err
	}
	switch p.tok.kind {
	case tokKeymap, tokSemantics, tokLayout:
		return p.parseComposite(flags)
	}
	return p.parseConfig(flags)
}

func (p *parser) parseComposite(flags mapFlags) (*xkbFile, error) {
	if err := p.advance(); err != nil {
		return nil, err
	}
	name, err := p.parseMapName()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokOBrace, "'{'"); err != nil {
		return nil, err
	}
	f := &xkbFile{typ: fileKeymap, name: escapeMapName(name), flags: flags}
	for p.tok.kind != tokCBrace {
		sectionFlags, err := p.parseFlags()
		if err != nil {
			return nil, err
		}
		section, err := p.parseConfig(sectionFlags)
		if err != nil {
			return nil, err
		}
		f.sections = append(f.sections, section)
	}
	if err := p.advance(); err != nil {
		return nil, err
	}
	if _, err := p.expect(tokSemi, "';'"); err != nil {
		return nil, err
	}
	return f, nil
}

func (p *parser) parseConfig(flags mapFlags) (*xkbFile, error) {
	var typ fileType
	switch p.tok.kind {
	case tokKeycodes:
		typ = fileKeycodes
	case tokTypes:
		typ = fileTypes
	case tokCompat:
		typ = fileCompat
	case tokSymbols:
		typ = fileSymbols
	case tokGeometry:
		typ = fileGeometry
	default:
		return nil, p.errorf("syntax error: expected a keymap section")
	}
	if err := p.advance(); err != nil {
		return nil, err
	}
	name, err := p.parseMapName()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokOBrace, "'{'"); err != nil {
		return nil, err
	}
	f := &xkbFile{typ: typ, name: escapeMapName(name), flags: flags}
	if typ == fileGeometry {
		p.ctx.verbose(verbosityBrief, warnGeometrySection, "Geometry sections are not supported; ignoring\n")
		if err := p.skipBlock(); err != nil {
			return nil, err
		}
	} else {
		if f.decls, err = p.parseDecls(); err != nil {
			return nil, err
		}
	}
	if _, err := p.expect(tokCBrace, "'}'"); err != nil {
		return nil, err
	}
	if _, err := p.expect(tokSemi, "';'"); err != nil {
		return nil, err
	}
	return f, nil
}

func (p *parser) skipBlock() error {
	for depth := 1; ; {
		switch p.tok.kind {
		case tokEOF:
			return p.errorf("syntax error: unterminated section")
		case tokOBrace:
			depth++
		case tokCBrace:
			depth--
			if depth == 0 {
				return nil
			}
		}
		if err := p.advance(); err != nil {
			return err
		}
	}
}

func (p *parser) parseDecls() ([]decl, error) {
	var decls []decl
	for p.tok.kind != tokCBrace {
		ds, err := p.parseDecl()
		if err != nil {
			return nil, err
		}
		decls = append(decls, ds...)
	}
	return decls, nil
}

func (p *parser) parseMergeMode() (mergeMode, bool, error) {
	var mode mergeMode
	switch p.tok.kind {
	case tokInclude, tokAlternate:
		mode = mergeDefault
	case tokAugment:
		mode = mergeAugment
	case tokOverride:
		mode = mergeOverride
	case tokReplace:
		mode = mergeReplace
	default:
		return mergeDefault, false, nil
	}
	return mode, true, p.advance()
}

func (p *parser) parseDecl() ([]decl, error) {
	merge, hasMerge, err := p.parseMergeMode()
	if err != nil {
		return nil, err
	}
	switch p.tok.kind {
	case tokString:
		if !hasMerge {
			return nil, p.errorf("syntax error: unexpected string")
		}
		text := p.tok.text
		if err := p.advance(); err != nil {
			return nil, err
		}
		inc, err := parseInclude(text, merge)
		if err != nil {
			return nil, p.errorf("%v", err)
		}
		if len(inc.stmts) == 0 {
			return nil, nil
		}
		return p.oneDecl(inc), nil
	case tokVirtualMods:
		return p.parseVMods(merge)
	case tokKeyName:
		return p.parseKeycode(merge)
	case tokAlias:
		return p.parseAlias(merge)
	case tokVirtual:
		return p.parseLedName(merge, true)
	case tokInterpret:
		if p.next.kind != tokDot && p.next.kind != tokEquals && p.next.kind != tokOBracket && p.next.kind != tokSemi {
			return p.parseInterpret(merge)
		}
	case tokType:
		if p.next.kind == tokString {
			name, body, err := p.parseNamedBody()
			if err != nil {
				return nil, err
			}
			return p.oneDecl(typeDecl{merge: merge, name: name, body: body}), nil
		}
	case tokKey:
		if p.next.kind == tokKeyName {
			return p.parseKeySymbols(merge)
		}
	case tokGroup:
		if p.next.kind == tokDigit || p.next.kind == tokInteger {
			return p.parseGroupCompat()
		}
	case tokModifierMap:
		if p.next.kind == tokIdent || p.next.kind == tokDefault {
			return p.parseModMap(merge)
		}
	case tokIndicator:
		switch p.next.kind {
		case tokString:
			name, body, err := p.parseNamedBody()
			if err != nil {
				return nil, err
			}
			return p.oneDecl(ledMapDecl{merge: merge, name: name, body: body}), nil
		case tokDigit, tokInteger:
			return p.parseLedName(merge, false)
		}
	}
	v, err := p.parseVarDecl()
	if err != nil {
		return nil, err
	}
	v.merge = merge
	return p.oneDecl(v), nil
}

func (p *parser) parseNamedBody() (string, []varDecl, error) {
	if err := p.advance(); err != nil {
		return "", nil, err
	}
	name, err := p.expect(tokString, "a string")
	if err != nil {
		return "", nil, err
	}
	body, err := p.parseVarBlock()
	return name.text, body, err
}

func (p *parser) parseVarBlock() ([]varDecl, error) {
	if _, err := p.expect(tokOBrace, "'{'"); err != nil {
		return nil, err
	}
	start := len(p.varScratch)
	defer func() { p.varScratch = popScratch(p.varScratch, start) }()
	for p.tok.kind != tokCBrace {
		v, err := p.parseVarDecl()
		if err != nil {
			return nil, err
		}
		p.varScratch = append(p.varScratch, v)
	}
	body := carveScratch(p.ast.bodies, p.varScratch[start:])
	if err := p.advance(); err != nil {
		return nil, err
	}
	_, err := p.expect(tokSemi, "';'")
	return body, err
}

func carveScratch[T any](a *heapslab.Arena[T], items []T) []T {
	if len(items) == 0 {
		return nil
	}
	out := a.AllocSlice(len(items))
	copy(out, items)
	return out
}

func popScratch[T any](scratch []T, start int) []T {
	clear(scratch[start:])
	return scratch[:start]
}

func (p *parser) parseVMods(merge mergeMode) ([]decl, error) {
	if err := p.advance(); err != nil {
		return nil, err
	}
	var decls []decl
	for {
		name, err := p.parseIdent()
		if err != nil {
			return nil, err
		}
		d := vmodDecl{merge: merge, name: name}
		if p.tok.kind == tokEquals {
			if err := p.advance(); err != nil {
				return nil, err
			}
			if d.value, err = p.parseExpr(0); err != nil {
				return nil, err
			}
		}
		decls = append(decls, d)
		more, err := p.acceptComma()
		if err != nil {
			return nil, err
		}
		if !more {
			break
		}
	}
	_, err := p.expect(tokSemi, "';'")
	return decls, err
}

func (p *parser) parseIdent() (string, error) {
	if p.tok.kind != tokIdent && p.tok.kind != tokDefault {
		return "", p.errorf("syntax error: expected an identifier")
	}
	text := p.tok.text
	return text, p.advance()
}

func (p *parser) parseKeycode(merge mergeMode) ([]decl, error) {
	name := p.tok.text
	if err := p.advance(); err != nil {
		return nil, err
	}
	if _, err := p.expect(tokEquals, "'='"); err != nil {
		return nil, err
	}
	if p.tok.kind != tokDigit && p.tok.kind != tokInteger {
		return nil, p.errorf("syntax error: expected a keycode")
	}
	value := p.tok.num
	if err := p.advance(); err != nil {
		return nil, err
	}
	_, err := p.expect(tokSemi, "';'")
	d := p.ast.keycodes.Alloc()
	*d = keycodeDecl{merge: merge, name: name, value: value}
	return p.oneDecl(d), err
}

func (p *parser) parseAlias(merge mergeMode) ([]decl, error) {
	if err := p.advance(); err != nil {
		return nil, err
	}
	alias, err := p.expect(tokKeyName, "a key name")
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokEquals, "'='"); err != nil {
		return nil, err
	}
	real, err := p.expect(tokKeyName, "a key name")
	if err != nil {
		return nil, err
	}
	_, err = p.expect(tokSemi, "';'")
	return p.oneDecl(aliasDecl{merge: merge, alias: alias.text, real: real.text}), err
}

func (p *parser) parseLedName(merge mergeMode, virtual bool) ([]decl, error) {
	if err := p.advance(); err != nil {
		return nil, err
	}
	if virtual {
		if _, err := p.expect(tokIndicator, "'indicator'"); err != nil {
			return nil, err
		}
	}
	if p.tok.kind != tokDigit && p.tok.kind != tokInteger {
		return nil, p.errorf("syntax error: expected an indicator index")
	}
	index := p.tok.num
	if err := p.advance(); err != nil {
		return nil, err
	}
	if _, err := p.expect(tokEquals, "'='"); err != nil {
		return nil, err
	}
	value, err := p.parseExpr(0)
	if err != nil {
		return nil, err
	}
	_, err = p.expect(tokSemi, "';'")
	return p.oneDecl(ledNameDecl{merge: merge, index: index, name: value}), err
}

func (p *parser) parseInterpret(merge mergeMode) ([]decl, error) {
	if err := p.advance(); err != nil {
		return nil, err
	}
	sym, err := p.parseKeysym()
	if err != nil {
		return nil, err
	}
	d := p.ast.interps.Alloc()
	*d = interpDecl{merge: merge, sym: sym}
	if p.tok.kind == tokPlus {
		if err := p.advance(); err != nil {
			return nil, err
		}
		if d.match, err = p.parseExpr(0); err != nil {
			return nil, err
		}
	}
	if d.body, err = p.parseVarBlock(); err != nil {
		return nil, err
	}
	return p.oneDecl(d), nil
}

func (p *parser) parseGroupCompat() ([]decl, error) {
	if err := p.advance(); err != nil {
		return nil, err
	}
	if err := p.advance(); err != nil {
		return nil, err
	}
	if _, err := p.expect(tokEquals, "'='"); err != nil {
		return nil, err
	}
	if _, err := p.parseExpr(0); err != nil {
		return nil, err
	}
	_, err := p.expect(tokSemi, "';'")
	return p.oneDecl(groupCompatDecl{}), err
}

func (p *parser) parseModMap(merge mergeMode) ([]decl, error) {
	if err := p.advance(); err != nil {
		return nil, err
	}
	modifier, err := p.parseIdent()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokOBrace, "'{'"); err != nil {
		return nil, err
	}
	d := modMapDecl{merge: merge, modifier: modifier}
	for {
		if p.tok.kind == tokKeyName {
			d.keys = append(d.keys, exprKeyName{name: p.tok.text})
			if err := p.advance(); err != nil {
				return nil, err
			}
		} else {
			sym, err := p.parseKeysym()
			if err != nil {
				return nil, err
			}
			d.keys = append(d.keys, exprKeysym{sym: sym})
		}
		more, err := p.acceptComma()
		if err != nil {
			return nil, err
		}
		if !more {
			break
		}
	}
	if _, err := p.expect(tokCBrace, "'}'"); err != nil {
		return nil, err
	}
	_, err = p.expect(tokSemi, "';'")
	return p.oneDecl(d), err
}

func (p *parser) parseKeySymbols(merge mergeMode) ([]decl, error) {
	if err := p.advance(); err != nil {
		return nil, err
	}
	name := p.tok.text
	if err := p.advance(); err != nil {
		return nil, err
	}
	if _, err := p.expect(tokOBrace, "'{'"); err != nil {
		return nil, err
	}
	d := p.ast.symbols.Alloc()
	*d = symbolsDecl{merge: merge, name: name, body: p.ast.bodies.AllocSlice(symbolsBody)[:0]}
	if p.tok.kind != tokCBrace {
		for {
			v, err := p.parseSymbolsVar()
			if err != nil {
				return nil, err
			}
			d.body = append(d.body, v)
			more, err := p.acceptComma()
			if err != nil {
				return nil, err
			}
			if !more {
				break
			}
		}
	}
	if _, err := p.expect(tokCBrace, "'}'"); err != nil {
		return nil, err
	}
	_, err := p.expect(tokSemi, "';'")
	return p.oneDecl(d), err
}

func (p *parser) parseSymbolsVar() (varDecl, error) {
	switch p.tok.kind {
	case tokOBracket:
		value, err := p.parseLevels()
		return varDecl{value: value}, err
	case tokExclam:
		if err := p.advance(); err != nil {
			return varDecl{}, err
		}
		name, err := p.parseIdent()
		return varDecl{name: p.ident(name), value: exprBool{value: false}}, err
	}
	lhs, err := p.parseLhs()
	if err != nil {
		return varDecl{}, err
	}
	if p.tok.kind != tokEquals {
		id, ok := lhs.(*exprIdent)
		if !ok {
			return varDecl{}, p.errorf("syntax error: expected '='")
		}
		return varDecl{name: id, value: exprBool{value: true}}, nil
	}
	if err := p.advance(); err != nil {
		return varDecl{}, err
	}
	if p.tok.kind == tokOBracket {
		value, err := p.parseLevels()
		return varDecl{name: lhs, value: value}, err
	}
	value, err := p.parseExpr(0)
	return varDecl{name: lhs, value: value}, err
}

func (p *parser) parseVarDecl() (varDecl, error) {
	if p.tok.kind == tokExclam {
		if err := p.advance(); err != nil {
			return varDecl{}, err
		}
		name, err := p.parseIdent()
		if err != nil {
			return varDecl{}, err
		}
		_, err = p.expect(tokSemi, "';'")
		return varDecl{name: p.ident(name), value: exprBool{value: false}}, err
	}
	lhs, err := p.parseLhs()
	if err != nil {
		return varDecl{}, err
	}
	if p.tok.kind == tokSemi {
		id, ok := lhs.(*exprIdent)
		if !ok {
			return varDecl{}, p.errorf("syntax error: expected '='")
		}
		return varDecl{name: id, value: exprBool{value: true}}, p.advance()
	}
	if _, err := p.expect(tokEquals, "'='"); err != nil {
		return varDecl{}, err
	}
	value, err := p.parseExpr(0)
	if err != nil {
		return varDecl{}, err
	}
	_, err = p.expect(tokSemi, "';'")
	return varDecl{name: lhs, value: value}, err
}

func isFieldSpec(kind tokenKind) bool {
	switch kind {
	case tokIdent, tokDefault, tokAction, tokInterpret, tokType, tokKey, tokGroup, tokModifierMap, tokIndicator, tokShape, tokRow, tokSection, tokText:
		return true
	}
	return false
}

func (p *parser) parseFieldSpec() (string, error) {
	if !isFieldSpec(p.tok.kind) {
		return "", p.errorf("syntax error: expected a field name")
	}
	text := p.tok.text
	return text, p.advance()
}

func (p *parser) parseLhs() (expr, error) {
	first, err := p.parseFieldSpec()
	if err != nil {
		return nil, err
	}
	return p.parseLhsRest(first)
}

func (p *parser) parseLhsRest(first string) (expr, error) {
	switch p.tok.kind {
	case tokDot:
		if err := p.advance(); err != nil {
			return nil, err
		}
		second, err := p.parseFieldSpec()
		if err != nil {
			return nil, err
		}
		if p.tok.kind != tokOBracket {
			return exprField{elem: first, field: second}, nil
		}
		index, err := p.parseIndex()
		return p.array(first, second, index), err
	case tokOBracket:
		index, err := p.parseIndex()
		return p.array("", first, index), err
	}
	return p.ident(first), nil
}

func (p *parser) ident(name string) *exprIdent {
	e := p.ast.idents.Alloc()
	e.name = name
	return e
}

func (p *parser) array(elem, field string, index expr) *exprArray {
	e := p.ast.arrays.Alloc()
	*e = exprArray{elem: elem, field: field, index: index}
	return e
}

func (p *parser) parseIndex() (expr, error) {
	if err := p.advance(); err != nil {
		return nil, err
	}
	index, err := p.parseExpr(0)
	if err != nil {
		return nil, err
	}
	_, err = p.expect(tokCBracket, "']'")
	return index, err
}

const (
	precAssign = 1
	precAdd    = 2
	precMul    = 3
)

func binaryPrec(kind tokenKind) int {
	switch kind {
	case tokPlus, tokMinus:
		return precAdd
	case tokTimes, tokDivide:
		return precMul
	}
	return 0
}

// maxExprDepth bounds how deep an expression nests, by parentheses, unary
// operators or operator chains: keymap text is untrusted, and the Go stack
// overflowing while parsing or evaluating one is fatal, not an error.
const maxExprDepth = 512

func (p *parser) enterExpr() error {
	if p.exprDepth >= maxExprDepth {
		return p.errorf("an expression nested more than %d levels deep", maxExprDepth)
	}
	p.exprDepth++
	return nil
}

func (p *parser) parseExpr(minPrec int) (expr, error) {
	if err := p.enterExpr(); err != nil {
		return nil, err
	}
	defer func() { p.exprDepth-- }()
	left, err := p.parseTerm()
	if err != nil {
		return nil, err
	}
	for links := 0; ; links++ {
		if links >= maxExprDepth {
			return nil, p.errorf("an operator chain longer than %d", maxExprDepth)
		}
		if p.tok.kind == tokEquals && minPrec <= precAssign {
			switch left.(type) {
			case *exprIdent, exprField, *exprArray:
			default:
				return nil, p.errorf("syntax error: cannot assign to this expression")
			}
			if err := p.advance(); err != nil {
				return nil, err
			}
			right, err := p.parseExpr(precAssign)
			if err != nil {
				return nil, err
			}
			assign := p.ast.assigns.Alloc()
			*assign = exprAssign{left: left, right: right}
			left = assign
			continue
		}
		prec := binaryPrec(p.tok.kind)
		if prec == 0 || prec < minPrec {
			return left, nil
		}
		op := p.tok.kind
		if err := p.advance(); err != nil {
			return nil, err
		}
		right, err := p.parseExpr(prec + 1)
		if err != nil {
			return nil, err
		}
		left = exprBinary{op: op, left: left, right: right}
	}
}

func (p *parser) parseTerm() (expr, error) {
	if err := p.enterExpr(); err != nil {
		return nil, err
	}
	defer func() { p.exprDepth-- }()
	switch p.tok.kind {
	case tokMinus, tokPlus, tokExclam, tokInvert:
		op := p.tok.kind
		if err := p.advance(); err != nil {
			return nil, err
		}
		child, err := p.parseTerm()
		return exprUnary{op: op, child: child}, err
	case tokOParen:
		if err := p.advance(); err != nil {
			return nil, err
		}
		inner, err := p.parseExpr(0)
		if err != nil {
			return nil, err
		}
		_, err = p.expect(tokCParen, "')'")
		return inner, err
	case tokOBrace:
		return p.parseActionBlock()
	case tokString:
		v := exprString{value: p.tok.text}
		return v, p.advance()
	case tokDigit, tokInteger:
		v := exprInt{value: p.tok.num}
		return v, p.advance()
	case tokFloat:
		return exprFloat{}, p.advance()
	case tokKeyName:
		v := exprKeyName{name: p.tok.text}
		return v, p.advance()
	}
	first, err := p.parseFieldSpec()
	if err != nil {
		return nil, err
	}
	if p.tok.kind == tokOParen {
		action, err := p.parseActionCall(first)
		if err != nil {
			return nil, err
		}
		boxed := p.ast.actions.Alloc()
		*boxed = action
		return boxed, nil
	}
	return p.parseLhsRest(first)
}

func (p *parser) parseActionCall(name string) (exprAction, error) {
	if err := p.advance(); err != nil {
		return exprAction{}, err
	}
	action := exprAction{name: name}
	if p.tok.kind != tokCParen {
		start := len(p.argScratch)
		defer func() { p.argScratch = popScratch(p.argScratch, start) }()
		for {
			arg, err := p.parseExpr(0)
			if err != nil {
				return exprAction{}, err
			}
			p.argScratch = append(p.argScratch, arg)
			more, err := p.acceptComma()
			if err != nil {
				return exprAction{}, err
			}
			if !more {
				break
			}
		}
		action.args = carveScratch(p.ast.args, p.argScratch[start:])
	}
	_, err := p.expect(tokCParen, "')'")
	return action, err
}

func (p *parser) parseActionBlock() (exprActionList, error) {
	if err := p.advance(); err != nil {
		return exprActionList{}, err
	}
	var list exprActionList
	if p.tok.kind != tokCBrace {
		for {
			first, err := p.parseFieldSpec()
			if err != nil {
				return exprActionList{}, err
			}
			if p.tok.kind != tokOParen {
				return exprActionList{}, p.errorf("syntax error: expected an action")
			}
			action, err := p.parseActionCall(first)
			if err != nil {
				return exprActionList{}, err
			}
			list.actions = append(list.actions, action)
			more, err := p.acceptComma()
			if err != nil {
				return exprActionList{}, err
			}
			if !more {
				break
			}
		}
	}
	_, err := p.expect(tokCBrace, "'}'")
	return list, err
}

type levelKind uint8

const (
	levelEmpty levelKind = iota
	levelSyms
	levelActs
)

type levelItem struct {
	kind levelKind
	syms exprKeysymList
	acts exprActionList
}

func (p *parser) parseLevels() (expr, error) {
	if err := p.advance(); err != nil {
		return nil, err
	}
	items := p.levelScratch[:0]
	defer func() { p.levelScratch = items[:0] }()
	kind := levelEmpty
	if p.tok.kind != tokCBracket {
		for {
			item, err := p.parseLevelItem()
			if err != nil {
				return nil, err
			}
			if item.kind != levelEmpty {
				if kind != levelEmpty && kind != item.kind {
					return nil, p.errorf("syntax error: a level list mixes keysyms and actions")
				}
				kind = item.kind
			}
			items = append(items, item)
			more, err := p.acceptComma()
			if err != nil {
				return nil, err
			}
			if !more {
				break
			}
		}
	}
	if _, err := p.expect(tokCBracket, "']'"); err != nil {
		return nil, err
	}
	switch kind {
	case levelSyms:
		levels := p.ast.symLists.AllocSlice(len(items))
		for i, item := range items {
			levels[i] = item.syms
		}
		e := p.ast.symsLevels.Alloc()
		e.levels = levels
		return e, nil
	case levelActs:
		levels := make([]exprActionList, len(items))
		for i, item := range items {
			levels[i] = item.acts
		}
		return exprActsLevels{levels: levels}, nil
	}
	return exprEmptyList{}, nil
}

func (p *parser) parseLevelItem() (levelItem, error) {
	switch p.tok.kind {
	case tokOBrace:
		return p.parseLevelBlock()
	case tokString:
		syms, err := stringKeysyms(p.tok.text)
		if err != nil {
			return levelItem{}, p.errorf("%v", err)
		}
		return levelItem{kind: levelSyms, syms: exprKeysymList{syms: syms}}, p.advance()
	}
	if isFieldSpec(p.tok.kind) && p.next.kind == tokOParen {
		name, err := p.parseFieldSpec()
		if err != nil {
			return levelItem{}, err
		}
		action, err := p.parseActionCall(name)
		if err != nil {
			return levelItem{}, err
		}
		actions := p.ast.actions.AllocSlice(1)
		actions[0] = action
		return levelItem{kind: levelActs, acts: exprActionList{actions: actions}}, nil
	}
	sym, err := p.parseKeysymLit()
	if err != nil {
		return levelItem{}, err
	}
	item := levelItem{kind: levelSyms}
	if sym != KeyNoSymbol {
		item.syms.syms = p.ast.syms.AllocSlice(1)
		item.syms.syms[0] = sym
	}
	return item, nil
}

func (p *parser) parseLevelBlock() (levelItem, error) {
	if err := p.advance(); err != nil {
		return levelItem{}, err
	}
	if p.tok.kind == tokCBrace {
		return levelItem{}, p.advance()
	}
	if isFieldSpec(p.tok.kind) && p.next.kind == tokOParen {
		var list exprActionList
		for {
			name, err := p.parseFieldSpec()
			if err != nil {
				return levelItem{}, err
			}
			if p.tok.kind != tokOParen {
				return levelItem{}, p.errorf("syntax error: expected an action")
			}
			action, err := p.parseActionCall(name)
			if err != nil {
				return levelItem{}, err
			}
			list.actions = append(list.actions, action)
			more, err := p.acceptComma()
			if err != nil {
				return levelItem{}, err
			}
			if !more {
				break
			}
		}
		_, err := p.expect(tokCBrace, "'}'")
		return levelItem{kind: levelActs, acts: list}, err
	}
	var list exprKeysymList
	for {
		if p.tok.kind == tokString {
			syms, err := stringKeysyms(p.tok.text)
			if err != nil {
				return levelItem{}, p.errorf("%v", err)
			}
			list.syms = append(list.syms, syms...)
			if err := p.advance(); err != nil {
				return levelItem{}, err
			}
		} else {
			sym, err := p.parseKeysymLit()
			if err != nil {
				return levelItem{}, err
			}
			if sym != KeyNoSymbol {
				list.syms = append(list.syms, sym)
			}
		}
		more, err := p.acceptComma()
		if err != nil {
			return levelItem{}, err
		}
		if !more {
			break
		}
	}
	_, err := p.expect(tokCBrace, "'}'")
	return levelItem{kind: levelSyms, syms: list}, err
}

func stringKeysyms(s string) ([]Keysym, error) {
	syms := make([]Keysym, 0, len(s))
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && n <= 1 {
			return nil, fmt.Errorf("cannot convert string %q to keysyms: invalid UTF-8 at byte %d", s, i)
		}
		sym := UTF32ToKeysym(r)
		if sym == KeyNoSymbol {
			return nil, fmt.Errorf("cannot convert string %q to keysyms: U+%04X has no keysym", s, r)
		}
		syms = append(syms, sym)
		i += n
	}
	return syms, nil
}

func (p *parser) parseKeysym() (Keysym, error) {
	if p.tok.kind != tokString {
		return p.parseKeysymLit()
	}
	syms, err := stringKeysyms(p.tok.text)
	if err != nil {
		return 0, p.errorf("%v", err)
	}
	if len(syms) != 1 {
		return 0, p.errorf("expected a single code point, got %q", p.tok.text)
	}
	return syms[0], p.advance()
}

func (p *parser) parseKeysymLit() (Keysym, error) {
	switch p.tok.kind {
	case tokIdent:
		sym, known := resolveKeysymName(p.tok.text)
		p.warnKeysymName(p.tok.text, known)
		return sym, p.advance()
	case tokSection:
		return KeySection, p.advance()
	case tokDigit:
		return p.numericKeysym(int64(Key0) + p.tok.num), p.advance()
	case tokInteger:
		return p.numericKeysym(p.tok.num), p.advance()
	}
	return 0, p.errorf("syntax error: expected a keysym")
}

func (p *parser) numericKeysym(n int64) Keysym {
	if n < 0 || n > keysymMax {
		p.warnf(errInvalidNumericKeysym, "unrecognized keysym \"%#06x\" (%d)", n, n)
		return KeyNoSymbol
	}
	sym := Keysym(n)
	if p.ctx.LogVerbosity() >= minVerbosityDeprecated && checkDeprecatedValue(sym) {
		p.warnf(warnDeprecatedKeysym, "deprecated keysym \"%#06x\".", uint32(sym))
	}
	return sym
}

func resolveKeysymName(name string) (Keysym, bool) {
	switch {
	case asciiEqualFold(name, "any"), asciiEqualFold(name, "nosymbol"):
		return KeyNoSymbol, true
	case asciiEqualFold(name, "none"), asciiEqualFold(name, "voidsymbol"):
		return KeyVoidSymbol, true
	}
	sym := KeysymFromName(name, KeysymNoFlags)
	return sym, sym != KeyNoSymbol
}

const (
	warnUnrecognizedKeysym  = 107
	errInvalidNumericKeysym = 82
	warnDeprecatedKeysym    = 301
	warnDeprecatedName      = 302
	minVerbosityDeprecated  = 2
)

func (p *parser) warnf(code int, format string, args ...any) {
	p.ctx.warn(code, "%s:%d:%d: "+format+"\n", append([]any{p.lex.name, p.tok.line, 0}, args...)...)
}

func (p *parser) warnKeysymName(name string, known bool) {
	if !known {
		p.warnf(warnUnrecognizedKeysym, "unrecognized keysym \"%s\"", name)
		return
	}
	if p.ctx.LogVerbosity() < minVerbosityDeprecated {
		return
	}
	if deprecated, reference := checkDeprecatedName(name); deprecated {
		if reference == "" {
			p.warnf(warnDeprecatedKeysym, "deprecated keysym \"%s\".", name)
		} else {
			p.warnf(warnDeprecatedName, "deprecated keysym name \"%s\"; please use \"%s\" instead.", name, reference)
		}
	}
}

func parseInclude(text string, merge mergeMode) (includeDecl, error) {
	inc := includeDecl{text: text}
	rest := text
	for rest != "" {
		stmt, next, op, err := parseIncludeMap(rest)
		if err != nil {
			return includeDecl{}, fmt.Errorf("illegal include statement %q: %w", text, err)
		}
		rest = next
		if stmt.file == "" {
			continue
		}
		stmt.merge = merge
		inc.stmts = append(inc.stmts, stmt)
		switch op {
		case '|':
			merge = mergeAugment
		case '^':
			merge = mergeReplace
		default:
			merge = mergeOverride
		}
	}
	return inc, nil
}

func parseIncludeMap(s string) (includeStmt, string, byte, error) {
	var stmt includeStmt
	op := byte(0)
	cur, next := s, ""
	if i := strings.IndexAny(s, "+|^"); i >= 0 {
		cur, next, op = s[:i], s[i+1:], s[i]
	}
	if i := strings.IndexByte(cur, ':'); i >= 0 {
		stmt.modifier = cur[i+1:]
		cur = cur[:i]
	}
	if i := strings.IndexByte(cur, '('); i >= 0 {
		if i == 0 {
			return stmt, "", 0, fmt.Errorf("a map name with no file in %q", cur)
		}
		if !strings.HasSuffix(cur, ")") || strings.IndexByte(cur[i:], ')') != len(cur)-i-1 {
			return stmt, "", 0, fmt.Errorf("a malformed map name in %q", cur)
		}
		stmt.mapName = cur[i+1 : len(cur)-1]
		stmt.file = cur[:i]
	} else {
		stmt.file = cur
	}
	return stmt, next, op, nil
}

// astArenas carve a session's syntax tree from shared chunks, so the
// thousands of keys, identifiers and one-keysym levels a keymap parses
// to are a few allocations rather than one each. The tree lives as long
// as the session that parsed it.
type astArenas struct {
	keycodes   *heapslab.Arena[keycodeDecl]
	symbols    *heapslab.Arena[symbolsDecl]
	bodies     *heapslab.Arena[varDecl]
	idents     *heapslab.Arena[exprIdent]
	arrays     *heapslab.Arena[exprArray]
	syms       *heapslab.Arena[Keysym]
	symLists   *heapslab.Arena[exprKeysymList]
	symsLevels *heapslab.Arena[exprSymsLevels]
	args       *heapslab.Arena[expr]
	actions    *heapslab.Arena[exprAction]
	assigns    *heapslab.Arena[exprAssign]
	interps    *heapslab.Arena[interpDecl]
}

const (
	astChunk    = 256
	symbolsBody = 2
)

func newASTArenas() (*astArenas, error) {
	var a astArenas
	var errs [12]error
	a.keycodes, errs[0] = heapslab.NewArena[keycodeDecl](astChunk)
	a.symbols, errs[1] = heapslab.NewArena[symbolsDecl](astChunk)
	a.bodies, errs[2] = heapslab.NewArena[varDecl](astChunk)
	a.idents, errs[3] = heapslab.NewArena[exprIdent](astChunk)
	a.arrays, errs[4] = heapslab.NewArena[exprArray](astChunk)
	a.syms, errs[5] = heapslab.NewArena[Keysym](astChunk)
	a.symLists, errs[6] = heapslab.NewArena[exprKeysymList](astChunk)
	a.symsLevels, errs[7] = heapslab.NewArena[exprSymsLevels](astChunk)
	a.args, errs[8] = heapslab.NewArena[expr](astChunk)
	a.actions, errs[9] = heapslab.NewArena[exprAction](astChunk)
	a.assigns, errs[10] = heapslab.NewArena[exprAssign](astChunk)
	a.interps, errs[11] = heapslab.NewArena[interpDecl](astChunk)
	if err := errors.Join(errs[:]...); err != nil {
		return nil, fmt.Errorf("xkb: syntax tree arenas: %w", err)
	}
	return &a, nil
}

func (p *parser) oneDecl(d decl) []decl {
	p.declScratch[0] = d
	return p.declScratch[:1]
}
