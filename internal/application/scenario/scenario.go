// Package scenario is the use case of checking a program against scenarios:
// set inputs, let time pass, and expect values. The scenario text format is
// read by an infrastructure adapter (.ldtest); this package only knows the
// model and how to run it on the simulator.
package scenario

import (
	"fmt"

	"github.com/Piyoone3939/rungkit/internal/domain/i18n"
	"github.com/Piyoone3939/rungkit/internal/domain/ladder"
	"github.com/Piyoone3939/rungkit/internal/domain/sim"
)

// Script is a set of tests sharing a scan time and watched operands.
type Script struct {
	ScanMs int64
	Watch  []string
	Tests  []*Test
}

// Test is one named scenario. Every test starts from a fresh simulator.
type Test struct {
	Name  string
	Line  int
	Steps []Step
}

// Kind is a step command.
type Kind string

const (
	CmdSet     Kind = "set"     // Operand = Value (from outside, e.g. an input)
	CmdForce   Kind = "force"   // pin Operand to Value
	CmdUnforce Kind = "unforce" // release Operand
	CmdRun     Kind = "run"     // run Scans scans, or DurationMs of time
	CmdExpect  Kind = "expect"  // Operand must equal Value now
	CmdWait    Kind = "wait"    // run until Operand equals Value, at most DurationMs
	CmdWatch   Kind = "watch"   // record Operands after every scan
)

// Step is one command.
type Step struct {
	Line       int
	Kind       Kind
	Operand    string
	Operands   []string
	Value      int64
	Scans      int
	DurationMs int64 // used instead of Scans when ByTime is set (always for Wait)
	ByTime     bool
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
	// Trace holds the first scan, every scan where a watched value changed,
	// and the last scan.
	Trace []sim.Record `json:"trace,omitempty"`
	// TraceOmitted counts records dropped from the middle to keep the
	// result small (see Limit).
	TraceOmitted int `json:"trace_omitted,omitempty"`
}

// DefaultScanMs is used when a script does not set a scan time.
const DefaultScanMs = sim.DefaultScanMs

// Run executes every test of sc against p, each on a fresh simulator.
func Run(p *ladder.Program, sc *Script) ([]Result, error) {
	scanMs := sc.ScanMs
	if scanMs == 0 {
		scanMs = DefaultScanMs
	}
	var results []Result
	for _, t := range sc.Tests {
		s, err := sim.New(p)
		if err != nil {
			return nil, err
		}
		if err := s.SetScanTime(scanMs); err != nil {
			return nil, err
		}
		if err := s.Watch(sc.Watch...); err != nil {
			return nil, fmt.Errorf("watch: %w", err)
		}
		res := Result{Name: t.Name, Pass: true}
		for _, st := range t.Steps {
			if msg := do(s, st); msg != "" {
				res.Pass, res.Line, res.Msg = false, st.Line, msg
				break
			}
		}
		res.Scans, res.TimeMs = s.Scans(), s.Now()
		res.Watch, res.Trace = s.Watched(), Changes(s.Trace())
		results = append(results, res)
	}
	return results, nil
}

// do runs one step and returns a failure message ("" when it passed).
func do(s *sim.Sim, st Step) string {
	at := func() string { return i18n.T("（スキャン %d, %s）", s.Scans(), sim.FormatDuration(s.Now())) }
	scans := func() int {
		if st.ByTime || st.Kind == CmdWait {
			return int((st.DurationMs + s.ScanTime() - 1) / s.ScanTime())
		}
		return st.Scans
	}
	var err error
	switch st.Kind {
	case CmdSet:
		err = s.Set(st.Operand, st.Value)
	case CmdForce:
		err = s.Force(st.Operand, st.Value)
	case CmdUnforce:
		s.Unforce(st.Operand)
	case CmdWatch:
		err = s.Watch(st.Operands...)
	case CmdRun:
		err = s.Run(scans())
	case CmdExpect:
		got, err := s.Get(st.Operand)
		if err != nil {
			return err.Error()
		}
		if got != st.Value {
			return i18n.T("expect %s %s でしたが、%s でした%s",
				st.Operand, s.Format(st.Operand, st.Value), s.Format(st.Operand, got), at())
		}
	case CmdWait:
		limit, start := scans(), s.Now()
		for i := 0; ; i++ {
			got, err := s.Get(st.Operand)
			if err != nil {
				return err.Error()
			}
			if got == st.Value {
				return ""
			}
			if i == limit {
				return i18n.T("%s 待っても %s は %s になりませんでした（%s のまま）%s",
					sim.FormatDuration(s.Now()-start), st.Operand, s.Format(st.Operand, st.Value), s.Format(st.Operand, got), at())
			}
			if err := s.Scan(); err != nil {
				return err.Error()
			}
		}
	default:
		return i18n.T("命令 %q はありません", st.Kind)
	}
	if err != nil {
		return err.Error()
	}
	return ""
}

// Changes keeps the first record, every record whose values differ from the
// one before, and the last record.
func Changes(trace []sim.Record) []sim.Record {
	var out []sim.Record
	for i, r := range trace {
		if i == 0 || i == len(trace)-1 || !sameValues(r.Values, trace[i-1].Values) {
			out = append(out, r)
		}
	}
	return out
}

func sameValues(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Limit keeps at most max records of r's trace: the first and last halves,
// counting the rest in TraceOmitted. max <= 0 keeps everything.
func (r *Result) Limit(max int) {
	if max <= 0 || len(r.Trace) <= max {
		return
	}
	head, tail := max/2, max-max/2
	r.TraceOmitted = len(r.Trace) - max
	r.Trace = append(r.Trace[:head:head], r.Trace[len(r.Trace)-tail:]...)
}
