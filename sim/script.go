package sim

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"rungkit/ir"
)

// A scenario file (.ldtest) drives the simulator and checks the results.
//
//	scan 10ms
//	watch X0 Y0 T0
//
//	test "起動と停止" {
//	  set X0 on
//	  run 1            // scans
//	  expect Y0 on
//	  run 5s           // virtual time
//	  wait T0 on within 6s
//	  force X1 off / unforce X1
//	}
//
// Every test starts from a fresh simulator.

// Script is a parsed scenario file.
type Script struct {
	ScanMs int64
	Watch  []string
	Tests  []*Test
}

// Test is one named scenario.
type Test struct {
	Name  string
	Line  int
	Steps []Step
}

// Step is one command.
type Step struct {
	Line int
	Cmd  string
	Args []string
}

// Result is the outcome of one test.
type Result struct {
	Name   string   `json:"name"`
	Pass   bool     `json:"pass"`
	Line   int      `json:"line,omitempty"` // the failing step
	Msg    string   `json:"message,omitempty"`
	Scans  int      `json:"scans"`
	TimeMs int64    `json:"time_ms"`
	Watch  []string `json:"watch,omitempty"`
	Trace  []Record `json:"trace,omitempty"`
}

// ScriptError is a syntax error in a scenario file.
type ScriptError struct {
	Line int
	Msg  string
}

func (e *ScriptError) Error() string { return fmt.Sprintf("%d: %s", e.Line, e.Msg) }

var arity = map[string][2]int{ // min, max args
	"set": {2, 2}, "force": {2, 2}, "unforce": {1, 1}, "run": {1, 1},
	"expect": {2, 2}, "wait": {4, 4}, "watch": {1, 99},
}

// ParseScript reads a scenario file.
func ParseScript(src string) (*Script, error) {
	sc := &Script{ScanMs: DefaultScanMs}
	var cur *Test
	for i, raw := range strings.Split(src, "\n") {
		line := i + 1
		words, err := fields(raw)
		if err != nil {
			return nil, &ScriptError{line, err.Error()}
		}
		if len(words) == 0 {
			continue
		}
		errf := func(format string, args ...any) error {
			return &ScriptError{line, fmt.Sprintf(format, args...)}
		}
		if cur == nil {
			switch words[0] {
			case "scan":
				if len(words) != 2 {
					return nil, errf("scan 10ms のように書きます")
				}
				ms, err := parseDuration(words[1])
				if err != nil || ms <= 0 {
					return nil, errf("スキャン時間 %q が読めません（例: 10ms）", words[1])
				}
				sc.ScanMs = ms
			case "watch":
				if len(words) < 2 {
					return nil, errf("watch のあとに見たいデバイスを書きます")
				}
				sc.Watch = append(sc.Watch, words[1:]...)
			case "test":
				if len(words) != 3 || words[2] != "{" {
					return nil, errf(`test "名前" { のように書きます`)
				}
				cur = &Test{Name: words[1], Line: line}
			default:
				return nil, errf("scan / watch / test のどれかを書きます（%q は使えません）", words[0])
			}
			continue
		}
		if words[0] == "}" {
			if len(words) != 1 {
				return nil, errf("} は1行に単独で書きます")
			}
			sc.Tests = append(sc.Tests, cur)
			cur = nil
			continue
		}
		a, ok := arity[words[0]]
		if !ok {
			return nil, errf("命令 %q はありません（set / force / unforce / run / expect / wait / watch）", words[0])
		}
		if n := len(words) - 1; n < a[0] || n > a[1] {
			return nil, errf("%s の書き方が違います", words[0])
		}
		if words[0] == "wait" && words[3] != "within" {
			return nil, errf("wait T0 on within 6s のように書きます")
		}
		cur.Steps = append(cur.Steps, Step{Line: line, Cmd: words[0], Args: words[1:]})
	}
	if cur != nil {
		return nil, &ScriptError{cur.Line, fmt.Sprintf("test %q の } がありません", cur.Name)}
	}
	return sc, nil
}

// fields splits a line into words, keeping "quoted strings" whole and
// dropping // comments.
func fields(line string) ([]string, error) {
	var out []string
	i := 0
	for i < len(line) {
		c := line[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == '/' && i+1 < len(line) && line[i+1] == '/':
			return out, nil
		case c == '"':
			j := i + 1
			for j < len(line) && line[j] != '"' {
				if line[j] == '\\' {
					j++
				}
				j++
			}
			if j >= len(line) {
				return nil, fmt.Errorf("文字列が閉じていません")
			}
			s, err := strconv.Unquote(line[i : j+1])
			if err != nil {
				return nil, fmt.Errorf("文字列の書き方が正しくありません")
			}
			out = append(out, s)
			i = j + 1
		default:
			j := i
			for j < len(line) && line[j] != ' ' && line[j] != '\t' && line[j] != '\r' {
				j++
			}
			out = append(out, line[i:j])
			i = j
		}
	}
	return out, nil
}

