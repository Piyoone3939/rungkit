package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/Piyoone3939/rungkit/internal/application/scenario"
	"github.com/Piyoone3939/rungkit/internal/application/workspace"
	"github.com/Piyoone3939/rungkit/internal/domain/i18n"
)

func runTests(ws *workspace.Workspace, path string, trace, asJSON bool, stdout, stderr io.Writer) int {
	results, err := ws.RunScenarioFile(path, workspace.ScenarioOptions{Trace: trace})
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", path, err)
		return 1
	}
	failed := 0
	for _, r := range results {
		if !r.Pass {
			failed++
		}
	}
	if asJSON {
		b, _ := json.MarshalIndent(results, "", "  ")
		fmt.Fprintln(stdout, string(b))
	} else {
		format := ws.ValueFormatter()
		for _, r := range results {
			if r.Pass {
				fmt.Fprintln(stdout, i18n.T("PASS  %s  (%d スキャン)", r.Name, r.Scans))
			} else {
				fmt.Fprintf(stdout, "FAIL  %s\n      %s:%d: %s\n", r.Name, path, r.Line, r.Msg)
			}
			if trace && len(r.Trace) > 0 {
				printTrace(stdout, format, r)
			}
		}
		fmt.Fprintln(stdout, "\n"+i18n.T("%d 件中 %d 件成功", len(results), len(results)-failed))
	}
	if failed > 0 {
		return 1
	}
	return 0
}

// printTrace prints the scans where a watched value changed.
func printTrace(w io.Writer, format func(string, int64) string, r scenario.Result) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "      scan\ttime\t%s\n", strings.Join(r.Watch, "\t"))
	var prev []int64
	for _, rec := range r.Trace {
		if prev != nil && equal(prev, rec.Values) {
			continue
		}
		cells := make([]string, len(rec.Values))
		for i, v := range rec.Values {
			cells[i] = format(r.Watch[i], v)
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
