package ldt

import (
	"fmt"
	"strings"

	"rungkit/internal/domain/ladder"
)

// SourceMap remembers where each rung and element came from, so diagnostics
// can point at a line.
type SourceMap struct {
	Rungs map[string]Pos
	Elems map[string]Pos
}

// DefaultProfile is used when a file has no profile line.
const DefaultProfile = "melsec-iqr"

// Parse reads a .ldt source. The returned program is normalized and has IDs.
func Parse(src string) (*ladder.Program, *SourceMap, error) {
	toks, err := lex(src)
	if err != nil {
		return nil, nil, err
	}
	ps := &parser{toks: toks, prog: &ladder.Program{}, elemPos: map[ladder.Element]Pos{}}
	rungPos := map[*ladder.Rung]Pos{}
	for ps.peek().kind != tEOF {
		t := ps.peek()
		if t.kind != tWord {
			return nil, nil, ps.errf(t, "profile / sym / rung のどれかを書きます")
		}
		switch t.text {
		case "profile":
			ps.next()
			w, err := ps.expect(tWord)
			if err != nil {
				return nil, nil, err
			}
			if ps.prog.Profile != "" {
				return nil, nil, ps.errf(t, "profile が2回書かれています")
			}
			ps.prog.Profile = w.text
		case "sym":
			if err := ps.parseSym(); err != nil {
				return nil, nil, err
			}
		case "rung":
			r, err := ps.parseRung()
			if err != nil {
				return nil, nil, err
			}
			rungPos[r] = t.pos
			ps.prog.Rungs = append(ps.prog.Rungs, r)
		default:
			return nil, nil, ps.errf(t, "profile / sym / rung のどれかを書きます（%q は使えません）", t.text)
		}
	}
	p := ps.prog
	if p.Profile == "" {
		p.Profile = DefaultProfile
	}
	p.Normalize()
	p.AssignIDs()
	sm := &SourceMap{Rungs: map[string]Pos{}, Elems: map[string]Pos{}}
	for r, pos := range rungPos {
		sm.Rungs[r.ID] = pos
	}
	for e, pos := range ps.elemPos {
		sm.Elems[e.ElemID()] = pos
	}
	return p, sm, nil
}

type parser struct {
	toks    []token
	i       int
	prog    *ladder.Program
	elemPos map[ladder.Element]Pos
}

func (ps *parser) peek() token { return ps.toks[ps.i] }
func (ps *parser) peekAt(n int) token {
	if ps.i+n < len(ps.toks) {
		return ps.toks[ps.i+n]
	}
	return ps.toks[len(ps.toks)-1]
}
func (ps *parser) next() token {
	t := ps.toks[ps.i]
	if t.kind != tEOF {
		ps.i++
	}
	return t
}

func (ps *parser) errf(t token, format string, args ...any) error {
	return &Error{t.pos, fmt.Sprintf(format, args...)}
}

func (ps *parser) expect(k tokKind) (token, error) {
	t := ps.next()
	if t.kind != k {
		got := tokNames[t.kind]
		if t.kind == tWord || t.kind == tCmp {
			got = fmt.Sprintf("%q", t.text)
		}
		return t, ps.errf(t, "%s が必要ですが、%s があります", tokNames[k], got)
	}
	return t, nil
}

// elem records where e came from. It also skips an "@e12" ID annotation
// written right after the element (as FormatWithIDs prints it), so annotated
// text can be pasted back. The annotation is ignored; new IDs are assigned.
func (ps *parser) elem(e ladder.Element, t token) ladder.Element {
	ps.elemPos[e] = t.pos
	if ps.peek().kind == tAt && ps.peekAt(1).kind == tWord {
		ps.next()
		ps.next()
	}
	return e
}

