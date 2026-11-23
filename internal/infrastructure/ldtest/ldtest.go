// Package ldtest reads scenario files (.ldtest) into scenario.Script.
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
//	  force X1 off
//	  unforce X1
//	}
package ldtest

import (
	"fmt"
	"strconv"
	"strings"

	"rungkit/internal/application/scenario"
	"rungkit/internal/domain/profile"
)

// Error is a syntax error in a scenario file.
type Error struct {
	Line int
	Msg  string
}

func (e *Error) Error() string { return fmt.Sprintf("%d: %s", e.Line, e.Msg) }

// Parser adapts this package to the workspace.
type Parser struct{}

// ParseScenario reads a scenario file.
func (Parser) ParseScenario(src string) (*scenario.Script, error) { return Parse(src) }

var arity = map[string][2]int{ // min, max args
	"set": {2, 2}, "force": {2, 2}, "unforce": {1, 1}, "run": {1, 1},
	"expect": {2, 2}, "wait": {4, 4}, "watch": {1, 99},
}

// Parse reads a scenario file.
func Parse(src string) (*scenario.Script, error) {
	sc := &scenario.Script{ScanMs: scenario.DefaultScanMs}
	var cur *scenario.Test
	for i, raw := range strings.Split(src, "\n") {
		line := i + 1
		errf := func(format string, args ...any) error {
			return &Error{line, fmt.Sprintf(format, args...)}
		}
		words, err := fields(raw)
		if err != nil {
			return nil, errf("%s", err)
		}
		if len(words) == 0 {
			continue
		}
		if cur == nil {
			switch words[0] {
			case "scan":
				if len(words) != 2 {
					return nil, errf("scan 10ms のように書きます")
				}
				ms, err := profile.ParseDuration(words[1])
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
				cur = &scenario.Test{Name: words[1], Line: line}
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
		st, err := step(words)
		if err != nil {
			return nil, errf("%s", err)
		}
		st.Line = line
		cur.Steps = append(cur.Steps, st)
	}
	if cur != nil {
		return nil, &Error{cur.Line, fmt.Sprintf("test %q の } がありません", cur.Name)}
	}
	return sc, nil
}

func step(words []string) (scenario.Step, error) {
	cmd, args := words[0], words[1:]
	a, ok := arity[cmd]
	if !ok {
		return scenario.Step{}, fmt.Errorf("命令 %q はありません（set / force / unforce / run / expect / wait / watch）", cmd)
	}
	if len(args) < a[0] || len(args) > a[1] {
		return scenario.Step{}, fmt.Errorf("%s の書き方が違います", cmd)
	}
	st := scenario.Step{Kind: scenario.Kind(cmd), Operand: args[0]}
	var err error
	switch cmd {
	case "set", "force", "expect":
		st.Value, err = ParseValue(args[1])
	case "watch":
		st.Operand, st.Operands = "", args
	case "run":
		st.Operand = ""
		if n, e := strconv.Atoi(args[0]); e == nil && n >= 0 {
			st.Scans = n
		} else if ms, e := profile.ParseDuration(args[0]); e == nil {
			st.DurationMs, st.ByTime = ms, true
		} else {
			err = fmt.Errorf("%q はスキャン数（3）か時間（5s, 200ms）で書きます", args[0])
		}
	case "wait":
		if args[2] != "within" {
			return st, fmt.Errorf("wait T0 on within 6s のように書きます")
		}
		if st.Value, err = ParseValue(args[1]); err != nil {
			return st, err
		}
		st.ByTime = true
		if st.DurationMs, err = profile.ParseDuration(args[3]); err != nil {
			err = fmt.Errorf("待つ時間 %q が読めません（例: 6s）", args[3])
		}
	}
	return st, err
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

// ParseValue reads on/off, integers (100, -5, K100, H1F, 16#1F) and times.
func ParseValue(s string) (int64, error) {
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
	if ms, err := profile.ParseDuration(s); err == nil {
		return ms, nil
	}
	return 0, fmt.Errorf("値 %q が読めません（on / off / 数値 / 時間）", s)
}
