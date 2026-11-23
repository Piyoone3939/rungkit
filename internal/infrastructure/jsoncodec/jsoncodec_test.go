package jsoncodec

import (
	"testing"

	"github.com/Piyoone3939/rungkit/internal/infrastructure/ldt"
)

func TestRoundTrip(t *testing.T) {
	src := "profile melsec-iqr\n\nsym X0 \"運転\"\n\nrung R1 \"c\" {\n  (X0 | Y0) & !X1 & rise(X2) & [D0 >= K1] -> {\n    OUT(Y0)\n    TON(T0, K5)\n    CTU(C0, K3)\n    MOV(K1, D1)\n    M0 -> RST(M1)\n  }\n}\n"
	p, _, err := ldt.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	q, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}
	out, err := ldt.Format(q)
	if err != nil {
		t.Fatal(err)
	}
	if out != src {
		t.Errorf("got\n%s", out)
	}
	if q.Rungs[0].ID != "r1" {
		t.Errorf("ids kept: %s", q.Rungs[0].ID)
	}
}

func TestRejectsUnknown(t *testing.T) {
	for _, js := range []string{
		`{"t":"wire"}`,
		`{"t":"contact","mode":"XX","operand":"X0"}`,
		`{"t":"coil","mode":"KEEP","operand":"Y0"}`,
		`{"t":"series","items":[{"t":"nope"}]}`,
	} {
		if _, err := UnmarshalNode([]byte(js)); err == nil {
			t.Errorf("%s: want error", js)
		}
	}
}
