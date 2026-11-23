// Package editing is the use case of changing a program with deterministic
// commands that address rungs and elements by ID. A list of commands is
// applied atomically: if any command fails, or a rung ends up with an invalid
// shape, nothing changes. Each applied list is one undo step.
package editing

import (
	"fmt"
	"strings"

	"github.com/Piyoone3939/rungkit/internal/domain/check"
	"github.com/Piyoone3939/rungkit/internal/domain/i18n"
	"github.com/Piyoone3939/rungkit/internal/domain/ladder"
)

// Notation is what editing needs from a text notation (implemented by the
// .ldt adapter in the infrastructure layer).
type Notation interface {
	// ParseFragment reads a condition ("X0 & !X1") or output items
	// ("OUT(Y0)", "M1 -> SET(M10)").
	ParseFragment(text string) (ladder.Node, error)
	// FormatRung writes a rung body, with element IDs when withIDs is set.
	FormatRung(body ladder.Node, withIDs bool) string
}

// Op is one edit command. Which fields are used depends on Op.Op; see Ops.
type Op struct {
	Op       string  `json:"op" jsonschema:"command name: add_rung, replace_rung, delete_rung, move_rung, set_rung, insert_series, insert_parallel, replace_element, delete_element, set_operand, set_symbol, delete_symbol"`
	Rung     string  `json:"rung,omitempty" jsonschema:"rung ID such as r3"`
	After    string  `json:"after,omitempty" jsonschema:"rung ID to place the rung after; empty = at the end, 'start' = at the beginning"`
	Target   string  `json:"target,omitempty" jsonschema:"element ID such as e7"`
	To       string  `json:"to,omitempty" jsonschema:"insert_parallel: element ID ending a series range that starts at target"`
	Position string  `json:"position,omitempty" jsonschema:"insert_series: before or after"`
	Text     string  `json:"text,omitempty" jsonschema:"ladder text, e.g. '(X0 | Y0) & !X1 -> OUT(Y0)', '!X2', 'M1 -> SET(M10)'"`
	Comment  *string `json:"comment,omitempty" jsonschema:"rung or symbol comment"`
	Label    *string `json:"label,omitempty" jsonschema:"short rung identifier: letters, digits and _ only (descriptions go in comment)"`
	Operand  string  `json:"operand,omitempty" jsonschema:"set_operand: new operand, e.g. X10"`
	Name     string  `json:"name,omitempty" jsonschema:"symbol name"`
	Type     string  `json:"type,omitempty" jsonschema:"symbol type (iec profile), e.g. BOOL, INT, TON"`
	Address  string  `json:"address,omitempty" jsonschema:"symbol address (iec profile), e.g. %IX0.0"`
}

// Ops documents the commands and the fields each one uses.
const Ops = `add_rung(text, after?, comment?, label?)   add a rung; text is the rung body
replace_rung(rung, text)                      replace a rung body
delete_rung(rung)
move_rung(rung, after)                        after="" moves to the end, "start" to the beginning
set_rung(rung, comment?, label?)
insert_series(target, position, text)         put conditions before/after an element (AND)
insert_parallel(target, to?, text)            add a branch in parallel with target, or with the
                                              series range target..to: OR for conditions, an
                                              extra output for outputs
replace_element(target, text)                 replace one element with text
delete_element(target)
set_operand(target, operand)                  change the device of a contact, coil, timer or counter
set_symbol(name, type?, address?, comment?)   add or replace a symbol / device comment
delete_symbol(name)`

// RungView is a rung after an edit, with element IDs in its text.
type RungView struct {
	ID      string `json:"id"`
	Comment string `json:"comment,omitempty"`
	Text    string `json:"text"`
}

// Result describes what an applied edit did.
type Result struct {
	Rungs        []RungView         `json:"rungs,omitempty"`
	DeletedRungs []string           `json:"deleted_rungs,omitempty"`
	NewElements  []string           `json:"new_elements,omitempty"`
	Diagnostics  []check.Diagnostic `json:"diagnostics"`
}

// Session holds a program with its undo history.
type Session struct {
	notation   Notation
	prog       *ladder.Program
	undo, redo []*ladder.Program
	dirty      bool
}

// NewSession starts editing p.
func NewSession(p *ladder.Program, n Notation) *Session {
	p.Normalize()
	p.AssignIDs()
	return &Session{notation: n, prog: p}
}

// Program returns the current program. Callers must not modify it.
func (s *Session) Program() *ladder.Program { return s.prog }

// Dirty reports whether there are changes since MarkSaved.
func (s *Session) Dirty() bool { return s.dirty }

