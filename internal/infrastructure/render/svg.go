package render

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/Piyoone3939/rungkit/internal/domain/ladder"
)

// SVG is the workspace exporter for SVG drawings. Colors come from
// currentColor, so the drawing follows the page's light or dark theme.
type SVG struct{}

// Name is the export format name.
func (SVG) Name() string { return "svg" }

const (
	cellW   = 96
	rowH    = 56
	marginX = 24
	titleH  = 28
	gapH    = 20
)

func esc(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

// Export draws every rung of p into one SVG. Each element is a <g> with
// data-elem set to its ID and a <title> with the device comment.
func (SVG) Export(p *ladder.Program) (string, error) {
	var body strings.Builder
	y, width := 0, 0
	for _, r := range p.Rungs {
		l := Lay(r.Body)
		head := r.ID
		if r.Label != "" {
			head += " " + r.Label
		}
		if r.Comment != "" {
			head += "  " + r.Comment
		}
		fmt.Fprintf(&body, `<g class="rung" data-rung="%s" transform="translate(0,%d)">`+"\n", esc(r.ID), y)
		fmt.Fprintf(&body, `<text class="rung-title" x="4" y="18">%s</text>`+"\n", esc(head))
		drawSVG(&body, p, l)
		body.WriteString("</g>\n")
		y += titleH + l.Rows*rowH + gapH
		width = max(width, 2*marginX+l.Cols*cellW)
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" class="ladder" width="%d" height="%d" viewBox="0 0 %d %d" font-family="ui-monospace, monospace" font-size="12">`+"\n",
		width, y, width, y)
	b.WriteString(`<style>.ladder line,.ladder path,.ladder rect{stroke:currentColor;fill:none;stroke-width:1.5}.ladder text{fill:currentColor;stroke:none}.ladder .rail{stroke-width:3}.ladder .rung-title{font-weight:600}.ladder .label{text-anchor:middle}.ladder .mark{text-anchor:middle;font-size:10}.ladder g.elem:hover rect.hit{fill:currentColor;fill-opacity:.08}</style>` + "\n")
	b.WriteString(body.String())
	b.WriteString("</svg>\n")
	return b.String(), nil
}

func drawSVG(b *strings.Builder, p *ladder.Program, l Layout) {
	X := func(x int) int { return marginX + x*cellW }
	Y := func(row int) int { return titleH + row*rowH + rowH/2 + 6 }
	line := func(x1, y1, x2, y2 int, class string) {
		fmt.Fprintf(b, `<line x1="%d" y1="%d" x2="%d" y2="%d"%s/>`+"\n", x1, y1, x2, y2, class)
	}
	top, bottom := Y(0)-rowH/2+6, Y(max(l.Rows, 1)-1)+rowH/2-6
	line(X(0), top, X(0), bottom, ` class="rail"`)
	line(X(l.Cols), top, X(l.Cols), bottom, ` class="rail"`)
	for _, v := range l.V {
		line(X(v.X), Y(v.R1), X(v.X), Y(v.R2), "")
	}
	for _, h := range l.H {
		line(X(h.X1), Y(h.Row), X(h.X2), Y(h.Row), "")
	}
	for _, bx := range l.Boxes {
		x0, x1, yc := X(bx.Col), X(bx.Col+1), Y(bx.Row)
		cx := (x0 + x1) / 2
		sym, label := symbol(bx.Elem)
		title := label
		if s := p.Symbol(firstWord(label)); s != nil && s.Comment != "" {
			title = label + " — " + s.Comment
		}
		fmt.Fprintf(b, `<g class="elem" data-elem="%s"><title>%s</title>`, esc(bx.Elem.ElemID()), esc(title))
		fmt.Fprintf(b, `<rect class="hit" x="%d" y="%d" width="%d" height="%d" stroke-opacity="0"/>`, x0, yc-rowH/2+4, cellW, rowH-8)
		fmt.Fprintf(b, `<text class="label" x="%d" y="%d">%s</text>`, cx, yc-14, esc(label))
		switch bx.Elem.(type) {
		case *ladder.Contact:
			line(x0, yc, cx-7, yc, "")
			line(cx+7, yc, x1, yc, "")
			line(cx-7, yc-10, cx-7, yc+10, "")
			line(cx+7, yc-10, cx+7, yc+10, "")
			switch sym {
			case "]/[":
				line(cx-10, yc+10, cx+10, yc-10, "")
			case "]P[":
				fmt.Fprintf(b, `<text class="mark" x="%d" y="%d">↑</text>`, cx, yc+4)
			case "]F[":
				fmt.Fprintf(b, `<text class="mark" x="%d" y="%d">↓</text>`, cx, yc+4)
			}
		case *ladder.Coil:
			line(x0, yc, cx-10, yc, "")
			line(cx+10, yc, x1, yc, "")
			fmt.Fprintf(b, `<path d="M%d %d q-8 10 0 20"/><path d="M%d %d q8 10 0 20"/>`, cx-6, yc-10, cx+6, yc-10)
			if m := strings.Trim(sym, "( )"); m != "" {
				fmt.Fprintf(b, `<text class="mark" x="%d" y="%d">%s</text>`, cx, yc+4, m)
			}
		default: // comparisons, timers, counters, instructions
			name := strings.Trim(sym, "[]")
			line(x0, yc, cx-28, yc, "")
			line(cx+28, yc, x1, yc, "")
			fmt.Fprintf(b, `<rect x="%d" y="%d" width="56" height="22" rx="3"/>`, cx-28, yc-11)
			fmt.Fprintf(b, `<text class="mark" x="%d" y="%d">%s</text>`, cx, yc+4, esc(name))
		}
		b.WriteString("</g>\n")
	}
}

func firstWord(s string) string {
	w, _, _ := strings.Cut(s, " ")
	return w
}
