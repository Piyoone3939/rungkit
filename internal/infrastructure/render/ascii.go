package render

import (
	"strings"

	"github.com/Piyoone3939/rungkit/internal/domain/ladder"
)

// Symbol and label of an element, shared by both renderers.
func symbol(e ladder.Element) (sym, label string) {
	switch v := e.(type) {
	case *ladder.Contact:
		return map[ladder.ContactMode]string{ladder.NO: "] [", ladder.NC: "]/[", ladder.Rise: "]P[", ladder.Fall: "]F["}[v.Mode], v.Operand
	case *ladder.Compare:
		return "[" + v.Op + "]", v.LHS + " " + v.RHS
	case *ladder.Coil:
		return map[ladder.CoilMode]string{ladder.Out: "( )", ladder.Set: "(S)", ladder.Reset: "(R)", ladder.Pulse: "(P)", ladder.PulseFall: "(F)"}[v.Mode], v.Operand
	case *ladder.Timer:
		return "[" + v.Kind + "]", v.Instance + " " + v.Preset
	case *ladder.Counter:
		return "[" + v.Kind + "]", v.Instance + " " + v.Preset
	case *ladder.Instr:
		return "[" + v.Name + "]", strings.Join(v.Args, " ")
	}
	return "[?]", ""
}

// ASCII is the workspace exporter for text drawings.
type ASCII struct{}

// Name is the export format name.
func (ASCII) Name() string { return "ascii" }

// Export draws every rung of p.
func (ASCII) Export(p *ladder.Program) (string, error) {
	var b strings.Builder
	for i, r := range p.Rungs {
		if i > 0 {
			b.WriteString("\n")
		}
		head := r.ID
		if r.Label != "" {
			head += " " + r.Label
		}
		if r.Comment != "" {
			head += "  " + r.Comment
		}
		b.WriteString(head + "\n")
		b.WriteString(DrawASCII(r.Body))
	}
	return b.String(), nil
}

// DrawASCII draws one rung body, two text lines per grid row: the operand
// above, the symbol below.
func DrawASCII(body ladder.Node) string {
	l := Lay(body)
	cw := 11
	for _, bx := range l.Boxes {
		_, label := symbol(bx.Elem)
		cw = max(cw, len(label)+3)
		sym, _ := symbol(bx.Elem)
		cw = max(cw, len(sym)+4)
	}
	cw = min(cw, 21)
	lines := l.Rows * 2
	cols := l.Cols*cw + 1
	g := make([][]byte, lines)
	for i := range g {
		g[i] = []byte(strings.Repeat(" ", cols))
	}
	isRail := func(c int) bool { return c == 0 || c == l.Cols*cw }
	put := func(line, c int, ch byte) {
		if line < 0 || line >= lines || c < 0 || c >= cols || isRail(c) {
			return
		}
		if ch == '-' && (g[line][c] == '|' || g[line][c] == '+') {
			g[line][c] = '+'
			return
		}
		g[line][c] = ch
	}
	for i := range g { // rails
		g[i][0], g[i][cols-1] = '|', '|'
	}
	for _, v := range l.V {
		for line := 2*v.R1 + 1; line <= 2*v.R2+1; line++ {
			put(line, v.X*cw, '|')
		}
	}
	for _, h := range l.H {
		for c := h.X1 * cw; c <= h.X2*cw; c++ {
			put(2*h.Row+1, c, '-')
		}
	}
	for _, bx := range l.Boxes {
		sym, label := symbol(bx.Elem)
		line := 2*bx.Row + 1
		left := bx.Col * cw
		for c := left; c <= left+cw; c++ {
			put(line, c, '-')
		}
		start := left + (cw-len(sym))/2 + 1
		for i := 0; i < len(sym); i++ {
			put(line, start+i, sym[i])
		}
		if len(label) > cw-2 {
			label = label[:cw-3] + "~"
		}
		ls := left + (cw-len(label))/2 + 1
		for i := 0; i < len(label); i++ {
			put(line-1, ls+i, label[i])
		}
	}
	var b strings.Builder
	for _, row := range g {
		b.WriteString(strings.TrimRight(string(row), " ") + "\n")
	}
	return b.String()
}