// MarkSaved clears the dirty flag.
func (s *Session) MarkSaved() { s.dirty = false }

// View renders a rung with element IDs.
func (s *Session) View(r *ladder.Rung) RungView {
	return RungView{ID: r.ID, Comment: r.Comment, Text: s.notation.FormatRung(r.Body, true)}
}

// Undo reverts the last applied edit.
func (s *Session) Undo() error {
	if len(s.undo) == 0 {
		return i18n.Errorf("元に戻す編集がありません")
	}
	prev := s.undo[len(s.undo)-1]
	s.undo = s.undo[:len(s.undo)-1]
	prev.ReserveIDsOf(s.prog)
	s.redo = append(s.redo, s.prog)
	s.prog = prev
	s.dirty = true
	return nil
}

// Redo re-applies the last undone edit.
func (s *Session) Redo() error {
	if len(s.redo) == 0 {
		return i18n.Errorf("やり直す編集がありません")
	}
	next := s.redo[len(s.redo)-1]
	s.redo = s.redo[:len(s.redo)-1]
	next.ReserveIDsOf(s.prog)
	s.undo = append(s.undo, s.prog)
	s.prog = next
	s.dirty = true
	return nil
}

// Apply runs ops in order on a copy of the program and commits the copy only
// if every op succeeded and every touched rung is a valid ladder.
func (s *Session) Apply(ops []Op) (*Result, error) {
	if len(ops) == 0 {
		return nil, i18n.Errorf("操作がありません")
	}
	work := s.prog.Clone()
	before := map[string]bool{}
	for _, r := range work.Rungs {
		for _, e := range ladder.Elements(r.Body) {
			before[e.ElemID()] = true
		}
	}
	a := &applier{p: work, notation: s.notation, touched: map[string]bool{}}
	for i, op := range ops {
		if err := a.apply(op); err != nil {
			return nil, fmt.Errorf("ops[%d] %s: %w", i, op.Op, err)
		}
		work.Normalize()
		work.AssignIDs()
	}
	res := &Result{DeletedRungs: a.deleted}
	for _, id := range a.order {
		r, _ := work.Rung(id)
		if r == nil {
			continue
		}
		if err := ladder.CheckRung(r.Body); err != nil {
			where := ""
			if err.Elem != "" {
				where = " (" + err.Elem + ")"
			}
			return nil, i18n.Errorf("編集後のラング %s が正しい形になりません%s: %s %s", id, where, err.Code, err.Msg)
		}
		res.Rungs = append(res.Rungs, s.View(r))
	}
	for _, r := range work.Rungs {
		for _, e := range ladder.Elements(r.Body) {
			if !before[e.ElemID()] {
				res.NewElements = append(res.NewElements, e.ElemID())
			}
		}
	}
	res.Diagnostics = check.Program(work)
	if res.Diagnostics == nil {
		res.Diagnostics = []check.Diagnostic{}
	}
	s.undo = append(s.undo, s.prog)
	s.redo = nil
	s.prog = work
	s.dirty = true
	return res, nil
}

type applier struct {
	p        *ladder.Program
	notation Notation
	touched  map[string]bool
	order    []string
	deleted  []string
}

func (a *applier) touch(id string) {
	if !a.touched[id] {
		a.touched[id] = true
		a.order = append(a.order, id)
	}
}

func need(field, val string) error {
	if strings.TrimSpace(val) == "" {
		return i18n.Errorf("%s が必要です", field)
	}
	return nil
}

func (a *applier) rung(id string) (*ladder.Rung, int, error) {
	if err := need("rung", id); err != nil {
		return nil, -1, err
	}
	r, i := a.p.Rung(id)
	if r == nil {
		return nil, -1, i18n.Errorf("ラング %s はありません", id)
	}
	return r, i, nil
}

// insertAt returns the index a rung goes to for the given "after" value.
func (a *applier) insertAt(after string) (int, error) {
	switch after {
	case "":
		return len(a.p.Rungs), nil
	case "start":
		return 0, nil
	}
	r, i := a.p.Rung(after)
	if r == nil {
		return 0, i18n.Errorf("after のラング %s はありません", after)
	}
	return i + 1, nil
}

func (a *applier) fragment(text string) (ladder.Node, error) {
	if err := need("text", text); err != nil {
		return nil, err
	}
	n, err := a.notation.ParseFragment(text)
	if err != nil {
		return nil, i18n.Errorf("text を読めません: %w", err)
	}
	return n, nil
}

func insertNode(items []ladder.Node, at int, n ladder.Node) []ladder.Node {
	return append(items[:at], append([]ladder.Node{n}, items[at:]...)...)
}

