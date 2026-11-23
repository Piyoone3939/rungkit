package check

import (
	"strings"
	"testing"

	"github.com/Piyoone3939/rungkit/internal/domain/ladder"
	"github.com/Piyoone3939/rungkit/internal/infrastructure/ldt"
)

func codes(t *testing.T, src string) string {
	t.Helper()
	p, _, err := ldt.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var cs []string
	for _, d := range Program(p) {
		cs = append(cs, d.Code)
	}
	return strings.Join(cs, " ")
}

func TestMelsec(t *testing.T) {
	cases := []struct{ body, want string }{
		{`(X0 | Y0) & !X1 -> OUT(Y0)`, ""},
		{`X1F & B1A & W1F.F -> OUT(Y1F)`, ""}, // hex devices, bit of word
		{`T0 & C0 & SM400 & D0.3 -> OUT(M0)`, ""},
		{`X0 -> OUT(X1)`, "E_READ_ONLY"},
		{`X0 -> OUT(SM400)`, "E_READ_ONLY"},
		{`X1G -> OUT(Y0)`, "E_BAD_OPERAND"},  // X is hexadecimal
		{`MA -> OUT(Y0)`, "E_BAD_OPERAND"},   // M is decimal
		{`x0 -> OUT(Y0)`, "E_BAD_OPERAND"},   // lower case
		{`M0.1 -> OUT(Y0)`, "E_BAD_OPERAND"}, // bit of a bit device
		{`D0.G -> OUT(Y0)`, "E_BAD_OPERAND"}, // bit number 0-F
		{`D0 -> OUT(Y0)`, "E_TYPE_MISMATCH"}, // word as contact
		{`X0 -> OUT(D0)`, "E_TYPE_MISMATCH"}, // word as coil
		{`X0 -> RST(D0)`, ""},                // RST clears words
		{`X0 -> RST(T0)`, ""},                // and timers
		{`[D0 >= K10] & [H1F < R5] -> OUT(Y0)`, ""},
		{`[X0 = K1] -> OUT(Y0)`, "E_TYPE_MISMATCH"},
		{`X0 -> TON(T0, K50)`, ""},
		{`X0 -> TON(T0, D5)`, ""},
		{`X0 -> TON(D0, K50)`, "E_TYPE_MISMATCH"},
		{`X0 -> TOF(T0, K50)`, "E_INSTR_UNSUPPORTED"},
		{`X0 -> CTU(C0, K10)`, ""},
		{`X0 -> MOV(K1, D0)`, ""},
		{`X0 -> MOV(K1, K2)`, "E_TYPE_MISMATCH"},
		{`X0 -> MOV(K1, SD0)`, "E_READ_ONLY"},
		{`X0 -> MOV(K1)`, "E_ARG_COUNT"},
		{`X0 -> BMOV(D0, D1, K3)`, "E_INSTR_UNSUPPORTED"},
		{`X0 -> ADDP(D0, K1, D0)`, ""},
		{`X0 -> MOV(K99999999999, D0)`, "E_BAD_OPERAND"},
	}
	for _, c := range cases {
		if got := codes(t, "rung { "+c.body+" }"); got != c.want {
			t.Errorf("%s: got %q want %q", c.body, got, c.want)
		}
	}
}

func TestWarnings(t *testing.T) {
	src := `
rung { X0 -> OUT(Y0) }
rung { X1 -> OUT(Y0) }
rung { X0 -> SET(M0) }
rung { X1 -> SET(M0) }
rung { X0 -> TON(T0, K10) }
rung { X1 -> TON(T0, K20) }
rung { X0 -> CTU(C0, K10) }
rung { X1 -> CTU(C0, K10) }
`
	if got := codes(t, src); got != "W_DOUBLE_COIL W_TIMER_REUSED W_COUNTER_REUSED" {
		t.Errorf("got %q", got)
	}
}

func TestIEC(t *testing.T) {
	head := "profile iec\nsym Start : BOOL @ %IX0.0\nsym Motor : BOOL\nsym N : INT\nsym T1 : TON\nsym C1 : CTU\n"
	cases := []struct{ body, want string }{
		{`(Start | Motor) & !%IX0.1 -> OUT(Motor)`, ""},
		{`Start -> OUT(Start)`, "E_READ_ONLY"},
		{`Start -> OUT(%QX0.0)`, ""},
		{`Start -> OUT(Lamp)`, "E_BAD_OPERAND"},
		{`Start -> TON(T1, T#5s)`, ""},
		{`Start -> TON(T1, 50)`, "E_TYPE_MISMATCH"},
		{`Start -> TOF(T1, T#5s)`, ""},
		{`T1 & C1 -> OUT(Motor)`, ""},
		{`[N >= 16#FF] -> MOV(0, N)`, ""},
		{`Start -> OUT(N)`, "E_TYPE_MISMATCH"},
	}
	for _, c := range cases {
		if got := codes(t, head+"rung { "+c.body+" }"); got != c.want {
			t.Errorf("%s: got %q want %q", c.body, got, c.want)
		}
	}
}

func TestShapeAndProfileErrors(t *testing.T) {
	p := &ladder.Program{Profile: "melsec-iqr", Rungs: []*ladder.Rung{{ID: "r1", Body: &ladder.Series{Items: []ladder.Node{
		&ladder.Coil{Base: ladder.Base{ID: "e1"}, Mode: ladder.Out, Operand: "Y0"},
		&ladder.Contact{Base: ladder.Base{ID: "e2"}, Mode: ladder.NO, Operand: "X0"},
		&ladder.Coil{Base: ladder.Base{ID: "e3"}, Mode: ladder.Out, Operand: "Y1"},
	}}}}}
	ds := Program(p)
	if len(ds) != 1 || ds[0].Code != "E_OUTPUT_NOT_TERMINAL" || ds[0].Elem != "e1" || ds[0].Rung != "r1" {
		t.Errorf("got %+v", ds)
	}
	p.Profile = "omron"
	if ds := Program(p); len(ds) != 1 || ds[0].Code != "E_UNKNOWN_PROFILE" {
		t.Errorf("got %+v", ds)
	}
}

func TestMelsecSymbolTypeIgnored(t *testing.T) {
	if got := codes(t, "sym Y2 : BOOL @ %QX0.2 \"lamp\"\nrung { X0 -> OUT(Y2) }"); got != "W_SYMBOL_TYPE_IGNORED" {
		t.Errorf("got %q", got)
	}
	if got := codes(t, "sym Y2 \"lamp\"\nrung { X0 -> OUT(Y2) }"); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestBadLabel(t *testing.T) {
	p := &ladder.Program{Profile: "melsec-iqr", Rungs: []*ladder.Rung{{ID: "r1", Label: "運転", Body: &ladder.Coil{Mode: ladder.Out, Operand: "Y0"}}}}
	ds := Program(p)
	if len(ds) != 1 || ds[0].Code != "E_BAD_LABEL" {
		t.Fatalf("%+v", ds)
	}
}
