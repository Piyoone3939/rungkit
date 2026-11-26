package check

import (
	"strings"

	"github.com/Piyoone3939/rungkit/internal/domain/ladder"
	"github.com/Piyoone3939/rungkit/internal/domain/profile"
)

// guarded is a SET or RST coil with the contacts in front of it.
type guarded struct {
	rung  string
	coil  *ladder.Coil
	edges []*ladder.Contact // edge contacts every path to the coil passes
	reads map[string]bool   // operands of all contacts that feed the coil
}

// guards lists the SET/RST coils of body in scan order.
func guards(rung string, body ladder.Node) []guarded {
	var out []guarded
	var walk func(n ladder.Node, edges []*ladder.Contact, reads []string)
	walk = func(n ladder.Node, edges []*ladder.Contact, reads []string) {
		switch v := n.(type) {
		case *ladder.Series:
			for _, it := range v.Items {
				walk(it, edges, reads)
				for _, e := range ladder.Elements(it) {
					if c, ok := e.(*ladder.Contact); ok {
						reads = append(reads, c.Operand)
					}
				}
				if c, ok := it.(*ladder.Contact); ok && (c.Mode == ladder.Rise || c.Mode == ladder.Fall) {
					edges = append(edges, c)
				}
			}
		case *ladder.Parallel:
			for _, it := range v.Items {
				walk(it, edges, reads)
			}
		case *ladder.Coil:
			if v.Mode != ladder.Set && v.Mode != ladder.Reset {
				return
			}
			g := guarded{rung: rung, coil: v, edges: append([]*ladder.Contact(nil), edges...), reads: map[string]bool{}}
			for _, r := range reads {
				g.reads[r] = true
			}
			out = append(out, g)
		}
	}
	walk(body, nil, nil)
	return out
}

func sameEdge(a, b guarded) *ladder.Contact {
	for _, x := range a.edges {
		for _, y := range b.edges {
			if x.Mode == y.Mode && x.Operand == y.Operand {
				return x
			}
		}
	}
	return nil
}

// setResetSameEdge warns about a SET and an RST of one device that fire on
// the same edge when the later one also reads the device: the later rung sees
// what the earlier one just wrote, so both fire in the same scan and cancel
// out. This is the usual broken push-on/push-off toggle.
func (c *checker) setResetSameEdge(p *ladder.Program) {
	var all []guarded
	for _, r := range p.Rungs {
		if ladder.CheckRung(r.Body) != nil {
			continue
		}
		all = append(all, guards(r.ID, r.Body)...)
	}
	for j, later := range all {
		if !later.reads[later.coil.Operand] {
			continue
		}
		for _, first := range all[:j] {
			if first.coil.Operand != later.coil.Operand || first.coil.Mode == later.coil.Mode {
				continue
			}
			edge := sameEdge(first, later)
			if edge == nil {
				continue
			}
			c.add(Warning, "W_SET_RST_SAME_EDGE", later.rung, later.coil.ElemID(),
				"%s(%s) は %s(%s) と同じ %s で動き、条件で %s を見ています。同じスキャンで %s の結果を打ち消します（押すたびに切り替えるなら、1つのラングで OUT を使います）",
				later.coil.Mode, later.coil.Operand, first.coil.Mode, first.coil.ElemID(), edgeText(edge), later.coil.Operand, first.coil.ElemID())
			break
		}
	}
}

func edgeText(e *ladder.Contact) string {
	if e.Mode == ladder.Fall {
		return "fall(" + e.Operand + ")"
	}
	return "rise(" + e.Operand + ")"
}

// setWithoutReset warns about a device that is SET somewhere but never
// turned off by the program: no RST of it (or of the word holding the bit),
// no instruction writing that word, and no OUT, PLS or PLF of it.
func (c *checker) setWithoutReset(p *ladder.Program) {
	type first struct{ rung, elem string }
	sets := map[string]first{}
	var order []string
	cleared := map[string]bool{}
	for _, r := range p.Rungs {
		if ladder.CheckRung(r.Body) != nil {
			continue
		}
		for _, e := range ladder.Elements(r.Body) {
			switch v := e.(type) {
			case *ladder.Coil:
				if v.Mode == ladder.Set {
					if _, ok := sets[v.Operand]; !ok {
						sets[v.Operand] = first{r.ID, v.ElemID()}
						order = append(order, v.Operand)
					}
				} else {
					cleared[v.Operand] = true
				}
			case *ladder.Instr:
				if spec, ok := c.prof.Instr(v.Name); ok && len(spec.Args) == len(v.Args) {
					for i, a := range v.Args {
						if spec.Args[i] == profile.Dst {
							cleared[a] = true
						}
					}
				}
			}
		}
	}
	for _, op := range order {
		if cleared[op] || cleared[wordOf(op)] {
			continue
		}
		f := sets[op]
		c.add(Warning, "W_SET_WITHOUT_RST", f.rung, f.elem,
			"%s を SET していますが、プログラムのどこにも RST がありません。一度 ON になると、プログラムでは OFF に戻りません（表示器など外から戻すなら、この警告は無視できます）", op)
	}
}

// wordOf returns the word of a bit-of-word operand such as D0.3, or "".
func wordOf(op string) string {
	if i := strings.LastIndexByte(op, '.'); i > 0 {
		return op[:i]
	}
	return ""
}