func (a *applier) apply(op Op) error {
	switch op.Op {
	case "add_rung":
		body, err := a.fragment(op.Text)
		if err != nil {
			return err
		}
		at, err := a.insertAt(op.After)
		if err != nil {
			return err
		}
		r := &ladder.Rung{ID: a.p.NewRungID(), Body: body}
		if op.Comment != nil {
			r.Comment = *op.Comment
		}
		if op.Label != nil {
			if !ladder.ValidLabel(*op.Label) {
				return i18n.Errorf("label %q は英数字と _ だけで書きます（説明は comment に書きます）", *op.Label)
			}
			r.Label = *op.Label
		}
		a.p.Rungs = append(a.p.Rungs[:at], append([]*ladder.Rung{r}, a.p.Rungs[at:]...)...)
		a.touch(r.ID)
	case "replace_rung":
		r, _, err := a.rung(op.Rung)
		if err != nil {
			return err
		}
		body, err := a.fragment(op.Text)
		if err != nil {
			return err
		}
		r.Body = body
		a.touch(r.ID)
	case "delete_rung":
		r, i, err := a.rung(op.Rung)
		if err != nil {
			return err
		}
		a.p.Rungs = append(a.p.Rungs[:i], a.p.Rungs[i+1:]...)
		a.deleted = append(a.deleted, r.ID)
	case "move_rung":
		r, i, err := a.rung(op.Rung)
		if err != nil {
			return err
		}
		if op.After == r.ID {
			return i18n.Errorf("自分自身の後ろには動かせません")
		}
		a.p.Rungs = append(a.p.Rungs[:i], a.p.Rungs[i+1:]...)
		at, err := a.insertAt(op.After)
		if err != nil {
			return err
		}
		a.p.Rungs = append(a.p.Rungs[:at], append([]*ladder.Rung{r}, a.p.Rungs[at:]...)...)
		a.touch(r.ID)
	case "set_rung":
		r, _, err := a.rung(op.Rung)
		if err != nil {
			return err
		}
		if op.Comment == nil && op.Label == nil {
			return i18n.Errorf("comment か label が必要です")
		}
		if op.Comment != nil {
			r.Comment = *op.Comment
		}
		if op.Label != nil {
			if !ladder.ValidLabel(*op.Label) {
				return i18n.Errorf("label %q は英数字と _ だけで書きます（説明は comment に書きます）", *op.Label)
			}
			r.Label = *op.Label
		}
		a.touch(r.ID)
	case "insert_series":
		l, err := a.find(op.Target)
		if err != nil {
			return err
		}
		frag, err := a.fragment(op.Text)
		if err != nil {
			return err
		}
		var after bool
		switch op.Position {
		case "before":
		case "after":
			after = true
		default:
			return i18n.Errorf("position は before か after です")
		}
		p := l.parent()
		if s, ok := p.node.(*ladder.Series); ok {
			at := p.idx
			if after {
				at++
			}
			s.Items = insertNode(s.Items, at, frag)
		} else if after {
			l.replace(&ladder.Series{Items: []ladder.Node{l.node(), frag}})
		} else {
			l.replace(&ladder.Series{Items: []ladder.Node{frag, l.node()}})
		}
		a.touch(l.rung.ID)
	case "insert_parallel":
		l, err := a.find(op.Target)
		if err != nil {
			return err
		}
		frag, err := a.fragment(op.Text)
		if err != nil {
			return err
		}
		if op.To == "" || op.To == op.Target {
			l.replace(&ladder.Parallel{Items: []ladder.Node{l.node(), frag}})
			a.touch(l.rung.ID)
			return nil
		}
		l2, err := a.find(op.To)
		if err != nil {
			return err
		}
		s, from, to, ok := commonSeries(l, l2)
		if !ok {
			return i18n.Errorf("target と to は同じ直列の中で、target が左になるように指定します")
		}
		span := &ladder.Series{Items: append([]ladder.Node(nil), s.Items[from:to+1]...)}
		par := &ladder.Parallel{Items: []ladder.Node{span, frag}}
		s.Items = append(s.Items[:from], append([]ladder.Node{par}, s.Items[to+1:]...)...)
		a.touch(l.rung.ID)
	case "replace_element":
		l, err := a.find(op.Target)
		if err != nil {
			return err
		}
		frag, err := a.fragment(op.Text)
		if err != nil {
			return err
		}
		l.replace(frag)
		a.touch(l.rung.ID)
	case "delete_element":
		l, err := a.find(op.Target)
		if err != nil {
			return err
		}
		p := l.parent()
		switch c := p.node.(type) {
		case *ladder.Series:
			c.Items = append(c.Items[:p.idx], c.Items[p.idx+1:]...)
		case *ladder.Parallel:
			c.Items = append(c.Items[:p.idx], c.Items[p.idx+1:]...)
		default:
			return i18n.Errorf("%s はラングに1つだけの要素です。ラングごと消すときは delete_rung を使います", op.Target)
		}
		a.touch(l.rung.ID)
	case "set_operand":
		l, err := a.find(op.Target)
		if err != nil {
			return err
		}
		if err := need("operand", op.Operand); err != nil {
			return err
		}
		switch e := l.node().(type) {
		case *ladder.Contact:
			e.Operand = op.Operand
		case *ladder.Coil:
			e.Operand = op.Operand
		case *ladder.Timer:
			e.Instance = op.Operand
		case *ladder.Counter:
			e.Instance = op.Operand
		default:
			return i18n.Errorf("%s のデバイスは set_operand では変えられません（replace_element を使います）", op.Target)
		}
		a.touch(l.rung.ID)
	case "set_symbol":
		if err := need("name", op.Name); err != nil {
			return err
		}
		s := a.p.Symbol(op.Name)
		if s == nil {
			s = &ladder.Symbol{Name: op.Name}
			a.p.Symbols = append(a.p.Symbols, s)
		}
		s.Type, s.Address = strings.ToUpper(op.Type), op.Address
		if op.Comment != nil {
			s.Comment = *op.Comment
		}
	case "delete_symbol":
		if err := need("name", op.Name); err != nil {
			return err
		}
		for i, s := range a.p.Symbols {
			if s.Name == op.Name {
				a.p.Symbols = append(a.p.Symbols[:i], a.p.Symbols[i+1:]...)
				return nil
			}
		}
		return i18n.Errorf("sym %s はありません", op.Name)
	case "":
		return i18n.Errorf("op が必要です")
	default:
		return i18n.Errorf("操作 %q はありません。使えるもの:\n%s", op.Op, Ops)
	}
	return nil
}

