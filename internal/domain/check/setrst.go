package check

import "github.com/Piyoone3939/rungkit/internal/domain/ladder"

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
