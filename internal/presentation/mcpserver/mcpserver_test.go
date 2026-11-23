package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Piyoone3939/rungkit/internal/application/workspace"
	"github.com/Piyoone3939/rungkit/internal/infrastructure/export/melsec"
	"github.com/Piyoone3939/rungkit/internal/infrastructure/export/st"
	"github.com/Piyoone3939/rungkit/internal/infrastructure/filestore"
	"github.com/Piyoone3939/rungkit/internal/infrastructure/jsoncodec"
	"github.com/Piyoone3939/rungkit/internal/infrastructure/ldt"
	"github.com/Piyoone3939/rungkit/internal/infrastructure/ldtest"
)

func connect(t *testing.T) *mcp.ClientSession {
	t.Helper()
	ws := workspace.New(workspace.Deps{
		Files: filestore.Store{}, Notation: ldt.Notation{}, Scenarios: ldtest.Parser{},
		Exporters: []workspace.Exporter{melsec.Exporter{}, st.Exporter{}, jsoncodec.Exporter{}, ldt.Exporter{}},
	})
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	if _, err := New(ws, "test").Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "client"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

// call invokes a tool and returns its text; wantErr says whether the tool
// should report an error.
func call(t *testing.T, cs *mcp.ClientSession, name string, args any, wantErr bool) string {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	if res.IsError != wantErr {
		t.Fatalf("%s: isError=%v: %s", name, res.IsError, b.String())
	}
	return b.String()
}

func TestTools(t *testing.T) {
	cs := connect(t)
	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tl := range tools.Tools {
		names = append(names, tl.Name)
	}
	want := "check_program edit_program export_program new_program open_program read_program redo run_scenario save_program undo"
	if got := strings.Join(sorted(names), " "); got != want {
		t.Errorf("tools: %s", got)
	}
}

func sorted(xs []string) []string {
	for i := range xs {
		for j := i + 1; j < len(xs); j++ {
			if xs[j] < xs[i] {
				xs[i], xs[j] = xs[j], xs[i]
			}
		}
	}
	return xs
}

func TestAgentWorkflow(t *testing.T) {
	cs := connect(t)
	dir := t.TempDir()
	src, err := os.ReadFile("../../../examples/conveyor.ldt")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "conveyor.ldt")
	if err := os.WriteFile(file, src, 0o644); err != nil {
		t.Fatal(err)
	}

	// nothing open yet
	call(t, cs, "read_program", map[string]any{}, true)

	out := call(t, cs, "open_program", map[string]any{"path": file}, false)
	if !strings.Contains(out, "(X0@e1 | Y0@e2) & !X1@e3 -> OUT(Y0)@e4") || !strings.Contains(out, "errors: 0") {
		t.Fatalf("open:\n%s", out)
	}

	// add an emergency stop in series with the stop button
	out = call(t, cs, "edit_program", map[string]any{"ops": []map[string]any{
		{"op": "insert_series", "target": "e3", "position": "after", "text": "!X3"},
		{"op": "set_symbol", "name": "X3", "comment": "非常停止"},
	}}, false)
	var res struct {
		Rungs []struct{ ID, Text string }
		New   []string `json:"new_elements"`
		Diags []any    `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if len(res.Rungs) != 1 || res.Rungs[0].Text != "(X0@e1 | Y0@e2) & !X1@e3 & !X3@e16 -> OUT(Y0)@e4" || len(res.Diags) != 0 {
		t.Fatalf("edit: %s", out)
	}

	// a refused edit leaves the program alone and says why
	out = call(t, cs, "edit_program", map[string]any{"ops": []map[string]any{
		{"op": "insert_series", "target": "e4", "position": "after", "text": "X5"},
	}}, true)
	if !strings.Contains(out, "E_OUTPUT_NOT_TERMINAL") {
		t.Fatalf("refused edit: %s", out)
	}

	// the scenario proves the emergency stop works
	out = call(t, cs, "run_scenario", map[string]any{"scenario": `test "estop" {
  set X0 on
  run 1
  expect Y0 on
  set X0 off
  set X3 on
  run 1
  expect Y0 off
}`}, false)
	if !strings.Contains(out, `"pass": true`) {
		t.Fatalf("scenario: %s", out)
	}
	out = call(t, cs, "run_scenario", map[string]any{"path": "../../../examples/conveyor.ldtest"}, false)
	if strings.Contains(out, `"pass": false`) {
		t.Fatalf("example scenario: %s", out)
	}

	out = call(t, cs, "export_program", map[string]any{"format": "melsec"}, false)
	if !strings.Contains(out, "LD\tX0\nOR\tY0\nANI\tX1\nANI\tX3\nOUT\tY0") {
		t.Fatalf("export: %s", out)
	}
	call(t, cs, "export_program", map[string]any{"format": "pdf"}, true)

	// undo and redo
	out = call(t, cs, "undo", map[string]any{}, false)
	if strings.Contains(out, "X3@") {
		t.Fatalf("undo: %s", out)
	}
	call(t, cs, "redo", map[string]any{}, false)

	// save and read back from disk
	call(t, cs, "save_program", map[string]any{}, false)
	saved, _ := os.ReadFile(file)
	if !strings.Contains(string(saved), "(X0 | Y0) & !X1 & !X3 -> OUT(Y0)") || !strings.Contains(string(saved), `sym X3 "非常停止"`) {
		t.Fatalf("saved:\n%s", saved)
	}
	call(t, cs, "save_program", map[string]any{"path": filepath.Join(dir, "x.txt")}, true)

	ds := call(t, cs, "check_program", map[string]any{}, false)
	if strings.TrimSpace(ds) != "[]" {
		t.Fatalf("check: %s", ds)
	}
}

func TestNewProgramFromScratch(t *testing.T) {
	cs := connect(t)
	call(t, cs, "new_program", map[string]any{"profile": "omron"}, true)
	call(t, cs, "new_program", map[string]any{"profile": "iec"}, false)
	out := call(t, cs, "edit_program", map[string]any{"ops": []map[string]any{
		{"op": "set_symbol", "name": "Start", "type": "BOOL", "address": "%IX0.0"},
		{"op": "set_symbol", "name": "Motor", "type": "BOOL"},
		{"op": "set_symbol", "name": "Delay", "type": "TON"},
		{"op": "add_rung", "text": "Start -> TON(Delay, T#2s)", "comment": "起動遅延"},
		{"op": "add_rung", "text": "Delay -> OUT(Motor)"},
	}}, false)
	if !strings.Contains(out, `"diagnostics": []`) {
		t.Fatalf("%s", out)
	}
	out = call(t, cs, "run_scenario", map[string]any{"scenario": "test \"d\" {\n  set Start on\n  wait Motor on within 2100ms\n}\n", "watch": []string{"Delay.ET"}}, false)
	if !strings.Contains(out, `"pass": true`) || !strings.Contains(out, `"trace"`) {
		t.Fatalf("%s", out)
	}
	st := call(t, cs, "export_program", map[string]any{"format": "st"}, false)
	if !strings.Contains(st, "Delay(IN := Start, PT := T#2s);") {
		t.Fatalf("%s", st)
	}
}