// step is one level of a path from a rung body down to an element: the
// container and the index of the child taken. node is nil at the rung root.
type step struct {
	node ladder.Node
	idx  int
}

// location is the path to an element inside a rung.
type location struct {
	rung *ladder.Rung
	path []step // path[0] is the rung root (node nil), the last step leads to the element
	elem ladder.Node
}

func (l *location) node() ladder.Node { return l.elem }

// parent is the container holding the element (node nil when the element is
// the whole rung body).
func (l *location) parent() step { return l.path[len(l.path)-1] }

func (l *location) replace(n ladder.Node) {
	p := l.parent()
	switch c := p.node.(type) {
	case *ladder.Series:
		c.Items[p.idx] = n
	case *ladder.Parallel:
		c.Items[p.idx] = n
	default:
		l.rung.Body = n
	}
	l.elem = n
}

// commonSeries finds the deepest Series that holds both locations through
// two of its children, and returns their indices (from <= to).
func commonSeries(a, b *location) (*ladder.Series, int, int, bool) {
	for i := min(len(a.path), len(b.path)) - 1; i >= 0; i-- {
		if a.path[i].node != b.path[i].node {
			continue
		}
		if s, ok := a.path[i].node.(*ladder.Series); ok {
			from, to := a.path[i].idx, b.path[i].idx
			if from <= to {
				return s, from, to, true
			}
		}
		return nil, 0, 0, false
	}
	return nil, 0, 0, false
}

func (a *applier) find(id string) (*location, error) {
	if err := need("target", id); err != nil {
		return nil, err
	}
	for _, r := range a.p.Rungs {
		if path, elem := search(r.Body, []step{{}}, id); elem != nil {
			return &location{rung: r, path: path, elem: elem}, nil
		}
	}
	return nil, i18n.Errorf("要素 %s はありません", id)
}

// search walks n looking for the element id. path holds the steps that led to
// n; the last step's idx is n's index in its container.
func search(n ladder.Node, path []step, id string) ([]step, ladder.Node) {
	var items []ladder.Node
	switch v := n.(type) {
	case *ladder.Series:
		items = v.Items
	case *ladder.Parallel:
		items = v.Items
	case ladder.Element:
		if v.ElemID() == id {
			return path, n
		}
		return nil, nil
	}
	for i, it := range items {
		next := append(append([]step(nil), path...), step{node: n, idx: i})
		if p, e := search(it, next, id); e != nil {
			return p, e
		}
	}
	return nil, nil
}