// RunScript runs every test of sc against p, each on a fresh simulator.
func RunScript(p *ir.Program, sc *Script) ([]Result, error) {
	var results []Result
	for _, t := range sc.Tests {
		s, err := New(p)
		if err != nil {
			return nil, err
		}
		if err := s.SetScanTime(sc.ScanMs); err != nil {
			return nil, err
		}
		if err := s.Watch(sc.Watch...); err != nil {
			return nil, fmt.Errorf("watch: %w", err)
		}
		res := Result{Name: t.Name, Pass: true}
		for _, st := range t.Steps {
			if msg := s.step(st); msg != "" {
				res.Pass, res.Line, res.Msg = false, st.Line, msg
				break
			}
		}
		res.Scans, res.TimeMs = s.Scans(), s.Now()
		res.Watch, res.Trace = s.Watched(), s.Trace()
		results = append(results, res)
	}
	return results, nil
}

// step runs one command and returns a failure message ("" when it passed).
func (s *Sim) step(st Step) string {
	a := st.Args
	at := func() string { return fmt.Sprintf("（スキャン %d, %s）", s.Scans(), fmtDuration(s.Now())) }
	switch st.Cmd {
	case "set", "force":
		v, err := parseValue(a[1])
		if err != nil {
			return err.Error()
		}
		if st.Cmd == "set" {
			err = s.Set(a[0], v)
		} else {
			err = s.Force(a[0], v)
		}
		if err != nil {
			return err.Error()
		}
	case "unforce":
		s.Unforce(a[0])
	case "watch":
		if err := s.Watch(a...); err != nil {
			return err.Error()
		}
	case "run":
		n, err := s.scansFor(a[0])
		if err != nil {
			return err.Error()
		}
		if err := s.Run(n); err != nil {
			return err.Error()
		}
	case "expect":
		want, err := parseValue(a[1])
		if err != nil {
			return err.Error()
		}
		got, err := s.Get(a[0])
		if err != nil {
			return err.Error()
		}
		if got != want {
			return fmt.Sprintf("expect %s %s でしたが、%s でした%s", a[0], s.Format(a[0], want), s.Format(a[0], got), at())
		}
	case "wait":
		want, err := parseValue(a[1])
		if err != nil {
			return err.Error()
		}
		limit, err := s.scansFor(a[3])
		if err != nil {
			return err.Error()
		}
		start := s.Now()
		for i := 0; ; i++ {
			got, err := s.Get(a[0])
			if err != nil {
				return err.Error()
			}
			if got == want {
				return ""
			}
			if i == limit {
				return fmt.Sprintf("%s 待っても %s は %s になりませんでした（%s のまま）%s",
					fmtDuration(s.Now()-start), a[0], s.Format(a[0], want), s.Format(a[0], got), at())
			}
			if err := s.Scan(); err != nil {
				return err.Error()
			}
		}
	}
	return ""
}

// scansFor turns "3" (scans) or "5s" (time) into a number of scans.
func (s *Sim) scansFor(arg string) (int, error) {
	if n, err := strconv.Atoi(arg); err == nil {
		if n < 0 {
			return 0, fmt.Errorf("スキャン数 %d は使えません", n)
		}
		return n, nil
	}
	ms, err := parseDuration(arg)
	if err != nil {
		return 0, fmt.Errorf("%q はスキャン数（3）か時間（5s, 200ms）で書きます", arg)
	}
	return int((ms + s.scanMs - 1) / s.scanMs), nil
}

var reDur = regexp.MustCompile(`(?i)^(?:T#)?((?:\d+(?:ms|s|m|h|d))+)$`)
var reDurPart = regexp.MustCompile(`(?i)(\d+)(ms|s|m|h|d)`)

// parseDuration reads 500ms, 5s, 1m30s or IEC T#1s500ms into milliseconds.
func parseDuration(s string) (int64, error) {
	m := reDur.FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf("時間 %q が読めません", s)
	}
	var total int64
	for _, p := range reDurPart.FindAllStringSubmatch(m[1], -1) {
		n, _ := strconv.ParseInt(p[1], 10, 64)
		unit := map[string]int64{"ms": 1, "s": 1000, "m": 60000, "h": 3600000, "d": 86400000}[strings.ToLower(p[2])]
		total += n * unit
	}
	return total, nil
}

func fmtDuration(ms int64) string {
	if ms%1000 == 0 {
		return fmt.Sprintf("%ds", ms/1000)
	}
	if ms > 1000 {
		return fmt.Sprintf("%.2fs", float64(ms)/1000)
	}
	return fmt.Sprintf("%dms", ms)
}

// parseValue reads on/off, integers (100, -5, K100, H1F, 16#1F) and times.
func parseValue(s string) (int64, error) {
	switch strings.ToLower(s) {
	case "on", "true":
		return 1, nil
	case "off", "false":
		return 0, nil
	}
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		return v, nil
	}
	if len(s) > 1 && (s[0] == 'K' || s[0] == 'k') {
		if v, err := strconv.ParseInt(s[1:], 10, 64); err == nil {
			return v, nil
		}
	}
	if len(s) > 1 && (s[0] == 'H' || s[0] == 'h') {
		if v, err := strconv.ParseInt(s[1:], 16, 64); err == nil {
			return v, nil
		}
	}
	if strings.HasPrefix(s, "16#") {
		if v, err := strconv.ParseInt(s[3:], 16, 64); err == nil {
			return v, nil
		}
	}
	if ms, err := parseDuration(s); err == nil {
		return ms, nil
	}
	return 0, fmt.Errorf("値 %q が読めません（on / off / 数値 / 時間）", s)
}
