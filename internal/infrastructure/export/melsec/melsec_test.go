package melsec

import (
	"strings"
	"testing"

	"rungkit/internal/infrastructure/ldt"
)

func list(t *testing.T, body string) string {
	t.Helper()
	p, _, err := ldt.Parse("profile melsec-iqr\nrung {\n" + body + "\n}\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := Export(p)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	out = strings.TrimSuffix(out, "END\n")
	return strings.TrimSpace(strings.ReplaceAll(out, "\t", " "))
}

func TestExport(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"self hold", `(X0 | Y0) & !X1 -> OUT(Y0)`,
			"LD X0\nOR Y0\nANI X1\nOUT Y0"},
		{"or of series blocks", `(X0 & X1 | X2 & X3) & !X4 -> OUT(Y0)`,
			"LD X0\nAND X1\nLD X2\nAND X3\nORB\nANI X4\nOUT Y0"},
		{"and of parallel block", `X0 & (X1 | X2 & X3) -> OUT(Y0)`,
			"LD X0\nLD X1\nLD X2\nAND X3\nORB\nANB\nOUT Y0"},
		{"edges and compare", `rise(X0) & fall(X1) | [D0 <> K5] -> OUT(Y0)`,
			"LDP X0\nANDF X1\nOR<> D0 K5\nOUT Y0"},
		{"nc and edge in or", `X0 | !X1 | rise(X2) -> OUT(Y0)`,
			"LD X0\nORI X1\nORP X2\nOUT Y0"},
		{"branch needs no stack", `X0 -> { OUT(Y0)  M1 -> SET(M10) }`,
			"LD X0\nOUT Y0\nAND M1\nSET M10"},
		{"branch with three conditioned items", `X0 -> { M1 -> OUT(Y1)  M2 -> OUT(Y2)  M3 -> OUT(Y3) }`,
			"LD X0\nMPS\nAND M1\nOUT Y1\nMRD\nAND M2\nOUT Y2\nMPP\nAND M3\nOUT Y3"},
		{"restore before plain output", `X0 -> { M1 -> OUT(Y1)  OUT(Y2)  OUT(Y3) }`,
			"LD X0\nMPS\nAND M1\nOUT Y1\nMPP\nOUT Y2\nOUT Y3"},
		{"nested branch", `X0 -> { M1 -> { OUT(Y1)  M2 -> OUT(Y2) }  OUT(Y3) }`,
			"LD X0\nMPS\nAND M1\nOUT Y1\nAND M2\nOUT Y2\nMPP\nOUT Y3"},
		{"parallel condition inside branch", `X0 -> { OUT(Y0)  (M1 | M2) -> SET(M10) }`,
			"LD X0\nOUT Y0\nLD M1\nOR M2\nANB\nSET M10"},
		{"rail level branch", `X0 -> OUT(Y0)
		X1 -> OUT(Y1)`,
			"LD X0\nOUT Y0\nLD X1\nOUT Y1"},
		{"unconditional", `OUT(Y0)`, "LD SM400\nOUT Y0"},
		{"timer counter instr", `X0 -> { TON(T0, K50)  CTU(C0, D10)  ADD(D0, K1, D1)  MOVP(H0F, D2) }`,
			"LD X0\nOUT T0 K50\nOUT C0 D10\n+ D0 K1 D1\nMOVP H0F D2"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := list(t, c.body); got != c.want {
				t.Errorf("got\n%s\nwant\n%s", got, c.want)
			}
		})
	}
}

func TestExportRefusesErrors(t *testing.T) {
	p, _, err := ldt.Parse("profile melsec-iqr\nrung { X0 -> OUT(X1) }\n")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Export(p); err == nil {
		t.Fatal("want error for writing an input")
	}
}

func TestExportRefusesOtherProfile(t *testing.T) {
	p, _, err := ldt.Parse("profile iec\nsym A : BOOL\nrung { A -> OUT(A) }\n")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Export(p); err == nil {
		t.Fatal("want error for iec profile")
	}
}

func TestRungComment(t *testing.T) {
	p, _, _ := ldt.Parse("rung \"始動\" { X0 -> OUT(Y0) }\n")
	out, err := Export(p)
	if err != nil {
		t.Fatal(err)
	}
	if want := "; 始動\nLD\tX0\nOUT\tY0\nEND\n"; out != want {
		t.Errorf("got %q want %q", out, want)
	}
}
