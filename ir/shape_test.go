package ir

import "testing"

func c(op string) *Contact { return &Contact{Mode: NO, Operand: op} }
func out(op string) *Coil  { return &Coil{Mode: Out, Operand: op} }

func TestNormalize(t *testing.T) {
	n := Normalize(&Series{Items: []Node{
		&Series{Items: []Node{c("X0"), &Series{Items: []Node{c("X1")}}}},
		&Parallel{Items: []Node{&Parallel{Items: []Node{c("X2"), c("X3")}}, c("X4")}},
		&Parallel{Items: nil},
		out("Y0"),
	}})
	s, ok := n.(*Series)
	if !ok || len(s.Items) != 4 {
		t.Fatalf("want flat series of 4, got %#v", n)
	}
	if p, ok := s.Items[2].(*Parallel); !ok || len(p.Items) != 3 {
		t.Fatalf("want flat parallel of 3, got %#v", s.Items[2])
	}
	if _, ok := Normalize(&Series{Items: []Node{c("X0")}}).(*Contact); !ok {
		t.Fatal("single item series should unwrap")
	}
	if Normalize(&Parallel{}) != nil {
		t.Fatal("empty parallel should vanish")
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		body Node
		code string
	}{
		{"ok", &Series{Items: []Node{c("X0"), out("Y0")}}, ""},
		{"output branch", &Series{Items: []Node{c("X0"), &Parallel{Items: []Node{out("Y0"), &Series{Items: []Node{c("M1"), out("Y1")}}}}}}, ""},
		{"output in middle", &Series{Items: []Node{out("Y0"), c("X0"), out("Y1")}}, "E_OUTPUT_NOT_TERMINAL"},
		{"mixed parallel", &Series{Items: []Node{c("X0"), &Parallel{Items: []Node{out("Y0"), c("X1")}}}}, "E_MIXED_PARALLEL"},
		{"no output", &Series{Items: []Node{c("X0"), c("X1")}}, "E_NO_OUTPUT"},
		{"not normalized", &Series{Items: []Node{out("Y0")}}, "E_NOT_NORMALIZED"},
		{"empty", nil, "E_EMPTY"},
	}
	for _, tc := range cases {
		err := CheckRung(tc.body)
		got := ""
		if err != nil {
			got = err.Code
		}
		if got != tc.code {
			t.Errorf("%s: got %q want %q", tc.name, got, tc.code)
		}
	}
}

func TestAssignIDsKeepsExisting(t *testing.T) {
	a, b := c("X0"), out("Y0")
	a.ID = "e7"
	p := &Program{Rungs: []*Rung{{ID: "r3", Body: &Series{Items: []Node{a, b}}}, {Body: out("Y1")}}}
	p.AssignIDs()
	if a.ID != "e7" || b.ID != "e8" || p.Rungs[1].ID != "r4" {
		t.Fatalf("got %s %s %s", a.ID, b.ID, p.Rungs[1].ID)
	}
	if id := p.NewElemID(); id != "e10" {
		t.Fatalf("next id %s", id)
	}
}

func TestJSONRejectsUnknown(t *testing.T) {
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
