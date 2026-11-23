package ladder

// Clone returns a deep copy of p, including its ID counters, so IDs handed
// out after an undo never collide with IDs used before it.
func (p *Program) Clone() *Program {
	q := &Program{Profile: p.Profile, nextRung: p.nextRung, nextElem: p.nextElem}
	for _, s := range p.Symbols {
		c := *s
		q.Symbols = append(q.Symbols, &c)
	}
	for _, r := range p.Rungs {
		q.Rungs = append(q.Rungs, &Rung{ID: r.ID, Label: r.Label, Comment: r.Comment, Body: CloneNode(r.Body)})
	}
	return q
}

// ReserveIDsOf makes p hand out only IDs that other has not handed out
// either. Used after undo, so IDs given out in the undone edit stay unique.
func (p *Program) ReserveIDsOf(other *Program) {
	p.nextRung = max(p.nextRung, other.nextRung)
	p.nextElem = max(p.nextElem, other.nextElem)
}

// CloneNode deep-copies a node, keeping element IDs.
func CloneNode(n Node) Node {
	switch v := n.(type) {
	case *Series:
		return &Series{Items: cloneItems(v.Items)}
	case *Parallel:
		return &Parallel{Items: cloneItems(v.Items)}
	case *Contact:
		c := *v
		return &c
	case *Compare:
		c := *v
		return &c
	case *Coil:
		c := *v
		return &c
	case *Timer:
		c := *v
		return &c
	case *Counter:
		c := *v
		return &c
	case *Instr:
		c := *v
		c.Args = append([]string(nil), v.Args...)
		return &c
	}
	return nil
}

func cloneItems(items []Node) []Node {
	out := make([]Node, len(items))
	for i, it := range items {
		out[i] = CloneNode(it)
	}
	return out
}

// Rung returns the rung with the given ID and its index, or nil and -1.
func (p *Program) Rung(id string) (*Rung, int) {
	for i, r := range p.Rungs {
		if r.ID == id {
			return r, i
		}
	}
	return nil, -1
}
