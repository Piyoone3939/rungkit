// Package melsec writes a program as a MELSEC instruction list
// (LD / AND / OR / ANB / ORB / MPS / MRD / MPP ...).
//
// The output is one instruction per line, operands separated by tabs, with
// "; " comment lines for rung comments. The exact file layout GX Works3
// imports has not been confirmed yet, so treat the layout as provisional; the
// instruction sequence itself follows the standard conversion rules.
package melsec

import (
	"fmt"
	"strings"

	"rungkit/check"
	"rungkit/ir"
	"rungkit/profile"
)

// AlwaysOn is the always-ON special relay used for unconditional outputs.
const AlwaysOn = "SM400"

// Export converts p. It refuses programs that have check errors or that
// target another vendor.
func Export(p *ir.Program) (string, error) {
	if !strings.HasPrefix(p.Profile, "melsec") {
		return "", fmt.Errorf("三菱リスト形式は melsec のプロファイルでだけ書き出せます（今は %s）", p.Profile)
	}
	if ds := check.Program(p); check.HasErrors(ds) {
		return "", fmt.Errorf("エラーがあるので書き出せません（rungkit check で確認できます）")
	}
	prof, _ := profile.Get(p.Profile)
	w := &writer{prof: prof}
	for _, r := range p.Rungs {
		if r.Comment != "" {
			w.lines = append(w.lines, "; "+r.Comment)
		}
		if err := w.outputs(r.Body, true); err != nil {
			return "", fmt.Errorf("rung %s: %w", r.ID, err)
		}
	}
	w.emit("END")
	return strings.Join(w.lines, "\n") + "\n", nil
}

type writer struct {
	prof  profile.Profile
	lines []string
}

func (w *writer) emit(ins string, ops ...string) {
	w.lines = append(w.lines, strings.Join(append([]string{ins}, ops...), "\t"))
}

// outputs writes an Output node. atRail means no condition has been loaded
// yet (the node hangs directly off the left rail).
func (w *writer) outputs(n ir.Node, atRail bool) error {
	switch v := n.(type) {
	case *ir.Series:
		prefix, tail := v.Items[:len(v.Items)-1], v.Items[len(v.Items)-1]
		for i, it := range prefix {
			if i == 0 && atRail {
				w.load(it)
			} else {
				w.and(it)
			}
		}
		return w.outputs(tail, false)
	case *ir.Parallel:
		if atRail {
			// Separate circuits sharing the rail: each loads its own condition.
			for _, it := range v.Items {
				if err := w.outputs(it, true); err != nil {
					return err
				}
			}
			return nil
		}
		return w.branch(v.Items)
	case ir.Element:
		if atRail {
			w.emit("LD", AlwaysOn)
		}
		return w.output(v)
	}
	return fmt.Errorf("unexpected node %T", n)
}

// branch writes an output branch. The operation result only needs saving
// (MPS) when a branch other than the last one adds conditions of its own.
func (w *writer) branch(items []ir.Node) error {
	modifies := func(n ir.Node) bool { _, ok := n.(*ir.Series); return ok }
	laterModifier := func(from int) bool { // any modifier in items[from : len-1]
		for j := from; j < len(items)-1; j++ {
			if modifies(items[j]) {
				return true
			}
		}
		return false
	}
	pushed, dirty := false, false
	for i, it := range items {
		if dirty {
			if laterModifier(i) {
				w.emit("MRD")
			} else {
				w.emit("MPP")
				pushed = false
			}
			dirty = false
		}
		if modifies(it) && i < len(items)-1 && !pushed {
			w.emit("MPS")
			pushed = true
		}
		if err := w.outputs(it, false); err != nil {
			return err
		}
		if modifies(it) {
			dirty = true
		}
	}
	return nil
}

// load starts a new logic block with n.
func (w *writer) load(n ir.Node) {
	switch v := n.(type) {
	case *ir.Series:
		w.load(v.Items[0])
		for _, it := range v.Items[1:] {
			w.and(it)
		}
	case *ir.Parallel:
		w.load(v.Items[0])
		for _, it := range v.Items[1:] {
			w.or(it)
		}
	default:
		w.contact("LD", n)
	}
}

// and connects n in series with the current block.
func (w *writer) and(n ir.Node) {
	switch v := n.(type) {
	case *ir.Parallel:
		w.load(v)
		w.emit("ANB")
	case *ir.Series:
		for _, it := range v.Items {
			w.and(it)
		}
	default:
		w.contact("AND", n)
	}
}

// or connects n in parallel with the current block.
func (w *writer) or(n ir.Node) {
	switch v := n.(type) {
	case *ir.Series:
		w.load(v)
		w.emit("ORB")
	case *ir.Parallel:
		for _, it := range v.Items {
			w.or(it)
		}
	default:
		w.contact("OR", n)
	}
}

// contact writes LD/AND/OR forms of a single condition element.
func (w *writer) contact(base string, n ir.Node) {
	switch v := n.(type) {
	case *ir.Contact:
		suffix := map[ir.ContactMode]string{ir.NO: "", ir.NC: "I", ir.Rise: "P", ir.Fall: "F"}[v.Mode]
		ins := base + suffix
		if base == "AND" && v.Mode == ir.NC {
			ins = "ANI" // AND + I is spelled ANI
		}
		w.emit(ins, v.Operand)
	case *ir.Compare:
		w.emit(base+v.Op, v.LHS, v.RHS)
	}
}

func (w *writer) output(e ir.Element) error {
	switch v := e.(type) {
	case *ir.Coil:
		w.emit(string(v.Mode), v.Operand)
	case *ir.Timer:
		w.emit("OUT", v.Instance, v.Preset)
	case *ir.Counter:
		w.emit("OUT", v.Instance, v.Preset)
	case *ir.Instr:
		spec, ok := w.prof.Instr(v.Name)
		if !ok {
			return fmt.Errorf("命令 %s は使えません", v.Name)
		}
		w.emit(spec.Mnemonic, v.Args...)
	default:
		return fmt.Errorf("出力ではない要素 %T", e)
	}
	return nil
}
