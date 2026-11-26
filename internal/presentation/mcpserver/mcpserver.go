// Package mcpserver exposes the workspace as an MCP server, so any agent
// (Claude Code, Cursor, a local LLM client) can read, edit, check, simulate
// and export ladder programs. It only translates tool calls into workspace
// calls; all behavior lives in the application and domain layers.
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Piyoone3939/rungkit/internal/application/editing"
	"github.com/Piyoone3939/rungkit/internal/application/workspace"
	"github.com/Piyoone3939/rungkit/internal/domain/i18n"
)

// Instructions are sent to the client when it connects.
const Instructions = `rungkit edits PLC ladder programs as text.

Workflow: open_program (or new_program) -> read_program -> edit_program ->
check_program -> run_scenario -> save_program. Nothing is written to disk
until save_program. Paths are relative to the server's working folder; files
outside it cannot be opened or saved.

Ladder text (.ldt), one rung body:
  (X0 | Y0) & !X1 -> OUT(Y0)
  & series (AND, binds tighter), | parallel (OR), !X b-contact,
  rise(X)/fall(X) edges, [D0 >= K100] compare (= <> < <= > >=).
  Outputs go right of ->: OUT SET RST PLS PLF (coils), TON(T0, K50),
  CTU(C0, K10), MOV ADD SUB MUL DIV INC DEC (P suffix = once per rising edge).
  Several outputs: X0 -> { OUT(Y0)  M1 -> SET(M10) }

read_program shows every element with its ID (X0@e3, OUT(Y0)@e4) and every
rung with "// r1". Edit commands address those IDs. An edit_program call is
atomic and is one undo step; it is refused if a rung would become an
invalid ladder. Check errors (e.g. writing an input) are reported, not
refused, so fix them with further edits. Give an op key: "name" to refer
to what it creates later in the same call as $name (or $name.2 for its 2nd
new element); the result's keys map each name to the real IDs.

Profiles: melsec-iqr (Mitsubishi iQ-R: X/Y/B/W hexadecimal, D0.3 bit of word,
K/H constants, T counts 100 ms); there, set_symbol takes only a device name and a
comment and iec (variables declared with set_symbol,
%IX0.0, T#5s).

Scenarios (run_scenario):
  scan 10ms
  test "name" {
    set X0 on
    run 1          // scans; or run 5s
    expect Y0 on
    wait T0 on within 6s
  }
T0.ET reads a timer's elapsed ms, C0.CV a counter's value.`

// Serve runs the server on stdin/stdout until the client disconnects.
func Serve(ctx context.Context, ws *workspace.Workspace, version string) error {
	return New(ws, version).Run(ctx, &mcp.StdioTransport{})
}

// New builds the server with all tools registered.
func New(ws *workspace.Workspace, version string) *mcp.Server {
	h := &handler{ws: ws}
	s := mcp.NewServer(&mcp.Implementation{Name: "rungkit", Version: version}, &mcp.ServerOptions{Instructions: Instructions})

	mcp.AddTool(s, &mcp.Tool{Name: "open_program", Description: "Open a ladder program: .ldt, .json or PLCopen XML (.xml, Ladder Diagram). Starts a new undo history."}, h.open)
	mcp.AddTool(s, &mcp.Tool{Name: "new_program", Description: "Start an empty program for a profile (melsec-iqr or iec)."}, h.newProgram)
	mcp.AddTool(s, &mcp.Tool{Name: "save_program", Description: "Save the program as .ldt, .json or PLCopen XML (.xml). Without path, saves to the file it was opened from."}, h.save)
	mcp.AddTool(s, &mcp.Tool{Name: "read_program", Description: "Show the whole program as ladder text with rung IDs (// r1) and element IDs (X0@e3), plus the number of errors and warnings."}, h.read)
	mcp.AddTool(s, &mcp.Tool{Name: "check_program", Description: "Validate the program. Returns diagnostics with codes (E_READ_ONLY, W_DOUBLE_COIL, W_SET_RST_SAME_EDGE, ...), rung and element IDs."}, h.check)
	mcp.AddTool(s, &mcp.Tool{Name: "edit_program", Description: "Apply edit commands atomically (all or nothing, one undo step). Commands:\n" + editing.Ops + "\nReturns the changed rungs with IDs, new element IDs and diagnostics."}, h.edit)
	mcp.AddTool(s, &mcp.Tool{Name: "undo", Description: "Revert the last edit_program call."}, h.undo)
	mcp.AddTool(s, &mcp.Tool{Name: "redo", Description: "Re-apply the last undone edit."}, h.redo)
	mcp.AddTool(s, &mcp.Tool{Name: "export_program", Description: "Convert the program: melsec (Mitsubishi instruction list), gxw3 (the same list as the CSV GX Works3 imports with Edit > Import File; returned here as text, while the file itself is UTF-16, written by `rungkit export -to gxw3 -o FILE`), st (IEC Structured Text), plcopen (PLCopen XML TC6 v2.01 Ladder Diagram), ascii (the ladder drawn as text, handy to show a person), svg, json, ldt."}, h.export)
	mcp.AddTool(s, &mcp.Tool{Name: "run_scenario", Description: "Run test scenarios on the scan simulator (virtual time, deterministic). Give the scenario text, or a path to a .ldtest file. With trace, watched values are returned for the first scan, every scan where one changed, and the last scan (at most 200 records per test)."}, h.runScenario)
	return s
}

