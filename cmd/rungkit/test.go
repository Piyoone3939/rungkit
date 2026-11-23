package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"rungkit/ir"
	"rungkit/sim"
)

func runTests(p *ir.Program, scenario string, trace, asJSON bool, stdout, stderr io.Writer) int {
	src, err := os.ReadFile(scenario)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	sc, err := sim.ParseScript(string(src))
	if err != nil {
		fmt.Fprintf(stderr, "%s:%s\n", scenario, err)
		return 1
	}
	if trace && len(sc.Watch) == 0 {
		sc.Watch = sim.Operands(p) // trace everything the program touches
	}
	results, err := sim.RunScript(p, sc)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	failed := 0
	for _, r := range results {
		if !r.Pass {
			failed++
		}
	}
	if asJSON {
		if !trace {
			for i := range results {
				results[i].Watch, results[i].Trace = nil, nil
			}
		}
		b, _ := json.MarshalIndent(results, "", "  ")
		fmt.Fprintln(stdout, string(b))
	} else {
		// A Sim is only used to format values the way the program reads them.
		fmtSim, _ := sim.New(p)
		for _, r := range results {
			if r.Pass {
				fmt.Fprintf(stdout, "PASS  %s  (%d スキャン)\n", r.Name, r.Scans)
			} else {
				fmt.Fprintf(stdout, "FAIL  %s\n      %s:%d: %s\n", r.Name, scenario, r.Line, r.Msg)
			}
			if trace && len(r.Trace) > 0 {
				printTrace(stdout, fmtSim, r)
			}
		}
		fmt.Fprintf(stdout, "\n%d 件中 %d 件成功\n", len(results), len(results)-failed)
	}
	if failed > 0 {
		return 1
	}
	return 0
}

// printTrace prints the scans where a watched value changed.
func printTrace(w io.Writer, s *sim.Sim, r sim.Result) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "      scan\ttime\t%s\n", strings.Join(r.Watch, "\t"))
	var prev []int64
	for _, rec := range r.Trace {
		if prev != nil && equal(prev, rec.Values) {
			continue
		}
		cells := make([]string, len(rec.Values))
		for i, v := range rec.Values {
			cells[i] = s.Format(r.Watch[i], v)
			if prev != nil && prev[i] != v {
				cells[i] += "*"
			}
		}
		fmt.Fprintf(tw, "      %d\t%dms\t%s\n", rec.Scan, rec.TimeMs, strings.Join(cells, "\t"))
		prev = rec.Values
	}
	tw.Flush()
}

func equal(a, b []int64) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