// ParseFragment reads a piece of a rung: either a condition ("X0 & !X1",
// "(X0 | Y0)") or output items ("OUT(Y0)", "M1 -> SET(M10)", several items
// make an output branch). The result is normalized and has no IDs.
func ParseFragment(src string) (ladder.Node, error) {
	toks, err := lex(src)
	if err != nil {
		return nil, err
	}
	newParser := func() *parser {
		return &parser{toks: toks, prog: &ladder.Program{}, elemPos: map[ladder.Element]Pos{}}
	}
	ps := newParser()
	if ps.peek().kind == tEOF {
		return nil, &Error{ps.peek().pos, "中身が空です"}
	}
	if !ps.isCall() && ps.peek().kind != tLBrace {
		if n, err := ps.parseOr(); err == nil && ps.peek().kind == tEOF {
			return ladder.Normalize(n), nil
		}
	}
	ps = newParser()
	var items []ladder.Node
	for ps.peek().kind != tEOF {
		if ps.peek().kind == tSemi {
			ps.next()
			continue
		}
		it, err := ps.parseItem()
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	if len(items) == 1 {
		return ladder.Normalize(items[0]), nil
	}
	return ladder.Normalize(&ladder.Parallel{Items: items}), nil
}

// sym NAME [: TYPE] [@ ADDR] ["comment"]
func (ps *parser) parseSym() error {
	ps.next()
	name, err := ps.expect(tWord)
	if err != nil {
		return err
	}
	s := &ladder.Symbol{Name: name.text}
	if ps.peek().kind == tColon {
		ps.next()
		t, err := ps.expect(tWord)
		if err != nil {
			return err
		}
		s.Type = strings.ToUpper(t.text)
	}
	if ps.peek().kind == tAt {
		ps.next()
		t, err := ps.expect(tWord)
		if err != nil {
			return err
		}
		s.Address = t.text
	}
	if ps.peek().kind == tString {
		s.Comment = ps.next().text
	}
	if ps.prog.Symbol(s.Name) != nil {
		return ps.errf(name, "sym %s が2回宣言されています", s.Name)
	}
	ps.prog.Symbols = append(ps.prog.Symbols, s)
	return nil
}

// rung [LABEL] ["comment"] { item... }
func (ps *parser) parseRung() (*ladder.Rung, error) {
	ps.next()
	r := &ladder.Rung{}
	if ps.peek().kind == tWord {
		r.Label = ps.next().text
	}
	if ps.peek().kind == tString {
		r.Comment = ps.next().text
	}
	open, err := ps.expect(tLBrace)
	if err != nil {
		return nil, err
	}
	body, err := ps.parseItems(open)
	if err != nil {
		return nil, err
	}
	r.Body = body
	return r, nil
}

// parseItems reads output items up to the closing brace. Several items form
// an output branch (Parallel).
func (ps *parser) parseItems(open token) (ladder.Node, error) {
	var items []ladder.Node
	for {
		for ps.peek().kind == tSemi {
			ps.next()
		}
		if ps.peek().kind == tRBrace {
			ps.next()
			break
		}
		if ps.peek().kind == tEOF {
			return nil, ps.errf(open, "{ が閉じていません")
		}
		it, err := ps.parseItem()
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	switch len(items) {
	case 0:
		return nil, ps.errf(open, "{ } の中に出力がありません")
	case 1:
		return items[0], nil
	}
	return &ladder.Parallel{Items: items}, nil
}

// item := call | { item... } | cond -> target
func (ps *parser) parseItem() (ladder.Node, error) {
	if ps.peek().kind == tLBrace {
		return ps.parseItems(ps.next())
	}
	if ps.isCall() {
		return ps.parseCall()
	}
	cond, err := ps.parseOr()
	if err != nil {
		return nil, err
	}
	if t := ps.peek(); t.kind != tArrow {
		return nil, ps.errf(t, "条件の後には -> と出力を書きます")
	}
	ps.next()
	var target ladder.Node
	if ps.peek().kind == tLBrace {
		target, err = ps.parseItems(ps.next())
	} else {
		target, err = ps.parseItem()
	}
	if err != nil {
		return nil, err
	}
	return &ladder.Series{Items: []ladder.Node{cond, target}}, nil
}

func (ps *parser) isCall() bool {
	t := ps.peek()
	if t.kind != tWord || ps.peekAt(1).kind != tLParen {
		return false
	}
	return t.text != "rise" && t.text != "fall"
}

// call := NAME ( [arg {, arg}] )
func (ps *parser) parseCall() (ladder.Node, error) {
	name := ps.next()
	ps.next() // (
	var args []string
	if ps.peek().kind != tRParen {
		for {
			a, err := ps.expect(tWord)
			if err != nil {
				return nil, err
			}
			args = append(args, a.text)
			if ps.peek().kind != tComma {
				break
			}
			ps.next()
		}
	}
	if _, err := ps.expect(tRParen); err != nil {
		return nil, err
	}
	n := strings.ToUpper(name.text)
	want := func(k int) error {
		if len(args) != k {
			return ps.errf(name, "%s の引数は %d 個です（%d 個あります）", n, k, len(args))
		}
		return nil
	}
	switch n {
	case "OUT", "SET", "RST", "PLS", "PLF":
		if err := want(1); err != nil {
			return nil, err
		}
		return ps.elem(&ladder.Coil{Mode: ladder.CoilMode(n), Operand: args[0]}, name), nil
	case "TON", "TOF", "TP":
		if err := want(2); err != nil {
			return nil, err
		}
		return ps.elem(&ladder.Timer{Kind: n, Instance: args[0], Preset: args[1]}, name), nil
	case "CTU":
		if err := want(2); err != nil {
			return nil, err
		}
		return ps.elem(&ladder.Counter{Kind: n, Instance: args[0], Preset: args[1]}, name), nil
	}
	return ps.elem(&ladder.Instr{Name: n, Args: args}, name), nil
}

// cond := term { | term }
func (ps *parser) parseOr() (ladder.Node, error) {
	first, err := ps.parseAnd()
	if err != nil {
		return nil, err
	}
	items := []ladder.Node{first}
	for ps.peek().kind == tOr {
		ps.next()
		n, err := ps.parseAnd()
		if err != nil {
			return nil, err
		}
		items = append(items, n)
	}
	if len(items) == 1 {
		return first, nil
	}
	return &ladder.Parallel{Items: items}, nil
}

// term := factor { & factor }
func (ps *parser) parseAnd() (ladder.Node, error) {
	first, err := ps.parseFactor()
	if err != nil {
		return nil, err
	}
	items := []ladder.Node{first}
	for ps.peek().kind == tAnd {
		ps.next()
		n, err := ps.parseFactor()
		if err != nil {
			return nil, err
		}
		items = append(items, n)
	}
	if len(items) == 1 {
		return first, nil
	}
	return &ladder.Series{Items: items}, nil
}

func (ps *parser) parseFactor() (ladder.Node, error) {
	t := ps.peek()
	switch t.kind {
	case tNot:
		ps.next()
		w := ps.peek()
		if w.kind != tWord || ps.peekAt(1).kind == tLParen {
			return nil, ps.errf(t, "! は1つの接点にだけ付けられます（例: !X1）")
		}
		ps.next()
		return ps.elem(&ladder.Contact{Mode: ladder.NC, Operand: w.text}, t), nil
	case tLParen:
		ps.next()
		n, err := ps.parseOr()
		if err != nil {
			return nil, err
		}
		if _, err := ps.expect(tRParen); err != nil {
			return nil, err
		}
		return n, nil
	case tLBrack:
		ps.next()
		lhs, err := ps.expect(tWord)
		if err != nil {
			return nil, err
		}
		op, err := ps.expect(tCmp)
		if err != nil {
			return nil, err
		}
		rhs, err := ps.expect(tWord)
		if err != nil {
			return nil, err
		}
		if _, err := ps.expect(tRBrack); err != nil {
			return nil, err
		}
		return ps.elem(&ladder.Compare{Op: op.text, LHS: lhs.text, RHS: rhs.text}, t), nil
	case tWord:
		if t.text == "rise" || t.text == "fall" {
			if ps.peekAt(1).kind == tLParen {
				ps.next()
				ps.next()
				w, err := ps.expect(tWord)
				if err != nil {
					return nil, err
				}
				if _, err := ps.expect(tRParen); err != nil {
					return nil, err
				}
				mode := ladder.Rise
				if t.text == "fall" {
					mode = ladder.Fall
				}
				return ps.elem(&ladder.Contact{Mode: mode, Operand: w.text}, t), nil
			}
		}
		if ps.peekAt(1).kind == tLParen {
			return nil, ps.errf(t, "出力命令 %s(...) は -> の右側に書きます", t.text)
		}
		ps.next()
		return ps.elem(&ladder.Contact{Mode: ladder.NO, Operand: t.text}, t), nil
	}
	return nil, ps.errf(t, "接点・比較・( のどれかが必要です")
}
