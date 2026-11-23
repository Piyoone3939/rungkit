package st

import (
	"strings"
	"testing"

	"rungkit/internal/infrastructure/ldt"
)

func body(t *testing.T, src string) string {
	t.Helper()
	p, _, err := ldt.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := Export(p, "Main")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	_, after, _ := strings.Cut(out, "END_VAR\n\n")
	return strings.TrimSpace(strings.TrimSuffix(after, "END_PROGRAM\n"))
}

func TestMelsecRungs(t *testing.T) {
	cases := []struct{ body, want string }{
		{`(X0 | Y0) & !X1 -> OUT(Y0)`, "Y0 := (X0 OR Y0) AND NOT X1;"},
		{`X0 & X1 | X2 -> OUT(Y0)`, "Y0 := (X0 AND X1 OR X2);"},
		// later branches read the saved condition, not a re-evaluated one
		{`M5 -> { RST(M5)  M1 -> SET(M10) }`,
			"_r1_1 := M5;\nIF _r1_1 THEN M5 := FALSE; END_IF;\nIF _r1_1 AND M1 THEN M10 := TRUE; END_IF;"},
		{`rise(X0) -> PLS(M0)`,
			"_rt_e1(CLK := X0);\n_pls_e2(CLK := _rt_e1.Q);\nM0 := _pls_e2.Q;"},
		{`X0 -> TON(T0, K50)`, "T0(IN := X0, PT := T#5000ms);"},
		{`X0 -> TON(T0, D1)`, "T0(IN := X0, PT := INT_TO_TIME(D1 * 100));"},
		{`T0 -> CTU(C0, K3)`, "C0(CU := T0.Q, PV := 3);"},
		{`[D0 >= H10] -> ADD(D0, K-1, D1)`, "IF (D0 >= 16#10) THEN D1 := D0 + -1; END_IF;"},
		{`X0 -> MOVP(K1, D0)`, "_p_e2(CLK := X0);\nIF _p_e2.Q THEN D0 := 1; END_IF;"},
		{`OUT(Y0)`, "Y0 := TRUE;"},
		{`X0 -> RST(T0)`, "IF X0 THEN T0(IN := FALSE); END_IF;"},
	}
	for _, c := range cases {
		if got := body(t, "rung { "+c.body+" }"); got != c.want {
			t.Errorf("%s:\ngot\n%s\nwant\n%s", c.body, got, c.want)
		}
	}
}

func TestDeclarations(t *testing.T) {
	p, _, _ := ldt.Parse("sym X0 \"start\"\nrung { X0 & D0.3 -> TON(T0, K5) }\n")
	out, err := Export(p, "Conv")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"PROGRAM Conv", "D0 : WORD;", "T0 : TON;", "X0 : BOOL; (* start *)"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
}

func TestIEC(t *testing.T) {
	src := "profile iec\nsym Start : BOOL @ %IX0.0\nsym Motor : BOOL\nsym T1 : TON\nrung { Start & !T1 -> { OUT(Motor)  TON(T1, T#5s) } }\n"
	want := "_r1_1 := Start AND NOT T1.Q;\nMotor := _r1_1;\nT1(IN := _r1_1, PT := T#5s);"
	if got := body(t, src); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestCounterResetUnsupported(t *testing.T) {
	p, _, _ := ldt.Parse("rung { X0 -> RST(C0) }\n")
	if _, err := Export(p, "Main"); err == nil {
		t.Fatal("want error")
	}
}
