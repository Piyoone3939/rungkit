// Command rungkit reads, checks and converts ladder programs (.ldt / .json).
//
//	rungkit fmt [-w] FILE          canonical .ldt (-w rewrites the file)
//	rungkit check FILE             report errors and warnings
//	rungkit export -to FORMAT FILE melsec | st | json | ldt
//	rungkit test [-trace] [-json] FILE SCENARIO  run a scenario on the simulator
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"rungkit/check"
	"rungkit/export/melsec"
	"rungkit/export/st"
	"rungkit/ir"
	"rungkit/ldt"
)

const usage = `rungkit — ladder programs as text

usage:
  rungkit fmt [-w] FILE
  rungkit check FILE
  rungkit export -to melsec|st|json|ldt FILE
  rungkit test [-trace] [-json] FILE SCENARIO.ldtest

FILE is .ldt or .json ("-" reads .ldt from stdin).
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	cmd, rest := args[0], args[1:]
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	write := fs.Bool("w", false, "rewrite the file (fmt)")
	to := fs.String("to", "", "melsec | st | json | ldt (export)")
	name := fs.String("name", "Main", "PROGRAM name (export -to st)")
	trace := fs.Bool("trace", false, "print the scan trace (test)")
	asJSON := fs.Bool("json", false, "print results as JSON (test)")
	if err := fs.Parse(rest); err != nil {
		return 2
	}
	wantArgs := 1
	if cmd == "test" {
		wantArgs = 2
	}
	if fs.NArg() != wantArgs {
		fmt.Fprint(stderr, usage)
		return 2
	}
	file := fs.Arg(0)
	p, sm, err := load(file)
	if err != nil {
		fmt.Fprintf(stderr, "%s:%s\n", file, err)
		return 1
	}

	switch cmd {
	case "test":
		return runTests(p, fs.Arg(1), *trace, *asJSON, stdout, stderr)
	case "fmt":
		out, err := ldt.Format(p)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %s\n", file, err)
			return 1
		}
		if *write && file != "-" {
			if err := os.WriteFile(file, []byte(out), 0o644); err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			return 0
		}
		fmt.Fprint(stdout, out)
	case "check":
		ds := check.Program(p)
		for _, d := range ds {
			fmt.Fprintf(stdout, "%s:%s %s %s: %s\n", file, where(d, sm), d.Severity, d.Code, d.Message)
		}
		if check.HasErrors(ds) {
			return 1
		}
		if len(ds) == 0 {
			fmt.Fprintf(stdout, "%s: OK（%d ラング）\n", file, len(p.Rungs))
		}
	case "export":
		var out string
		switch *to {
		case "melsec":
			out, err = melsec.Export(p)
		case "st":
			out, err = st.Export(p, *name)
		case "json":
			var b []byte
			b, err = ir.MarshalProgram(p)
			out = string(b) + "\n"
		case "ldt":
			out, err = ldt.Format(p)
		default:
			fmt.Fprintln(stderr, "-to には melsec / st / json / ldt のどれかを指定します")
			return 2
		}
		if err != nil {
			fmt.Fprintf(stderr, "%s: %s\n", file, err)
			return 1
		}
		fmt.Fprint(stdout, out)
	default:
		fmt.Fprint(stderr, usage)
		return 2
	}
	return 0
}

func load(file string) (*ir.Program, *ldt.SourceMap, error) {
	var data []byte
	var err error
	if file == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(file)
	}
	if err != nil {
		return nil, nil, errors.New(" " + err.Error())
	}
	if strings.EqualFold(filepath.Ext(file), ".json") {
		p, err := ir.UnmarshalProgram(data)
		if err != nil {
			return nil, nil, errors.New(" " + err.Error())
		}
		p.Normalize()
		p.AssignIDs()
		return p, nil, nil
	}
	p, sm, err := ldt.Parse(string(data))
	if err != nil {
		return nil, nil, err
	}
	return p, sm, nil
}

// where returns "line:col:" for a diagnostic when the source is known.
func where(d check.Diagnostic, sm *ldt.SourceMap) string {
	if sm != nil {
		if pos, ok := sm.Elems[d.Elem]; ok {
			return pos.String() + ":"
		}
		if pos, ok := sm.Rungs[d.Rung]; ok {
			return pos.String() + ":"
		}
	}
	var ids []string
	if d.Rung != "" {
		ids = append(ids, d.Rung)
	}
	if d.Elem != "" {
		ids = append(ids, d.Elem)
	}
	if len(ids) == 0 {
		return ""
	}
	return strings.Join(ids, "/") + ":"
}
