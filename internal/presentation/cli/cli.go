// Package cli is the command-line interface. It only parses arguments,
// calls the workspace and prints results.
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"rungkit/internal/application/workspace"
	"rungkit/internal/presentation/mcpserver"
)

const usage = `rungkit — ladder programs as text

usage:
  rungkit fmt [-w] FILE                         canonical .ldt (-w rewrites the file)
  rungkit check FILE                            errors and warnings
  rungkit export -to FORMAT [-name N] FILE      melsec | st | json | ldt
  rungkit test [-trace] [-json] FILE SCENARIO   run a .ldtest scenario
  rungkit mcp [FILE]                            MCP server on stdio

FILE is .ldt or .json ("-" reads .ldt from stdin).
`

// Options are settings that change how the workspace is assembled.
type Options struct {
	// STProgramName is the PROGRAM name used by the st exporter.
	STProgramName string
}

// Factory builds a workspace (the composition root supplies it).
type Factory func(Options) *workspace.Workspace

// Version is shown by the MCP server.
var Version = "0.1.0"

// Run executes a command and returns the process exit code.
func Run(args []string, stdout, stderr io.Writer, newWorkspace Factory) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	cmd, rest := args[0], args[1:]
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	write := fs.Bool("w", false, "rewrite the file (fmt)")
	to := fs.String("to", "", "export format")
	name := fs.String("name", "Main", "PROGRAM name (export -to st)")
	trace := fs.Bool("trace", false, "print the scan trace (test)")
	asJSON := fs.Bool("json", false, "print results as JSON (test)")
	if err := fs.Parse(rest); err != nil {
		return 2
	}
	ws := newWorkspace(Options{STProgramName: *name})

	nargs := map[string][2]int{"fmt": {1, 1}, "check": {1, 1}, "export": {1, 1}, "test": {2, 2}, "mcp": {0, 1}}
	n, ok := nargs[cmd]
	if !ok || fs.NArg() < n[0] || fs.NArg() > n[1] {
		fmt.Fprint(stderr, usage)
		return 2
	}
	if cmd == "mcp" {
		if fs.NArg() == 1 {
			if err := ws.Open(fs.Arg(0)); err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
		}
		if err := mcpserver.Serve(context.Background(), ws, Version); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}

	file := fs.Arg(0)
	if err := ws.Open(file); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	switch cmd {
	case "fmt":
		if *write && file != "-" {
			if _, err := ws.Save(file); err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			return 0
		}
		out, err := ws.Text(false)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %s\n", file, err)
			return 1
		}
		fmt.Fprint(stdout, out)
	case "check":
		return printCheck(ws, file, stdout, stderr)
	case "export":
		if *to == "" {
			fmt.Fprintf(stderr, "-to に書き出し形式を指定します（%v）\n", ws.Formats())
			return 2
		}
		out, err := ws.Export(*to)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %s\n", file, err)
			return 1
		}
		fmt.Fprint(stdout, out)
	case "test":
		return runTests(ws, fs.Arg(1), *trace, *asJSON, stdout, stderr)
	}
	return 0
}

func printCheck(ws *workspace.Workspace, file string, stdout, stderr io.Writer) int {
	ds, err := ws.Check()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	pos := ws.Positions()
	failed := false
	for _, d := range ds {
		at := ""
		if p, ok := pos[d.Elem]; ok {
			at = p + ":"
		} else if p, ok := pos[d.Rung]; ok {
			at = p + ":"
		} else if d.Rung != "" {
			at = d.Rung + "/" + d.Elem + ":"
		}
		fmt.Fprintf(stdout, "%s:%s %s %s: %s\n", file, at, d.Severity, d.Code, d.Message)
		failed = failed || d.Severity == "error"
	}
	if failed {
		return 1
	}
	if len(ds) == 0 {
		p, _ := ws.Program()
		fmt.Fprintf(stdout, "%s: OK（%d ラング）\n", file, len(p.Rungs))
	}
	return 0
}
