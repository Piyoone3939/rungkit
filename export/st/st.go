// Package st writes a program as IEC 61131-3 Structured Text.
//
// Each rung becomes statements that keep ladder semantics: rungs run top to
// bottom, outputs are written immediately, and an output branch first saves
// the shared condition in a temporary so later branches see the value the
// ladder would (like MPS on MELSEC).
package st

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"rungkit/check"
	"rungkit/ir"
	"rungkit/profile"
)

// Export converts p into a PROGRAM named name.
func Export(p *ir.Program, name string) (string, error) {
	if ds := check.Program(p); check.HasErrors(ds) {
		return "", fmt.Errorf("エラーがあるので書き出せません（rungkit check で確認できます）")
	}
	prof, _ := profile.Get(p.Profile)
	g := &gen{p: p, prof: prof, vars: map[string]string{}, melsec: strings.HasPrefix(p.Profile, "melsec")}
	var body strings.Builder
	for ri, r := range p.Rungs {
		g.pre, g.stmts, g.tmp = nil, nil, 0
		g.rungIdx = ri + 1
		if err := g.outputs(r.Body, "TRUE"); err != nil {
			return "", fmt.Errorf("rung %s: %w", r.ID, err)
		}
		if r.Comment != "" {
			fmt.Fprintf(&body, "(* %s *)\n", r.Comment)
		}
		for _, s := range append(g.pre, g.stmts...) {
			body.WriteString(s + "\n")
		}
	}
	var out strings.Builder
	fmt.Fprintf(&out, "PROGRAM %s\nVAR\n", name)
	names := make([]string, 0, len(g.vars))
	for n := range g.vars {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool { return varOrder(names[i]) < varOrder(names[j]) })
	for _, n := range names {
		decl := fmt.Sprintf("  %s : %s;", n, g.vars[n])
		if s := p.Symbol(n); s != nil && s.Comment != "" {
			decl += " (* " + s.Comment + " *)"
		}
		out.WriteString(decl + "\n")
	}
	out.WriteString("END_VAR\n\n")
	out.WriteString(body.String())
	out.WriteString("END_PROGRAM\n")
	return out.String(), nil
}

// varOrder sorts user operands before generated helpers, then by name.
func varOrder(n string) string {
	if strings.HasPrefix(n, "_") {
		return "1" + n
	}
	return "0" + n
}

type gen struct {
	p       *ir.Program
	prof    profile.Profile
	melsec  bool
	vars    map[string]string // declared name -> type (melsec only; iec uses symbols)
	pre     []string          // edge-detector calls for the current rung
	stmts   []string
	tmp     int
	rungIdx int
}

func (g *gen) declare(name, typ string) {
	if _, ok := g.vars[name]; !ok {
		g.vars[name] = typ
	}
}

// ref returns the ST spelling of an operand used as a value.
func (g *gen) ref(op string) (string, error) {
	o, err := g.prof.Resolve(op, g.p)
	if err != nil {
		return "", err
	}
	if !g.melsec {
		if s := g.p.Symbol(op); s != nil {
			g.declare(op, s.Type)
		}
		switch o.Class {
		case profile.TimerInst, profile.CounterInst:
			return op + ".Q", nil
		}
		return op, nil
	}
	switch o.Class {
	case profile.Const:
		if op[0] == 'H' {
			return "16#" + op[1:], nil
		}
		return strconv.FormatInt(o.Value, 10), nil
	case profile.TimerInst:
		g.declare(op, "TON")
		return op + ".Q", nil
	case profile.CounterInst:
		g.declare(op, "CTU")
		return op + ".Q", nil
	case profile.Word:
		g.declare(op, "INT")
		return op, nil
	}
	if o.BitOfWord {
		word, _, _ := strings.Cut(op, ".")
		g.declare(word, "WORD")
		return op, nil
	}
	g.declare(op, "BOOL")
	return op, nil
}

// target returns the ST spelling of an operand being written.
func (g *gen) target(op string) (string, error) {
	r, err := g.ref(op)
	return strings.TrimSuffix(r, ".Q"), err
}

func and(a, b string) string {
	if a == "TRUE" {
		return b
	}
	return a + " AND " + b
}

// outputs writes statements for an Output node running under condition cond.
func (g *gen) outputs(n ir.Node, cond string) error {
	switch v := n.(type) {
	case *ir.Series:
		prefix, tail := v.Items[:len(v.Items)-1], v.Items[len(v.Items)-1]
		c, err := g.andExpr(prefix)
		if err != nil {
			return err
		}
		// c is AND terms with OR parts already parenthesized, so it can follow
		// another AND without extra parentheses.
		full := and(cond, c)
		if _, isBranch := tail.(*ir.Parallel); isBranch {
			g.tmp++
			t := fmt.Sprintf("_r%d_%d", g.rungIdx, g.tmp)
			g.declare(t, "BOOL")
			g.stmts = append(g.stmts, fmt.Sprintf("%s := %s;", t, full))
			full = t
		}
		return g.outputs(tail, full)
	case *ir.Parallel:
		for _, it := range v.Items {
			if err := g.outputs(it, cond); err != nil {
				return err
			}
		}
		return nil
	case ir.Element:
		return g.output(v, cond)
	}
	return fmt.Errorf("unexpected node %T", n)
}

func (g *gen) andExpr(items []ir.Node) (string, error) {
	parts := make([]string, len(items))
	for i, it := range items {
		s, err := g.expr(it)
		if err != nil {
			return "", err
		}
		if _, ok := it.(*ir.Parallel); ok {
			s = "(" + s + ")"
		}
		parts[i] = s
	}
	return strings.Join(parts, " AND "), nil
}

