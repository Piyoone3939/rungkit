package ladder

// Normalize returns the canonical form of n:
//   - a Series directly inside a Series is flattened (same for Parallel),
//   - containers with one item are replaced by that item,
//   - empty containers disappear (nil).
//
// The same circuit always normalizes to the same tree, which keeps diffs and
// round trips stable.
func Normalize(n Node) Node {
	switch v := n.(type) {
	case *Series:
		var items []Node
		for _, it := range v.Items {
			it = Normalize(it)
			if s, ok := it.(*Series); ok {
				items = append(items, s.Items...)
			} else if it != nil {
				items = append(items, it)
			}
		}
		switch len(items) {
		case 0:
			return nil
		case 1:
			return items[0]
		}
		return &Series{Items: items}
	case *Parallel:
		var items []Node
		for _, it := range v.Items {
			it = Normalize(it)
			if s, ok := it.(*Parallel); ok {
				items = append(items, s.Items...)
			} else if it != nil {
				items = append(items, it)
			}
		}
		switch len(items) {
		case 0:
			return nil
		case 1:
			return items[0]
		}
		return &Parallel{Items: items}
	}
	return n
}

// Normalize normalizes every rung body.
func (p *Program) Normalize() {
	for _, r := range p.Rungs {
		r.Body = Normalize(r.Body)
	}
}

// Part says which side of the rung a node belongs to.
type Part int

const (
	// Cond nodes only pass or block power (contacts, comparisons).
	Cond Part = iota
	// Output nodes end in outputs (coils, timers, instructions).
	Output
)

// ShapeError reports a tree that is not a valid ladder.
type ShapeError struct {
	Code string
	Msg  string
	// Elem is the ID of the element the problem is closest to (may be "").
	Elem string
}

func (e *ShapeError) Error() string { return e.Code + ": " + e.Msg }

// Classify tells whether n is a condition or an output part, or why it is
// neither. The rules:
//   - Series is Output when only its last item is Output, Cond when none is.
//   - Parallel is Cond when all items are Cond, Output when all are Output.
//     Mixing the two is an error (a branch that leads nowhere).
func Classify(n Node) (Part, *ShapeError) {
	switch v := n.(type) {
	case nil:
		return Cond, &ShapeError{Code: "E_EMPTY", Msg: "ラングが空です"}
	case Element:
		if v.IsOutput() {
			return Output, nil
		}
		return Cond, nil
	case *Series:
		if len(v.Items) < 2 {
			return Cond, &ShapeError{Code: "E_NOT_NORMALIZED", Msg: "直列の要素が2つ未満です"}
		}
		part := Cond
		for i, it := range v.Items {
			p, err := Classify(it)
			if err != nil {
				return Cond, err
			}
			if p == Output {
				if i != len(v.Items)-1 {
					return Cond, &ShapeError{
						Code: "E_OUTPUT_NOT_TERMINAL",
						Msg:  "出力の右側にまだ要素があります。出力はラングの右端に置きます",
						Elem: firstID(it),
					}
				}
				part = Output
			}
		}
		return part, nil
	case *Parallel:
		if len(v.Items) < 2 {
			return Cond, &ShapeError{Code: "E_NOT_NORMALIZED", Msg: "並列の要素が2つ未満です"}
		}
		var outs, conds int
		for _, it := range v.Items {
			p, err := Classify(it)
			if err != nil {
				return Cond, err
			}
			if p == Output {
				outs++
			} else {
				conds++
			}
		}
		if outs > 0 && conds > 0 {
			return Cond, &ShapeError{
				Code: "E_MIXED_PARALLEL",
				Msg:  "並列の中に、出力で終わる枝と終わらない枝が混ざっています",
				Elem: firstID(v),
			}
		}
		if outs > 0 {
			return Output, nil
		}
		return Cond, nil
	}
	return Cond, &ShapeError{Code: "E_UNKNOWN_NODE", Msg: "不明なノードです"}
}

// CheckRung validates the shape of a whole rung body: it must end in outputs.
func CheckRung(body Node) *ShapeError {
	p, err := Classify(body)
	if err != nil {
		return err
	}
	if p != Output {
		return &ShapeError{Code: "E_NO_OUTPUT", Msg: "ラングに出力（コイルや命令）がありません", Elem: firstID(body)}
	}
	return nil
}

func firstID(n Node) string {
	if es := Elements(n); len(es) > 0 {
		return es[0].ElemID()
	}
	return ""
}
