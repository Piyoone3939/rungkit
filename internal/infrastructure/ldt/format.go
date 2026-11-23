package ldt

import (
	"fmt"
	"strconv"
	"strings"

	"rungkit/internal/domain/ladder"
)

// Format writes p in canonical .ldt form. Format(Parse(x)) is stable: running
// it again gives the same text.
func Format(p *ladder.Program) (string, error) { return formatter{}.program(p) }

// FormatWithIDs writes p like Format but tags every element with its ID
// ("X0@e3", "OUT(Y0)@e4") and every rung with "// r1", so a tool can point
// at them. The parser accepts and ignores the tags.
func FormatWithIDs(p *ladder.Program) (string, error) { return formatter{ids: true}.program(p) }

// FormatRung writes one rung body (without the rung header).
func FormatRung(body ladder.Node) string { return formatter{}.rung(body, 0) }

// FormatRungWithIDs writes one rung body with element IDs.
func FormatRungWithIDs(body ladder.Node) string { return formatter{ids: true}.rung(body, 0) }

// FormatNode writes any node: a condition, an output item or a branch.
func FormatNode(n ladder.Node) string {
	f := formatter{}
	if p, err := ladder.Classify(n); err == nil && p == ladder.Output {
		return f.rung(n, 0)
	}
	return f.cond(n)
}

type formatter struct{ ids bool }

func (f formatter) program(p *ladder.Program) (string, error) {
	var b strings.Builder
	prof := p.Profile
	if prof == "" {
		prof = DefaultProfile
	}
	fmt.Fprintf(&b, "profile %s\n", prof)
	if len(p.Symbols) > 0 {
		b.WriteString("\n")
	}
	for _, s := range p.Symbols {
		b.WriteString("sym " + s.Name)
		if s.Type != "" {
			b.WriteString(" : " + s.Type)
		}
		if s.Address != "" {
			b.WriteString(" @ " + s.Address)
		}
		if s.Comment != "" {
			b.WriteString(" " + strconv.Quote(s.Comment))
		}
		b.WriteString("\n")
	}
	for _, r := range p.Rungs {
		b.WriteString("\nrung")
		if r.Label != "" {
			b.WriteString(" " + r.Label)
		}
		if r.Comment != "" {
			b.WriteString(" " + strconv.Quote(r.Comment))
		}
		b.WriteString(" {")
		if f.ids {
			b.WriteString(" // " + r.ID)
		}
		b.WriteString("\n")
		body := ladder.Normalize(r.Body)
		if err := ladder.CheckRung(body); err != nil {
			return "", fmt.Errorf("rung %s: %w", r.ID, err)
		}
		b.WriteString(f.rung(body, 1) + "\n}\n")
	}
	return b.String(), nil
}

// rung writes a rung body. A rail-level output branch is written one item per
// line, because the rung braces already act as the block.
func (f formatter) rung(body ladder.Node, depth int) string {
	body = ladder.Normalize(body)
	if par, ok := body.(*ladder.Parallel); ok {
		if p, err := ladder.Classify(par); err == nil && p == ladder.Output {
			lines := make([]string, len(par.Items))
			for i, it := range par.Items {
				lines[i] = indent(depth) + f.out(it, depth)
			}
			return strings.Join(lines, "\n")
		}
	}
	return indent(depth) + f.out(body, depth)
}

func indent(n int) string { return strings.Repeat("  ", n) }

func (f formatter) tag(s string, e ladder.Element) string {
	if f.ids && e.ElemID() != "" {
		return s + "@" + e.ElemID()
	}
	return s
}

// out writes an Output node.
func (f formatter) out(n ladder.Node, depth int) string {
	switch v := n.(type) {
	case *ladder.Series:
		prefix, tail := v.Items[:len(v.Items)-1], v.Items[len(v.Items)-1]
		return f.and(prefix) + " -> " + f.out(tail, depth)
	case *ladder.Parallel:
		var b strings.Builder
		b.WriteString("{\n")
		for _, it := range v.Items {
			b.WriteString(indent(depth+1) + f.out(it, depth+1) + "\n")
		}
		b.WriteString(indent(depth) + "}")
		return b.String()
	case *ladder.Coil:
		return f.tag(fmt.Sprintf("%s(%s)", v.Mode, v.Operand), v)
	case *ladder.Timer:
		return f.tag(fmt.Sprintf("%s(%s, %s)", v.Kind, v.Instance, v.Preset), v)
	case *ladder.Counter:
		return f.tag(fmt.Sprintf("%s(%s, %s)", v.Kind, v.Instance, v.Preset), v)
	case *ladder.Instr:
		return f.tag(fmt.Sprintf("%s(%s)", v.Name, strings.Join(v.Args, ", ")), v)
	}
	return f.cond(n)
}

// and writes condition items joined in series.
func (f formatter) and(items []ladder.Node) string {
	parts := make([]string, len(items))
	for i, it := range items {
		switch v := it.(type) {
		case *ladder.Parallel:
			parts[i] = "(" + f.or(v) + ")"
		case *ladder.Series:
			parts[i] = f.and(v.Items)
		default:
			parts[i] = f.cond(it)
		}
	}
	return strings.Join(parts, " & ")
}

func (f formatter) or(p *ladder.Parallel) string {
	parts := make([]string, len(p.Items))
	for i, it := range p.Items {
		switch v := it.(type) {
		case *ladder.Series:
			parts[i] = f.and(v.Items)
		case *ladder.Parallel:
			parts[i] = f.or(v)
		default:
			parts[i] = f.cond(it)
		}
	}
	return strings.Join(parts, " | ")
}

// cond writes a condition node.
func (f formatter) cond(n ladder.Node) string {
	switch v := n.(type) {
	case *ladder.Contact:
		s := v.Operand
		switch v.Mode {
		case ladder.NC:
			s = "!" + v.Operand
		case ladder.Rise:
			s = "rise(" + v.Operand + ")"
		case ladder.Fall:
			s = "fall(" + v.Operand + ")"
		}
		return f.tag(s, v)
	case *ladder.Compare:
		return f.tag(fmt.Sprintf("[%s %s %s]", v.LHS, v.Op, v.RHS), v)
	case *ladder.Series:
		return f.and(v.Items)
	case *ladder.Parallel:
		return f.or(v)
	}
	return fmt.Sprintf("<%T>", n)
}
