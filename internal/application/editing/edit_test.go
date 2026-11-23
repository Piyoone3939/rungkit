package editing

import (
	"strings"
	"testing"

	"rungkit/internal/infrastructure/ldt"
)

func session(t *testing.T, src string) *Session {
	t.Helper()
	p, _, err := ldt.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	return NewSession(p, ldt.Notation{})
}

func apply(t *testing.T, s *Session, ops ...Op) *Result {
	t.Helper()
	res, err := s.Apply(ops)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// body returns rung i's text without IDs.
func body(s *Session, i int) string {
	return ldt.FormatRung(s.Program().Rungs[i].Body)
}

func str(s string) *string { return &s }

// The self-hold rung: (X0@e1 | Y0@e2) & !X1@e3 -> OUT(Y0)@e4
const selfHold = "rung { (X0 | Y0) & !X1 -> OUT(Y0) }\n"

func TestElementEdits(t *testing.T) {
	cases := []struct {
		name string
		op   Op
		want string
	}{
		{"series after", Op{Op: "insert_series", Target: "e3", Position: "after", Text: "!X2"},
			"(X0 | Y0) & !X1 & !X2 -> OUT(Y0)"},
		{"series before inside OR wraps", Op{Op: "insert_series", Target: "e1", Position: "before", Text: "M0"},
			"(M0 & X0 | Y0) & !X1 -> OUT(Y0)"},
		{"parallel joins the existing OR", Op{Op: "insert_parallel", Target: "e2", Text: "M5"},
			"(X0 | Y0 | M5) & !X1 -> OUT(Y0)"},
		{"parallel over a range", Op{Op: "insert_parallel", Target: "e1", To: "e3", Text: "M9"},
			"((X0 | Y0) & !X1 | M9) -> OUT(Y0)"},
		{"parallel output", Op{Op: "insert_parallel", Target: "e4", Text: "M1 -> SET(M10)"},
			"(X0 | Y0) & !X1 -> {\n  OUT(Y0)\n  M1 -> SET(M10)\n}"},
		{"replace element", Op{Op: "replace_element", Target: "e3", Text: "[D0 < K10]"},
			"(X0 | Y0) & [D0 < K10] -> OUT(Y0)"},
		{"delete in OR unwraps", Op{Op: "delete_element", Target: "e2"},
			"X0 & !X1 -> OUT(Y0)"},
		{"set operand", Op{Op: "set_operand", Target: "e4", Operand: "Y10"},
			"(X0 | Y0) & !X1 -> OUT(Y10)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := session(t, selfHold)
			apply(t, s, c.op)
			if got := body(s, 0); got != c.want {
				t.Errorf("got\n%s\nwant\n%s", got, c.want)
			}
		})
	}
}

func TestRungEdits(t *testing.T) {
	s := session(t, "rung \"a\" { X0 -> OUT(Y0) }\nrung \"b\" { X1 -> OUT(Y1) }\n")
	res := apply(t, s,
		Op{Op: "add_rung", Text: "X2 -> OUT(Y2)", Comment: str("c")},
		Op{Op: "add_rung", Text: "X3 -> OUT(Y3)", After: "start", Label: str("TOP")},
		Op{Op: "move_rung", Rung: "r1", After: "r3"},
		Op{Op: "set_rung", Rung: "r2", Comment: str("B")},
	)
	var order []string
	for _, r := range s.Program().Rungs {
		order = append(order, r.ID+":"+r.Comment)
	}
	if got := strings.Join(order, " "); got != "r4: r2:B r3:c r1:a" {
		t.Errorf("order %s", got)
	}
	if s.Program().Rungs[0].Label != "TOP" {
		t.Error("label")
	}
	if len(res.Rungs) != 4 || res.Rungs[0].ID != "r3" || res.Rungs[0].Text != "X2@e5 -> OUT(Y2)@e6" {
		t.Errorf("views %+v", res.Rungs)
	}
	if strings.Join(res.NewElements, " ") != "e7 e8 e5 e6" { // document order
		t.Errorf("new %v", res.NewElements)
	}
	res = apply(t, s, Op{Op: "delete_rung", Rung: "r2"}, Op{Op: "replace_rung", Rung: "r1", Text: "{ X0 -> OUT(Y0)  X9 -> OUT(Y9) }"})
	if len(s.Program().Rungs) != 3 || res.DeletedRungs[0] != "r2" {
		t.Errorf("delete %+v", res)
	}
	if got := body(s, 2); got != "X0 -> OUT(Y0)\nX9 -> OUT(Y9)" {
		t.Errorf("replace_rung: %q", got)
	}
}

