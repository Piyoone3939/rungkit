//go:build js && wasm

// Package web exposes the workspace to the browser as a global JavaScript
// object, rungkit, whose functions take strings and return JSON strings.
// The page (web/) is plain HTML and JavaScript.
package web

import (
	"encoding/json"
	"regexp"
	"strconv"
	"syscall/js"

	"github.com/Piyoone3939/rungkit/internal/application/workspace"
	"github.com/Piyoone3939/rungkit/internal/domain/i18n"
)

// Buffer holds the text being edited, so the workspace can open it by name.
// The composition root passes an in-memory store.
type Buffer interface {
	Put(path string, data []byte)
}

type diag struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Rung     string `json:"rung,omitempty"`
	Elem     string `json:"elem,omitempty"`
	Line     int    `json:"line,omitempty"`
	Col      int    `json:"col,omitempty"`
}

type update struct {
	OK          bool              `json:"ok"`
	Error       string            `json:"error,omitempty"`
	Line        int               `json:"line,omitempty"`
	Col         int               `json:"col,omitempty"`
	Profile     string            `json:"profile,omitempty"`
	Rungs       int               `json:"rungs"`
	Diagnostics []diag            `json:"diagnostics"`
	SVG         string            `json:"svg,omitempty"`
	Positions   map[string][2]int `json:"positions,omitempty"`
}

var reAt = regexp.MustCompile(`:(\d+):(\d+):`)

func at(s string) (int, int) {
	m := reAt.FindStringSubmatch(s)
	if m == nil {
		return 0, 0
	}
	l, _ := strconv.Atoi(m[1])
	c, _ := strconv.Atoi(m[2])
	return l, c
}

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// Serve registers the functions and blocks forever.
func Serve(ws *workspace.Workspace, buf Buffer, version string) {
	api := map[string]any{
		"version": version,

		// update(name, text): load the text and return diagnostics, the
		// drawing and element positions.
		"update": js.FuncOf(func(_ js.Value, args []js.Value) any {
			name, text := args[0].String(), args[1].String()
			buf.Put(name, []byte(text))
			if err := ws.Open(name); err != nil {
				l, c := at(err.Error())
				return toJSON(update{Error: err.Error(), Line: l, Col: c, Diagnostics: []diag{}})
			}
			u := update{OK: true, Diagnostics: []diag{}, Positions: map[string][2]int{}}
			if p, err := ws.Program(); err == nil {
				u.Profile, u.Rungs = p.Profile, len(p.Rungs)
			}
			for id, pos := range ws.Positions() {
				l, c := at(":" + pos + ":")
				u.Positions[id] = [2]int{l, c}
			}
			ds, _ := ws.Check()
			for _, d := range ds {
				x := diag{Severity: string(d.Severity), Code: d.Code, Message: d.Message, Rung: d.Rung, Elem: d.Elem}
				if p, ok := u.Positions[d.Elem]; ok {
					x.Line, x.Col = p[0], p[1]
				} else if p, ok := u.Positions[d.Rung]; ok {
					x.Line, x.Col = p[0], p[1]
				}
				u.Diagnostics = append(u.Diagnostics, x)
			}
			if svg, err := ws.Export("svg"); err == nil {
				u.SVG = svg
			}
			return toJSON(u)
		}),

		// export(format): the program in another format.
		"export": js.FuncOf(func(_ js.Value, args []js.Value) any {
			out, err := ws.Export(args[0].String())
			if err != nil {
				return toJSON(map[string]any{"ok": false, "error": err.Error()})
			}
			return toJSON(map[string]any{"ok": true, "text": out})
		}),

		// format(): the canonical .ldt text.
		"format": js.FuncOf(func(_ js.Value, _ []js.Value) any {
			out, err := ws.Text(false)
			if err != nil {
				return toJSON(map[string]any{"ok": false, "error": err.Error()})
			}
			return toJSON(map[string]any{"ok": true, "text": out})
		}),

		// runScenario(text, trace): run .ldtest text; trace rows come with
		// values formatted (on/off, times).
		"runScenario": js.FuncOf(func(_ js.Value, args []js.Value) any {
			res, err := ws.RunScenario(args[0].String(), workspace.ScenarioOptions{Trace: args[1].Bool(), MaxTrace: 400})
			if err != nil {
				l, c := at(err.Error())
				return toJSON(map[string]any{"ok": false, "error": err.Error(), "line": l, "col": c})
			}
			format := ws.ValueFormatter()
			type test struct {
				Name    string     `json:"name"`
				Pass    bool       `json:"pass"`
				Line    int        `json:"line,omitempty"`
				Msg     string     `json:"message,omitempty"`
				Scans   int        `json:"scans"`
				TimeMs  int64      `json:"time_ms"`
				Watch   []string   `json:"watch,omitempty"`
				Rows    [][]string `json:"rows,omitempty"`
				Omitted int        `json:"omitted,omitempty"`
			}
			var out []test
			for _, r := range res {
				t := test{Name: r.Name, Pass: r.Pass, Line: r.Line, Msg: r.Msg, Scans: r.Scans, TimeMs: r.TimeMs, Watch: r.Watch, Omitted: r.TraceOmitted}
				for _, rec := range r.Trace {
					row := []string{strconv.Itoa(rec.Scan), strconv.FormatInt(rec.TimeMs, 10)}
					for i, v := range rec.Values {
						row = append(row, format(r.Watch[i], v))
					}
					t.Rows = append(t.Rows, row)
				}
				out = append(out, t)
			}
			return toJSON(map[string]any{"ok": true, "results": out})
		}),

		"setLanguage": js.FuncOf(func(_ js.Value, args []js.Value) any {
			i18n.SetLanguage(args[0].String())
			return nil
		}),

		"formats": js.FuncOf(func(_ js.Value, _ []js.Value) any {
			return toJSON(ws.Formats())
		}),
	}
	js.Global().Set("rungkit", js.ValueOf(api))
	js.Global().Call("dispatchEvent", js.Global().Get("Event").New("rungkit-ready"))
	select {}
}
