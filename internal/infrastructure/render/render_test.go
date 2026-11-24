package render

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/Piyoone3939/rungkit/internal/infrastructure/ldt"
)

func draw(t *testing.T, body string) string {
	t.Helper()
	p, _, err := ldt.Parse("rung { " + body + " }")
	if err != nil {
		t.Fatal(err)
	}
	return DrawASCII(p.Rungs[0].Body)
}

func TestDrawASCII(t *testing.T) {
	cases := []struct{ body, want string }{
		{`(X0 | Y0) & !X1 -> OUT(Y0)`, `
|    X0         X1         Y0    |
|----] [---+----]/[--------( )---|
|    Y0    |                     |
|----] [---+                     |
`},
		{`X0 -> { OUT(Y0)  rise(X2) -> SET(M10) }`, `
|    X0                    Y0    |
|----] [---+---------------( )---|
|          |    X2         M10   |
|          +----]P[--------(S)---|
`},
		{`(X0 & X1 | X2) & X3 -> TON(T0, K50)`, `
|    X0         X1         X3       T0 K50  |
|----] [--------] [---+----] [-------[TON]--|
|    X2               |                     |
|----] [--------------+                     |
`},
		{`OUT(Y0)`, `
|    Y0    |
|----( )---|
`},
	}
	for _, c := range cases {
		if got := draw(t, c.body); got != strings.TrimPrefix(c.want, "\n") {
			t.Errorf("%s:\ngot\n%s\nwant\n%s", c.body, got, c.want)
		}
	}
}

func TestSVG(t *testing.T) {
	p, _, err := ldt.Parse("sym X0 \"起動 <A>\"\nrung \"自己保持\" { (X0 | Y0) & !X1 & [D0 >= K1] -> { OUT(Y0)  TON(T0, K50) } }\nrung { OUT(Y9) }\n")
	if err != nil {
		t.Fatal(err)
	}
	svg, err := SVG{}.Export(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := xml.Unmarshal([]byte(svg), new(struct{})); err != nil {
		t.Fatalf("not well-formed: %v\n%s", err, svg)
	}
	for _, want := range []string{`data-rung="r1"`, `data-elem="e1"`, `data-elem="e7"`, `X0 — 起動 &lt;A&gt;`, `自己保持`, `>TON<`} {
		if !strings.Contains(svg, want) {
			t.Errorf("svg lacks %s", want)
		}
	}
}

func TestASCIIExport(t *testing.T) {
	p, _, _ := ldt.Parse("rung R1 \"note\" { X0 -> OUT(Y0) }\n")
	out, _ := ASCII{}.Export(p)
	if !strings.HasPrefix(out, "r1 R1  note\n|") {
		t.Errorf("%q", out)
	}
}
