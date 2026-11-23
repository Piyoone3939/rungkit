package ldt

import (
	"fmt"
	"strconv"
	"strings"

	"rungkit/ir"
)

// Format writes p in canonical .ldt form. Format(Parse(x)) is stable: running
// it again gives the same text.
func Format(p *ir.Program) (string, error) {
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
		b.WriteString(" {\n")
		body := ir.Normalize(r.Body)
		if err := ir.CheckRung(body); err != nil {
			return "", fmt.Errorf("rung %s: %w", r.ID, err)
		}
		if par, ok := body.(*ir.Parallel); ok {
			// A rail-level output branch: the rung braces act as the block.
			for _, it := range par.Items {
				b.WriteString("  " + formatOut(it, 1) + "\n")
			}
		} else {
			b.WriteString("  " + formatOut(body, 1) + "\n")
		}
		b.WriteString("}\n")
	}
	return b.String(), nil
}

// FormatRung writes one rung body (without the rung header) on one or more
// lines. Useful for tool results.
func FormatRung(body ir.Node) string {
	body = ir.Normalize(body)
	if par, ok := body.(*ir.Parallel); ok {
		var lines []string
		for _, it := range par.Items {
			lines = append(lines, formatOut(it, 0))
		}
		return strings.Join(lines, "\n")
	}
	return formatOut(body, 0)
}

func indent(n int) string { return strings.Repeat("  ", n) }

// formatOut writes an Output node.
func formatOut(n ir.Node, depth int) string {
	switch v := n.(type) {
	case *ir.Series:
		prefix, tail := v.Items[:len(v.Items)-1], v.Items[len(v.Items)-1]
		return andText(prefix) + " -> " + formatOut(tail, depth)
	case *ir.Parallel:
		var b strings.Builder
		b.WriteString("{\n")
		for _, it := range v.Items {
			b.WriteString(indent(depth+1) + formatOut(it, depth+1) + "\n")
		}
		b.WriteString(indent(depth) + "}")
		return b.String()
	case *ir.Coil:
		return fmt.Sprintf("%s(%s)", v.Mode, v.Operand)
	case *ir.Timer:
		return fmt.Sprintf("%s(%s, %s)", v.Kind, v.Instance, v.Preset)
	case *ir.Counter:
		return fmt.Sprintf("%s(%s, %s)", v.Kind, v.Instance, v.Preset)
	case *ir.Instr:
		return fmt.Sprintf("%s(%s)", v.Name, strings.Join(v.Args, ", "))
	}
	return condAtom(n)
}

// andText writes condition items joined in series.
func andText(items []ir.Node) string {
	parts := make([]string, len(items))
	for i, it := range items {
		switch v := it.(type) {
		case *ir.Parallel:
			parts[i] = "(" + orText(v) + ")"
		case *ir.Series:
			parts[i] = andText(v.Items)
		default:
			parts[i] = condAtom(it)
		}
	}
	return strings.Join(parts, " & ")
}

func orText(p *ir.Parallel) string {
	parts := make([]string, len(p.Items))
	for i, it := range p.Items {
		switch v := it.(type) {
		case *ir.Series:
			parts[i] = andText(v.Items)
		case *ir.Parallel:
			parts[i] = orText(v)
		default:
			parts[i] = condAtom(it)
		}
	}
	return strings.Join(parts, " | ")
}

func condAtom(n ir.Node) string {
	switch v := n.(type) {
	case *ir.Contact:
		switch v.Mode {
		case ir.NC:
			return "!" + v.Operand
		case ir.Rise:
			return "rise(" + v.Operand + ")"
		case ir.Fall:
			return "fall(" + v.Operand + ")"
		}
		return v.Operand
	case *ir.Compare:
		return fmt.Sprintf("[%s %s %s]", v.LHS, v.Op, v.RHS)
	case *ir.Series:
		return andText(v.Items)
	case *ir.Parallel:
		return "(" + orText(v) + ")"
	}
	return fmt.Sprintf("<%T>", n)
}
