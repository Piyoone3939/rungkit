// Package ir is rungkit's vendor-neutral ladder representation.
//
// A rung is a series-parallel tree. Coordinates are never stored; layout is
// derived from the tree. Every element carries an ID that stays stable while
// the program is edited, so tools can address "the element e12" instead of
// "the third contact".
package ladder

import (
	"fmt"
	"strconv"
	"strings"
)

// Program is one ladder program (a single POU in this version).
type Program struct {
	Profile string
	Symbols []*Symbol
	Rungs   []*Rung

	nextRung int
	nextElem int
}

// Symbol declares a variable or gives a device a comment.
type Symbol struct {
	Name    string
	Type    string // BOOL, INT, TON ... (required by the iec profile)
	Address string // %IX0.0 ... (optional)
	Comment string
}

// Rung is one horizontal line of the ladder.
type Rung struct {
	ID      string
	Label   string // optional user-visible name, persisted in .ldt
	Comment string
	Body    Node
}

// Node is a Series, a Parallel or an Element.
type Node interface{ node() }

// Series connects items left to right (AND on the condition side).
type Series struct{ Items []Node }

// Parallel connects items top to bottom (OR on the condition side, an output
// branch on the output side).
type Parallel struct{ Items []Node }

func (*Series) node()   {}
func (*Parallel) node() {}

// Element is a leaf of the rung tree.
type Element interface {
	Node
	ElemID() string
	SetElemID(string)
	// IsOutput reports whether the element must sit at the right end of a
	// path (coils, timers, instructions).
	IsOutput() bool
}

// Base holds the fields every element shares.
type Base struct{ ID string }

func (b *Base) ElemID() string      { return b.ID }
func (b *Base) SetElemID(id string) { b.ID = id }

// ContactMode is the kind of a contact.
type ContactMode string

const (
	NO   ContactMode = "NO"   // a 接点
	NC   ContactMode = "NC"   // b 接点
	Rise ContactMode = "RISE" // 立上り
	Fall ContactMode = "FALL" // 立下り
)

// Contact passes power depending on a bit operand.
type Contact struct {
	Base
	Mode    ContactMode
	Operand string
}

// Compare passes power when LHS Op RHS holds. Op is one of = <> < <= > >=.
type Compare struct {
	Base
	Op       string
	LHS, RHS string
}

// CoilMode is the kind of a coil.
type CoilMode string

const (
	Out       CoilMode = "OUT"
	Set       CoilMode = "SET"
	Reset     CoilMode = "RST"
	Pulse     CoilMode = "PLS"
	PulseFall CoilMode = "PLF"
)

// Coil writes a bit operand (RST may also clear timers, counters and words).
type Coil struct {
	Base
	Mode    CoilMode
	Operand string
}

// Timer is an on-delay style timer placed as an output (MELSEC style).
type Timer struct {
	Base
	Kind     string // TON (TOF, TP for the iec profile)
	Instance string
	Preset   string
}

// Counter is an up counter placed as an output.
type Counter struct {
	Base
	Kind     string // CTU
	Instance string
	Preset   string
}

// Instr is an application instruction such as MOV or ADD.
type Instr struct {
	Base
	Name string
	Args []string
}

func (*Contact) node() {}
func (*Compare) node() {}
func (*Coil) node()    {}
func (*Timer) node()   {}
func (*Counter) node() {}
func (*Instr) node()   {}

func (*Contact) IsOutput() bool { return false }
func (*Compare) IsOutput() bool { return false }
func (*Coil) IsOutput() bool    { return true }
func (*Timer) IsOutput() bool   { return true }
func (*Counter) IsOutput() bool { return true }
func (*Instr) IsOutput() bool   { return true }

// CompareOps lists the comparison operators in canonical spelling.
var CompareOps = []string{"=", "<>", "<", "<=", ">", ">="}

// Walk calls fn for n and every node below it, depth first, left to right.
func Walk(n Node, fn func(Node)) {
	if n == nil {
		return
	}
	fn(n)
	switch v := n.(type) {
	case *Series:
		for _, it := range v.Items {
			Walk(it, fn)
		}
	case *Parallel:
		for _, it := range v.Items {
			Walk(it, fn)
		}
	}
}

// Elements returns the elements under n in document order.
func Elements(n Node) []Element {
	var out []Element
	Walk(n, func(x Node) {
		if e, ok := x.(Element); ok {
			out = append(out, e)
		}
	})
	return out
}

// Symbol looks up a symbol by name.
func (p *Program) Symbol(name string) *Symbol {
	for _, s := range p.Symbols {
		if s.Name == name {
			return s
		}
	}
	return nil
}

// AssignIDs gives an ID to every rung and element that has none. IDs already
// present are kept, and new IDs never collide with them.
func (p *Program) AssignIDs() {
	for _, r := range p.Rungs {
		bump(&p.nextRung, r.ID, "r")
		for _, e := range Elements(r.Body) {
			bump(&p.nextElem, e.ElemID(), "e")
		}
	}
	for _, r := range p.Rungs {
		if r.ID == "" {
			r.ID = p.NewRungID()
		}
		for _, e := range Elements(r.Body) {
			if e.ElemID() == "" {
				e.SetElemID(p.NewElemID())
			}
		}
	}
}

// NewRungID returns an unused rung ID.
func (p *Program) NewRungID() string {
	p.nextRung++
	return fmt.Sprintf("r%d", p.nextRung)
}

// NewElemID returns an unused element ID.
func (p *Program) NewElemID() string {
	p.nextElem++
	return fmt.Sprintf("e%d", p.nextElem)
}

func bump(counter *int, id, prefix string) {
	if !strings.HasPrefix(id, prefix) {
		return
	}
	if n, err := strconv.Atoi(id[len(prefix):]); err == nil && n > *counter {
		*counter = n
	}
}

// ValidLabel tells whether s can be a rung label: letters, digits and _,
// not starting with a digit. Descriptions belong in the rung comment.
func ValidLabel(s string) bool {
	if s == "" {
		return true
	}
	for i, r := range s {
		switch {
		case r == '_', r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}