// maxTrace caps trace records per test in tool results; only scans where a
// watched value changed are kept, and the middle is dropped beyond this.
const maxTrace = 200

type handler struct {
	mu sync.Mutex
	ws *workspace.Workspace
}

type none struct{}

type pathIn struct {
	Path string `json:"path" jsonschema:"file path of a .ldt, .json or PLCopen .xml program"`
}

type savePathIn struct {
	Path string `json:"path,omitempty" jsonschema:"where to save (.ldt, .json or .xml); empty = the opened file"`
}

type profileIn struct {
	Profile string `json:"profile" jsonschema:"melsec-iqr or iec"`
}

type editIn struct {
	Ops []editing.Op `json:"ops" jsonschema:"edit commands, applied in order"`
}

type exportIn struct {
	Format string `json:"format" jsonschema:"melsec, st, plcopen, ascii, svg, json or ldt"`
}

type scenarioIn struct {
	Scenario string   `json:"scenario,omitempty" jsonschema:"scenario text (.ldtest syntax)"`
	Path     string   `json:"path,omitempty" jsonschema:"path to a .ldtest file (instead of scenario)"`
	Trace    bool     `json:"trace,omitempty" jsonschema:"return watched values after every scan"`
	Watch    []string `json:"watch,omitempty" jsonschema:"operands to record, e.g. Y0, T0.ET, D0"`
}

func text(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}

func jsonText(v any) (*mcp.CallToolResult, any, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return text(string(b)), nil, nil
}

// summary is the program text with IDs and a status line.
func (h *handler) summary(prefix string) (*mcp.CallToolResult, any, error) {
	t, err := h.ws.Text(true)
	if err != nil {
		return nil, nil, err
	}
	ds, _ := h.ws.Check()
	var errs, warns int
	for _, d := range ds {
		if d.Severity == "error" {
			errs++
		} else {
			warns++
		}
	}
	path := h.ws.Path()
	if path == "" {
		path = i18n.T("(未保存)")
	}
	status := fmt.Sprintf("file: %s  unsaved changes: %v  errors: %d  warnings: %d", path, h.ws.Dirty(), errs, warns)
	return text(strings.TrimSpace(prefix+"\n"+status) + "\n\n" + t), nil, nil
}

func (h *handler) open(_ context.Context, _ *mcp.CallToolRequest, in pathIn) (*mcp.CallToolResult, any, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.ws.Open(in.Path); err != nil {
		return nil, nil, err
	}
	return h.summary("opened")
}

func (h *handler) newProgram(_ context.Context, _ *mcp.CallToolRequest, in profileIn) (*mcp.CallToolResult, any, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.ws.NewProgram(in.Profile); err != nil {
		return nil, nil, err
	}
	return h.summary("new program")
}

func (h *handler) save(_ context.Context, _ *mcp.CallToolRequest, in savePathIn) (*mcp.CallToolResult, any, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	path, err := h.ws.Save(in.Path)
	if err != nil {
		return nil, nil, err
	}
	return text("saved " + path), nil, nil
}

func (h *handler) read(_ context.Context, _ *mcp.CallToolRequest, _ none) (*mcp.CallToolResult, any, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.summary("")
}

func (h *handler) check(_ context.Context, _ *mcp.CallToolRequest, _ none) (*mcp.CallToolResult, any, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	ds, err := h.ws.Check()
	if err != nil {
		return nil, nil, err
	}
	return jsonText(ds)
}

func (h *handler) edit(_ context.Context, _ *mcp.CallToolRequest, in editIn) (*mcp.CallToolResult, any, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	res, err := h.ws.Edit(in.Ops)
	if err != nil {
		return nil, nil, err
	}
	return jsonText(res)
}

func (h *handler) undo(_ context.Context, _ *mcp.CallToolRequest, _ none) (*mcp.CallToolResult, any, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.ws.Undo(); err != nil {
		return nil, nil, err
	}
	return h.summary("undone")
}

func (h *handler) redo(_ context.Context, _ *mcp.CallToolRequest, _ none) (*mcp.CallToolResult, any, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.ws.Redo(); err != nil {
		return nil, nil, err
	}
	return h.summary("redone")
}

func (h *handler) export(_ context.Context, _ *mcp.CallToolRequest, in exportIn) (*mcp.CallToolResult, any, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	out, err := h.ws.Export(in.Format)
	if err != nil {
		return nil, nil, err
	}
	return text(out), nil, nil
}

func (h *handler) runScenario(_ context.Context, _ *mcp.CallToolRequest, in scenarioIn) (*mcp.CallToolResult, any, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	opt := workspace.ScenarioOptions{Trace: in.Trace, Watch: in.Watch, MaxTrace: maxTrace}
	var err error
	var res any
	switch {
	case in.Scenario != "" && in.Path != "":
		return nil, nil, i18n.Errorf("scenario と path はどちらか一方だけ指定します")
	case in.Scenario != "":
		res, err = h.ws.RunScenario(in.Scenario, opt)
	case in.Path != "":
		res, err = h.ws.RunScenarioFile(in.Path, opt)
	default:
		return nil, nil, i18n.Errorf("scenario か path が必要です")
	}
	if err != nil {
		return nil, nil, err
	}
	return jsonText(res)
}
