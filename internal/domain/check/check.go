// Package check validates a program and reports problems with codes, so a
// person or an agent can fix them one by one.
package check

import (
	"fmt"

	"rungkit/internal/domain/ladder"
	"rungkit/internal/domain/profile"
)

// Severity of a diagnostic.
type Severity string

const (
	Error   Severity = "error"
	Warning Severity = "warning"
)

// Diagnostic is one problem.
type Diagnostic struct {
	Severity Severity `json:"severity"`
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Rung     string   `json:"rung,omitempty"`
	Elem     string   `json:"elem,omitempty"`
}

// HasErrors reports whether any diagnostic is an error.
func HasErrors(ds []Diagnostic) bool {
	for _, d := range ds {
		if d.Severity == Error {
			return true
		}
	}
	return false
}

// Program checks the whole program.
func Program(p *ladder.Program) []Diagnostic {
	prof, err := profile.Get(p.Profile)
	if err != nil {
		return []Diagnostic{{Severity: Error, Code: "E_UNKNOWN_PROFILE", Message: err.Error()}}
	}
	c := &checker{p: p, prof: prof, outCoils: map[string]string{}, timers: map[string]string{}, counters: map[string]string{}}
	seen := map[string]bool{}
	for _, s := range p.Symbols {
		if seen[s.Name] {
			c.add(Error, "E_DUPLICATE_SYMBOL", "", "", "sym %s が2回宣言されています", s.Name)
		}
		seen[s.Name] = true
	}
	for _, r := range p.Rungs {
		c.rung = r.ID
		if err := ladder.CheckRung(r.Body); err != nil {
			c.add(Error, err.Code, r.ID, err.Elem, "%s", err.Msg)
			continue
		}
		for _, e := range ladder.Elements(r.Body) {
			c.elem(e)
		}
	}
	return c.ds
}

type checker struct {
	p        *ladder.Program
	prof     profile.Profile
	rung     string
	ds       []Diagnostic
	outCoils map[string]string // operand -> first element ID
	timers   map[string]string
	counters map[string]string
}

func (c *checker) add(sev Severity, code, rung, elem, format string, args ...any) {
	c.ds = append(c.ds, Diagnostic{sev, code, fmt.Sprintf(format, args...), rung, elem})
}

func (c *checker) errAt(e ladder.Element, code, format string, args ...any) {
	c.add(Error, code, c.rung, e.ElemID(), format, args...)
}

// resolve reports a diagnostic and returns ok=false when op cannot be read.
func (c *checker) resolve(e ladder.Element, op string) (profile.Operand, bool) {
	o, err := c.prof.Resolve(op, c.p)
	if err != nil {
		c.errAt(e, "E_BAD_OPERAND", "%s", err.Error())
		return o, false
	}
	return o, true
}

func (c *checker) want(e ladder.Element, op string, o profile.Operand, role string, classes ...profile.Class) bool {
	for _, k := range classes {
		if o.Class == k {
			return true
		}
	}
	c.errAt(e, "E_TYPE_MISMATCH", "%s は%sです。%sには使えません", op, o.Class, role)
	return false
}

func (c *checker) elem(e ladder.Element) {
	switch v := e.(type) {
	case *ladder.Contact:
		if o, ok := c.resolve(e, v.Operand); ok {
			c.want(e, v.Operand, o, "接点", profile.Bit, profile.TimerInst, profile.CounterInst)
		}
	case *ladder.Compare:
		if !validOp(v.Op) {
			c.errAt(e, "E_BAD_COMPARE", "比較演算子 %q は使えません", v.Op)
		}
		for _, op := range []string{v.LHS, v.RHS} {
			if o, ok := c.resolve(e, op); ok {
				c.want(e, op, o, "比較", profile.Word, profile.Const)
			}
		}
	case *ladder.Coil:
		o, ok := c.resolve(e, v.Operand)
		if !ok {
			return
		}
		if v.Mode == ladder.Reset {
			if !c.want(e, v.Operand, o, "RST", profile.Bit, profile.Word, profile.TimerInst, profile.CounterInst) {
				return
			}
		} else if !c.want(e, v.Operand, o, string(v.Mode), profile.Bit) {
			return
		}
		if !o.Writable {
			c.errAt(e, "E_READ_ONLY", "%s は書き込めません（入力または読み出し専用）", v.Operand)
			return
		}
		if v.Mode == ladder.Out {
			if first, dup := c.outCoils[v.Operand]; dup {
				c.add(Warning, "W_DOUBLE_COIL", c.rung, e.ElemID(),
					"%s は %s でも OUT しています（ダブルコイル）。後ろの OUT の結果だけが残ります", v.Operand, first)
			} else {
				c.outCoils[v.Operand] = e.ElemID()
			}
		}
	case *ladder.Timer:
		if !contains(c.prof.TimerKinds(), v.Kind) {
			c.errAt(e, "E_INSTR_UNSUPPORTED", "%s ではタイマー %s は使えません（使えるもの: %v）", c.prof.Name(), v.Kind, c.prof.TimerKinds())
		}
		if o, ok := c.resolve(e, v.Instance); ok {
			c.want(e, v.Instance, o, "タイマーの指定", profile.TimerInst)
		}
		if o, ok := c.resolve(e, v.Preset); ok && !c.prof.TimerPresetOK(o) {
			c.errAt(e, "E_TYPE_MISMATCH", "%s はタイマーの設定値に使えません", v.Preset)
		}
		c.reuse(e, c.timers, v.Instance, "W_TIMER_REUSED", "タイマー")
	case *ladder.Counter:
		if v.Kind != "CTU" {
			c.errAt(e, "E_INSTR_UNSUPPORTED", "カウンタ %s にはまだ対応していません", v.Kind)
		}
		if o, ok := c.resolve(e, v.Instance); ok {
			c.want(e, v.Instance, o, "カウンタの指定", profile.CounterInst)
		}
		if o, ok := c.resolve(e, v.Preset); ok {
			c.want(e, v.Preset, o, "カウンタの設定値", profile.Const, profile.Word)
		}
		c.reuse(e, c.counters, v.Instance, "W_COUNTER_REUSED", "カウンタ")
	case *ladder.Instr:
		spec, ok := c.prof.Instr(v.Name)
		if !ok {
			c.errAt(e, "E_INSTR_UNSUPPORTED", "%s では命令 %s は使えません", c.prof.Name(), v.Name)
			return
		}
		if len(v.Args) != len(spec.Args) {
			c.errAt(e, "E_ARG_COUNT", "%s の引数は %d 個です（%d 個あります）", v.Name, len(spec.Args), len(v.Args))
			return
		}
		for i, a := range v.Args {
			o, ok := c.resolve(e, a)
			if !ok {
				continue
			}
			switch spec.Args[i] {
			case profile.Src:
				c.want(e, a, o, v.Name+" の入力", profile.Word, profile.Const)
			case profile.Dst:
				if c.want(e, a, o, v.Name+" の出力先", profile.Word) && !o.Writable {
					c.errAt(e, "E_READ_ONLY", "%s は書き込めません", a)
				}
			}
		}
	}
}

func (c *checker) reuse(e ladder.Element, seen map[string]string, inst, code, what string) {
	if first, dup := seen[inst]; dup {
		c.add(Warning, code, c.rung, e.ElemID(), "%s%s は %s でも使っています", what, inst, first)
		return
	}
	seen[inst] = e.ElemID()
}

func validOp(op string) bool { return contains(ladder.CompareOps, op) }

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if x == y {
			return true
		}
	}
	return false
}