func TestAtomicAndShapeErrors(t *testing.T) {
	cases := []struct {
		ops  []Op
		want string
	}{
		{[]Op{{Op: "insert_series", Target: "e4", Position: "after", Text: "X5"}}, "E_OUTPUT_NOT_TERMINAL"},
		{[]Op{{Op: "insert_parallel", Target: "e3", Text: "OUT(Y5)"}}, "E_MIXED_PARALLEL"},
		{[]Op{{Op: "delete_element", Target: "e4"}}, "E_NO_OUTPUT"},
		{[]Op{{Op: "add_rung", Text: "X0 & X1"}}, "E_NO_OUTPUT"},
		{[]Op{{Op: "set_operand", Target: "e4", Operand: "Y1"}, {Op: "delete_element", Target: "e99"}}, "ops[1] delete_element: 要素 e99 はありません"},
		{[]Op{{Op: "insert_parallel", Target: "e3", To: "e1", Text: "M0"}}, "同じ直列"},
		{[]Op{{Op: "insert_series", Target: "e1", Position: "inside", Text: "M0"}}, "before か after"},
		{[]Op{{Op: "replace_element", Target: "e1", Text: "X0 &"}}, "text を読めません"},
		{[]Op{{Op: "move_rung", Rung: "r1", After: "r1"}}, "自分自身"},
		{[]Op{{Op: "set_rung", Rung: "r1"}}, "comment か label"},
		{[]Op{{Op: "set_operand", Target: "e4"}}, "operand が必要"},
		{[]Op{{Op: "explode"}}, "操作 \"explode\" はありません"},
		{nil, "操作がありません"},
	}
	for _, c := range cases {
		s := session(t, selfHold)
		before := body(s, 0)
		_, err := s.Apply(c.ops)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%+v: got %v, want %q", c.ops, err, c.want)
		}
		if body(s, 0) != before || s.Dirty() {
			t.Errorf("%+v: program changed after a failed edit", c.ops)
		}
	}
}

func TestDeleteOnlyElement(t *testing.T) {
	s := session(t, "rung { OUT(Y0) }\n")
	if _, err := s.Apply([]Op{{Op: "delete_element", Target: "e1"}}); err == nil || !strings.Contains(err.Error(), "delete_rung") {
		t.Fatal(err)
	}
}

func TestDiagnosticsAreReportedNotRejected(t *testing.T) {
	s := session(t, selfHold)
	res := apply(t, s, Op{Op: "set_operand", Target: "e4", Operand: "X5"})
	if len(res.Diagnostics) != 1 || res.Diagnostics[0].Code != "E_READ_ONLY" {
		t.Fatalf("%+v", res.Diagnostics)
	}
}

func TestUndoRedoAndIDs(t *testing.T) {
	s := session(t, selfHold)
	apply(t, s, Op{Op: "insert_series", Target: "e3", Position: "after", Text: "!X2"}) // e5
	if err := s.Undo(); err != nil {
		t.Fatal(err)
	}
	if body(s, 0) != "(X0 | Y0) & !X1 -> OUT(Y0)" {
		t.Fatal("undo")
	}
	if err := s.Redo(); err != nil {
		t.Fatal(err)
	}
	if body(s, 0) != "(X0 | Y0) & !X1 & !X2 -> OUT(Y0)" {
		t.Fatal("redo")
	}
	s.Undo()
	res := apply(t, s, Op{Op: "insert_series", Target: "e3", Position: "after", Text: "!X3"})
	// the undone e5 is never handed out again
	if res.NewElements[0] != "e6" {
		t.Errorf("new id %v", res.NewElements)
	}
	if err := s.Redo(); err == nil {
		t.Error("a new edit clears redo")
	}
	s.Undo()
	if err := s.Undo(); err == nil {
		t.Error("nothing more to undo")
	}
}

func TestSymbols(t *testing.T) {
	s := session(t, "profile iec\nsym A : BOOL\nrung { A -> OUT(%QX0.0) }\n")
	res := apply(t, s,
		Op{Op: "set_symbol", Name: "B", Type: "bool", Address: "%IX0.1", Comment: str("ボタン")},
		Op{Op: "insert_series", Target: "e1", Position: "after", Text: "B"},
		Op{Op: "set_symbol", Name: "A", Type: "BOOL", Comment: str("A")},
	)
	if len(res.Diagnostics) != 0 {
		t.Fatalf("%+v", res.Diagnostics)
	}
	out, _ := ldt.Format(s.Program())
	if !strings.Contains(out, `sym A : BOOL "A"`) || !strings.Contains(out, `sym B : BOOL @ %IX0.1 "ボタン"`) {
		t.Errorf("%s", out)
	}
	res = apply(t, s, Op{Op: "delete_symbol", Name: "B"})
	if len(res.Diagnostics) != 1 || res.Diagnostics[0].Code != "E_BAD_OPERAND" {
		t.Errorf("%+v", res.Diagnostics)
	}
}

func TestAnnotatedTextRoundTrip(t *testing.T) {
	s := session(t, "rung \"x\" { (X0 | Y0) & !X1 -> { OUT(Y0)  rise(X2) -> MOV(K1, D0) } }\n")
	text, err := ldt.FormatWithIDs(s.Program())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "rung \"x\" { // r1\n  (X0@e1 | Y0@e2) & !X1@e3 -> {\n    OUT(Y0)@e4\n    rise(X2)@e5 -> MOV(K1, D0)@e6") {
		t.Fatalf("%s", text)
	}
	// pasting annotated text back is accepted (tags are ignored)
	apply(t, s, Op{Op: "replace_rung", Rung: "r1", Text: "X0@e1 & !X1@e3 -> OUT(Y0)@e4"})
	if body(s, 0) != "X0 & !X1 -> OUT(Y0)" {
		t.Errorf("%q", body(s, 0))
	}
}