func (g *gen) expr(n ir.Node) (string, error) {
	switch v := n.(type) {
	case *ir.Series:
		return g.andExpr(v.Items)
	case *ir.Parallel:
		parts := make([]string, len(v.Items))
		for i, it := range v.Items {
			s, err := g.expr(it)
			if err != nil {
				return "", err
			}
			parts[i] = s
		}
		return strings.Join(parts, " OR "), nil
	case *ir.Contact:
		r, err := g.ref(v.Operand)
		if err != nil {
			return "", err
		}
		switch v.Mode {
		case ir.NC:
			return "NOT " + r, nil
		case ir.Rise, ir.Fall:
			fb, typ := "_rt_"+v.ID, "R_TRIG"
			if v.Mode == ir.Fall {
				fb, typ = "_ft_"+v.ID, "F_TRIG"
			}
			g.declare(fb, typ)
			g.pre = append(g.pre, fmt.Sprintf("%s(CLK := %s);", fb, r))
			return fb + ".Q", nil
		}
		return r, nil
	case *ir.Compare:
		l, err := g.ref(v.LHS)
		if err != nil {
			return "", err
		}
		r, err := g.ref(v.RHS)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("(%s %s %s)", l, v.Op, r), nil
	}
	return "", fmt.Errorf("unexpected condition %T", n)
}

func (g *gen) output(e ir.Element, cond string) error {
	emit := func(s string) { g.stmts = append(g.stmts, s) }
	ifThen := func(c, body string) {
		if c == "TRUE" {
			emit(body)
		} else {
			emit(fmt.Sprintf("IF %s THEN %s END_IF;", c, body))
		}
	}
	switch v := e.(type) {
	case *ir.Coil:
		t, err := g.target(v.Operand)
		if err != nil {
			return err
		}
		o, _ := g.prof.Resolve(v.Operand, g.p)
		switch v.Mode {
		case ir.Out:
			emit(fmt.Sprintf("%s := %s;", t, cond))
		case ir.Set:
			ifThen(cond, t+" := TRUE;")
		case ir.Reset:
			switch o.Class {
			case profile.TimerInst:
				ifThen(cond, t+"(IN := FALSE);")
			case profile.CounterInst:
				return fmt.Errorf("カウンタの RST はまだ ST に書き出せません")
			case profile.Word:
				ifThen(cond, t+" := 0;")
			default:
				ifThen(cond, t+" := FALSE;")
			}
		case ir.Pulse, ir.PulseFall:
			fb, typ := "_pls_"+v.ID, "R_TRIG"
			if v.Mode == ir.PulseFall {
				fb, typ = "_plf_"+v.ID, "F_TRIG"
			}
			g.declare(fb, typ)
			emit(fmt.Sprintf("%s(CLK := %s);", fb, cond))
			emit(fmt.Sprintf("%s := %s.Q;", t, fb))
		}
	case *ir.Timer:
		t, err := g.target(v.Instance)
		if err != nil {
			return err
		}
		if !g.melsec {
			g.declare(t, v.Kind)
		}
		pt, err := g.timerPreset(v.Preset)
		if err != nil {
			return err
		}
		emit(fmt.Sprintf("%s(IN := %s, PT := %s);", t, cond, pt))
	case *ir.Counter:
		t, err := g.target(v.Instance)
		if err != nil {
			return err
		}
		pv, err := g.ref(v.Preset)
		if err != nil {
			return err
		}
		emit(fmt.Sprintf("%s(CU := %s, PV := %s);", t, cond, pv))
	case *ir.Instr:
		spec, ok := g.prof.Instr(v.Name)
		if !ok {
			return fmt.Errorf("命令 %s は使えません", v.Name)
		}
		args := make([]string, len(v.Args))
		for i, a := range v.Args {
			r, err := g.ref(a)
			if err != nil {
				return err
			}
			args[i] = r
		}
		var stmt string
		switch spec.Op {
		case "MOV":
			stmt = fmt.Sprintf("%s := %s;", args[1], args[0])
		case "ADD", "SUB", "MUL", "DIV":
			sym := map[string]string{"ADD": "+", "SUB": "-", "MUL": "*", "DIV": "/"}[spec.Op]
			stmt = fmt.Sprintf("%s := %s %s %s;", args[2], args[0], sym, args[1])
		case "INC":
			stmt = fmt.Sprintf("%s := %s + 1;", args[0], args[0])
		case "DEC":
			stmt = fmt.Sprintf("%s := %s - 1;", args[0], args[0])
		default:
			return fmt.Errorf("命令 %s はまだ ST に書き出せません", v.Name)
		}
		if spec.Pulse {
			fb := "_p_" + v.ID
			g.declare(fb, "R_TRIG")
			emit(fmt.Sprintf("%s(CLK := %s);", fb, cond))
			ifThen(fb+".Q", stmt)
		} else {
			ifThen(cond, stmt)
		}
	default:
		return fmt.Errorf("出力ではない要素 %T", e)
	}
	return nil
}

// timerPreset converts a preset to a TIME expression. MELSEC T timers count
// in 100 ms units by default, so K50 is 5 s.
func (g *gen) timerPreset(op string) (string, error) {
	if !g.melsec {
		return op, nil
	}
	o, err := g.prof.Resolve(op, g.p)
	if err != nil {
		return "", err
	}
	if o.Class == profile.Const {
		return fmt.Sprintf("T#%dms", o.Value*100), nil
	}
	r, err := g.ref(op)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("INT_TO_TIME(%s * 100)", r), nil
}
