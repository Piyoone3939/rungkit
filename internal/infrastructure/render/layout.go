// Package render draws ladder diagrams as text (ascii) and SVG.
//
// A rung is laid out on a grid: a series grows to the right, a parallel grows
// downward, and outputs are pushed to the right rail as in usual ladder
// drawings. Both renderers draw the same layout.
package render

import "github.com/Piyoone3939/rungkit/internal/domain/ladder"

// Box is an element in a grid cell.
type Box struct {
	Elem     ladder.Element
	Col, Row int
}

// HLine is a horizontal wire on a row from boundary X1 to boundary X2.
type HLine struct{ Row, X1, X2 int }

// VLine is a vertical wire on boundary X from row R1 to row R2.
type VLine struct{ X, R1, R2 int }

// Layout is a laid-out rung. Boundaries run from 0 (left rail) to Cols
// (right rail).
type Layout struct {
	Cols, Rows int
	Boxes      []Box
	H          []HLine
	V          []VLine
}

// Lay lays out a rung body.
func Lay(body ladder.Node) Layout {
	body = ladder.Normalize(body)
	l := Layout{Cols: width(body)}
	l.Rows = l.out(body, 0, 0, l.Cols)
	return l
}

func width(n ladder.Node) int {
	switch v := n.(type) {
	case *ladder.Series:
		w := 0
		for _, it := range v.Items {
			w += width(it)
		}
		return w
	case *ladder.Parallel:
		w := 0
		for _, it := range v.Items {
			w = max(w, width(it))
		}
		return w
	}
	return 1
}

// cond places a condition part with its top-left cell at (col, row) and
// returns its width and height.
func (l *Layout) cond(n ladder.Node, col, row int) (int, int) {
	switch v := n.(type) {
	case *ladder.Series:
		x, h := col, 0
		for _, it := range v.Items {
			iw, ih := l.cond(it, x, row)
			x += iw
			h = max(h, ih)
		}
		return x - col, h
	case *ladder.Parallel:
		w := width(v)
		r, last := row, row
		for _, it := range v.Items {
			iw, ih := l.cond(it, col, r)
			if iw < w {
				l.H = append(l.H, HLine{r, col + iw, col + w})
			}
			last = r
			r += ih
		}
		l.V = append(l.V, VLine{col, row, last}, VLine{col + w, row, last})
		return w, r - row
	case ladder.Element:
		l.Boxes = append(l.Boxes, Box{v, col, row})
		return 1, 1
	}
	return 0, 0
}

// out places an output part starting at (col, row); outputs end at the right
// rail R. It returns the height.
func (l *Layout) out(n ladder.Node, col, row, R int) int {
	switch v := n.(type) {
	case *ladder.Series:
		last := len(v.Items) - 1
		x, h := col, 0
		for _, it := range v.Items[:last] {
			iw, ih := l.cond(it, x, row)
			x += iw
			h = max(h, ih)
		}
		return max(h, l.out(v.Items[last], x, row, R))
	case *ladder.Parallel:
		r, lastRow := row, row
		for _, it := range v.Items {
			lastRow = r
			r += l.out(it, col, r, R)
		}
		l.V = append(l.V, VLine{col, row, lastRow})
		return r - row
	case ladder.Element:
		if col < R-1 {
			l.H = append(l.H, HLine{row, col, R - 1})
		}
		l.Boxes = append(l.Boxes, Box{v, R - 1, row})
		return 1
	}
	return 0
}
