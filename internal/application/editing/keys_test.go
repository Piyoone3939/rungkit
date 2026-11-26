package editing

import (
	"strings"
	"testing"
)

func TestKeysChainWithinOneCall(t *testing.T) {
	s := session(t, selfHold)
	res := apply(t, s,
		Op{Op: "insert_series", Target: "e3", Position: "after", Text: "!X3 & !X4", Key: "stops"},
		Op{Op: "insert_parallel", Target: "$stops.2", Text: "M1"},
		Op{Op: "add_rung", Text: "X5 -> OUT(Y5)", Key: "lamp"},
		Op{Op: "insert_series", Target: "$lamp", Position: "after", Text: "!X6"},
		Op{Op: "set_rung", Rung: "$lamp", Comment: str("lamp")},
	)
	if got := body(s, 0); got != "(X0 | Y0) & !X1 & !X3 & (!X4 | M1) -> OUT(Y0)" {
		t.Errorf("rung 0: %q", got)
	}
	if got := body(s, 1); got != "X5 & !X6 -> OUT(Y5)" || s.Program().Rungs[1].Comment != "lamp" {
		t.Errorf("rung 1: %q", got)
	}
	st, lamp := res.Keys["stops"], res.Keys["lamp"]
	if st == nil || st.Rung != "r1" || strings.Join(st.Elements, " ") != "e5 e6" {
		t.Errorf("stops %+v", st)
	}
	if lamp == nil || lamp.Rung != "r2" || len(lamp.Elements) != 2 {
		t.Errorf("lamp %+v", lamp)
	}
}

func TestKeysDropRemovedElements(t *testing.T) {
	s := session(t, selfHold)
	res := apply(t, s,
		Op{Op: "insert_series", Target: "e3", Position: "after", Text: "!X3 & !X4", Key: "k"},
		Op{Op: "delete_element", Target: "$k"},
	)
	if k := res.Keys["k"]; k == nil || strings.Join(k.Elements, " ") != "e6" {
		t.Errorf("k %+v", k)
	}
}

func TestKeyErrors(t *testing.T) {
	cases := []struct {
		ops  []Op
		want string
	}{
		{[]Op{{Op: "delete_element", Target: "e1", Key: "x"}}, "key"},
		{[]Op{{Op: "insert_series", Target: "e3", Position: "after", Text: "X3", Key: "bad key"}}, "key"},
		{[]Op{{Op: "insert_series", Target: "$nope", Position: "after", Text: "X3"}}, "$nope"},
		{[]Op{
			{Op: "insert_series", Target: "e3", Position: "after", Text: "X3", Key: "a"},
			{Op: "insert_series", Target: "$a.2", Position: "after", Text: "X4"},
		}, "$a.2"},
		{[]Op{
			{Op: "insert_series", Target: "e3", Position: "after", Text: "X3", Key: "a"},
			{Op: "insert_series", Target: "e1", Position: "after", Text: "X4", Key: "a"},
		}, "a"},
	}
	for _, c := range cases {
		s := session(t, selfHold)
		_, err := s.Apply(c.ops)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%+v: err %v", c.ops, err)
		}
		if body(s, 0) != "(X0 | Y0) & !X1 -> OUT(Y0)" {
			t.Errorf("not atomic: %q", body(s, 0))
		}
	}
}
