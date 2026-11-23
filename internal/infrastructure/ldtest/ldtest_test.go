package ldtest

import (
	"os"
	"strings"
	"testing"

	"rungkit/internal/application/scenario"
	"rungkit/internal/domain/profile"
	"rungkit/internal/infrastructure/ldt"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestExampleScenario(t *testing.T) {
	src, err := os.ReadFile("../../../examples/conveyor.ldt")
	must(t, err)
	p, _, err := ldt.Parse(string(src))
	must(t, err)
	sc, err := os.ReadFile("../../../examples/conveyor.ldtest")
	must(t, err)
	script, err := Parse(string(sc))
	must(t, err)
	results, err := scenario.Run(p, script)
	must(t, err)
	if len(results) != 5 {
		t.Fatalf("%d results", len(results))
	}
	for _, r := range results {
		if !r.Pass {
			t.Errorf("%s: line %d: %s", r.Name, r.Line, r.Msg)
		}
	}
}

func run(t *testing.T, prog, script string) []scenario.Result {
	t.Helper()
	p, _, err := ldt.Parse(prog)
	must(t, err)
	sc, err := Parse(script)
	must(t, err)
	rs, err := scenario.Run(p, sc)
	must(t, err)
	return rs
}

func TestScriptFailures(t *testing.T) {
	prog := "rung { X0 -> TON(T0, K10) }\nrung { T0 -> OUT(Y0) }"
	rs := run(t, prog, `scan 100ms
watch T0.ET Y0
test "ok" {
  set X0 on
  // measuring starts in the first scan, so the output shows one scan late
  wait Y0 on within 1100ms
  expect T0.ET 1s
}
test "too early" {
  set X0 on
  run 5
  expect Y0 on
}
test "timeout" {
  set X0 on
  wait Y0 on within 500ms
}
test "fresh state" {
  expect Y0 off
  expect T0.ET 0
}
`)
	want := []struct {
		pass bool
		line int
		msg  string
	}{
		{true, 0, ""},
		{false, 12, "expect Y0 on でしたが、off でした（スキャン 5, 500ms）"},
		{false, 16, "500ms 待っても Y0 は on になりませんでした"},
		{true, 0, ""},
	}
	for i, w := range want {
		r := rs[i]
		if r.Pass != w.pass || r.Line != w.line || !strings.Contains(r.Msg, w.msg) {
			t.Errorf("%s: got pass=%v line=%d msg=%q", r.Name, r.Pass, r.Line, r.Msg)
		}
	}
	if got := rs[0]; got.Scans != 11 || len(got.Trace) != 11 || got.Trace[10].Values[0] != 1000 {
		t.Errorf("trace of ok: scans=%d %+v", got.Scans, got.Trace)
	}
}

func TestScriptValues(t *testing.T) {
	rs := run(t, "rung { [D0 = H10] -> MOV(D0, D1) }", `test "values" {
  set D0 K16
  run 1
  expect D1 16
  expect D1 H10
  expect D1 16#10
  force D0 3
  run 1
  expect D1 16
  unforce D0
  set D0 16
  run 1
  expect D1 16
}`)
	if !rs[0].Pass {
		t.Fatal(rs[0].Msg)
	}
}

func TestScriptRuntimeError(t *testing.T) {
	rs := run(t, "rung { X0 -> DIV(K1, D0, D1) }", "test \"div\" {\n  set X0 on\n  run 1\n}\n")
	if rs[0].Pass || !strings.Contains(rs[0].Msg, "0 で割りました") {
		t.Fatalf("%+v", rs[0])
	}
}

func TestScriptSyntax(t *testing.T) {
	cases := []struct{ src, want string }{
		{"test \"a\" {\n  set X0\n}", "2: set の書き方"},
		{"test \"a\" {\n  jump 3\n}", "命令 \"jump\" はありません"},
		{"test \"a\" {\n  run 1\n", "} がありません"},
		{"test a", `test "名前" {`},
		{"scan fast", "スキャン時間"},
		{"foo", "scan / watch / test"},
		{"test \"a\" {\n  wait Y0 on in 1s\n}", "within"},
		{"test \"a\" {\n  run soon\n}", "スキャン数"},
		{"test \"a\" {\n  expect Y0 maybe\n}", "値 \"maybe\""},
		{"test \"a {", "文字列が閉じていません"},
	}
	for _, c := range cases {
		_, err := Parse(c.src)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q: got %v, want %q", c.src, err, c.want)
		}
	}
}

func TestParseDurationAndValue(t *testing.T) {
	for in, want := range map[string]int64{"500ms": 500, "5s": 5000, "1m30s": 90000, "T#1s500ms": 1500, "t#2S": 2000} {
		got, err := profile.ParseDuration(in)
		if err != nil || got != want {
			t.Errorf("%s: %d %v", in, got, err)
		}
	}
	for in, want := range map[string]int64{"on": 1, "OFF": 0, "-5": -5, "K-5": -5, "H1F": 31, "16#1F": 31, "2s": 2000} {
		got, err := ParseValue(in)
		if err != nil || got != want {
			t.Errorf("%s: %d %v", in, got, err)
		}
	}
}
