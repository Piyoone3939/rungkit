package ldt

import (
	"os"
	"strings"
	"testing"

	"rungkit/internal/domain/ladder"
)

func TestFormatCanonical(t *testing.T) {
	cases := []struct{ in, want string }{
		// extra parentheses and nesting disappear
		{`((X0)) & ((X1 & X2)) -> OUT(Y0)`, `X0 & X1 & X2 -> OUT(Y0)`},
		{`X0 | (X1 | X2) -> OUT(Y0)`, `(X0 | X1 | X2) -> OUT(Y0)`},
		// & binds tighter than |
		{`X0 & X1 | X2 -> OUT(Y0)`, `(X0 & X1 | X2) -> OUT(Y0)`},
		// chained arrows flatten into one series
		{`X0 -> X1 -> out(Y0)`, `X0 & X1 -> OUT(Y0)`},
		// a one-item block is just the item
		{`X0 -> { OUT(Y0) }`, `X0 -> OUT(Y0)`},
		// nested blocks with no condition flatten
		{`X0 -> { { OUT(Y0) OUT(Y1) } OUT(Y2) }`, "X0 -> {\n    OUT(Y0)\n    OUT(Y1)\n    OUT(Y2)\n  }"},
		// spacing is canonicalized
		{`[D0>=K10]&rise(X0)->MOV(K1,D1)`, `[D0 >= K10] & rise(X0) -> MOV(K1, D1)`},
	}
	for _, c := range cases {
		p, _, err := Parse("rung {\n" + c.in + "\n}\n")
		if err != nil {
			t.Fatalf("%s: %v", c.in, err)
		}
		got, err := Format(p)
		if err != nil {
			t.Fatal(err)
		}
		want := "profile melsec-iqr\n\nrung {\n  " + c.want + "\n}\n"
		if got != want {
			t.Errorf("%s:\ngot  %q\nwant %q", c.in, got, want)
		}
	}
}

func TestRoundTripExample(t *testing.T) {
	src, err := os.ReadFile("../../../examples/conveyor.ldt")
	if err != nil {
		t.Fatal(err)
	}
	p, _, err := Parse(string(src))
	if err != nil {
		t.Fatal(err)
	}
	out, err := Format(p)
	if err != nil {
		t.Fatal(err)
	}
	if out != string(src) {
		t.Errorf("example is not in canonical form:\n%s", out)
	}
}

func TestSymbolsAndLabels(t *testing.T) {
	src := `profile iec

sym Start : BOOL @ %IX0.0 "起動 \"押しボタン\""
sym Motor : BOOL

rung R1 "始動" {
  Start -> OUT(Motor)
}
`
	p, _, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if p.Rungs[0].Label != "R1" || p.Symbols[0].Address != "%IX0.0" || p.Symbols[0].Comment != `起動 "押しボタン"` {
		t.Fatalf("bad parse: %+v %+v", p.Rungs[0], p.Symbols[0])
	}
	out, _ := Format(p)
	if out != src {
		t.Errorf("got\n%s", out)
	}
}

func TestIDsAndPositions(t *testing.T) {
	p, sm, err := Parse("rung {\n  X0 & !X1 -> OUT(Y0)\n}\nrung {\n  X2 -> SET(M0)\n}\n")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, r := range p.Rungs {
		ids = append(ids, r.ID)
		for _, e := range ladder.Elements(r.Body) {
			ids = append(ids, e.ElemID())
		}
	}
	if got := strings.Join(ids, " "); got != "r1 e1 e2 e3 r2 e4 e5" {
		t.Errorf("ids = %s", got)
	}
	if pos := sm.Elems["e2"]; pos != (Pos{2, 8}) {
		t.Errorf("e2 at %v", pos)
	}
	if pos := sm.Elems["e5"]; pos != (Pos{5, 9}) {
		t.Errorf("e5 at %v", pos)
	}
}

func TestParseErrors(t *testing.T) {
	cases := []struct{ src, want string }{
		{"rung { X0 & -> OUT(Y0) }", "1:13"},
		{"rung { X0 }", "-> と出力"},
		{"rung { !(X0 | X1) -> OUT(Y0) }", "1つの接点"},
		{"rung { !fall(X1) -> OUT(Y0) }", "1つの接点"},
		{"rung { OUT(Y0) & X0 -> OUT(Y1) }", "接点・比較"},
		{"rung { X0 -> OUT(Y0)", "閉じていません"},
		{"rung { X0 -> OUT(Y0, Y1) }", "引数は 1 個"},
		{"rung { X0 -> TON(T0) }", "引数は 2 個"},
		{"rung { X0 & SET(M0) -> OUT(Y0) }", "-> の右側"},
		{"rung { X0 -> OUT(Y0) } $", "使えない文字"},
		{"rung \"abc { X0 -> OUT(Y0) }", "文字列が閉じていません"},
		{"sym A\nsym A", "2回"},
		{"profile a\nprofile b", "2回"},
		{"rung { }", "出力がありません"},
		{"foo", "profile / sym / rung"},
	}
	for _, c := range cases {
		_, _, err := Parse(c.src)
		if err == nil {
			t.Errorf("%q: want error", c.src)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q: error %q does not mention %q", c.src, err, c.want)
		}
	}
}

func TestLexWordsWithDashAndHash(t *testing.T) {
	p, _, err := Parse("rung { [D0 > K-5] -> MOV(K-1, D1) }\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := FormatRung(p.Rungs[0].Body); got != "[D0 > K-5] -> MOV(K-1, D1)" {
		t.Errorf("got %q", got)
	}
	p, _, err = Parse("profile iec\nsym T1 : TON\nsym A : BOOL\nrung { A->TON(T1, T#1s500ms) }\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := FormatRung(p.Rungs[0].Body); got != "A -> TON(T1, T#1s500ms)" {
		t.Errorf("got %q", got)
	}
}
