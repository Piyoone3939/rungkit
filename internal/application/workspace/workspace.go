// Package workspace is the application service the user interfaces (CLI,
// MCP) talk to: open and save a program, show it, check it, edit it, export
// it and run scenarios. It depends only on the domain and on the interfaces
// it declares here; adapters for them live in the infrastructure layer.
package workspace

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Piyoone3939/rungkit/internal/application/editing"
	"github.com/Piyoone3939/rungkit/internal/application/scenario"
	"github.com/Piyoone3939/rungkit/internal/domain/check"
	"github.com/Piyoone3939/rungkit/internal/domain/i18n"
	"github.com/Piyoone3939/rungkit/internal/domain/ladder"
	"github.com/Piyoone3939/rungkit/internal/domain/profile"
	"github.com/Piyoone3939/rungkit/internal/domain/sim"
)

// Positions maps rung and element IDs to "line:col" in the source file.
type Positions map[string]string

// Files loads and stores programs and reads other text files.
type Files interface {
	LoadProgram(path string) (*ladder.Program, Positions, error)
	SaveProgram(path string, p *ladder.Program) error
	ReadText(path string) (string, error)
}

// Notation is the text notation for programs and rung fragments.
type Notation interface {
	editing.Notation
	FormatProgram(p *ladder.Program, withIDs bool) (string, error)
}

// Exporter converts a program into a target format.
type Exporter interface {
	Name() string
	Export(p *ladder.Program) (string, error)
}

// ScenarioParser reads scenario text.
type ScenarioParser interface {
	ParseScenario(src string) (*scenario.Script, error)
}

// Deps are the adapters a Workspace uses.
type Deps struct {
	Files     Files
	Notation  Notation
	Scenarios ScenarioParser
	Exporters []Exporter
}

// Workspace holds the program being worked on.
type Workspace struct {
	deps      Deps
	exporters map[string]Exporter
	session   *editing.Session
	path      string
	positions Positions
}

// New creates an empty workspace.
func New(d Deps) *Workspace {
	w := &Workspace{deps: d, exporters: map[string]Exporter{}}
	for _, e := range d.Exporters {
		w.exporters[e.Name()] = e
	}
	return w
}

// ErrNoProgram is returned when nothing is open yet. Its text is translated
// when shown, so it follows the language chosen at startup.
var ErrNoProgram error = noProgram{}

type noProgram struct{}

func (noProgram) Error() string {
	return i18n.T("プログラムが開かれていません（open か new_program を先に使います）")
}

// Open loads a program file and starts a fresh edit history.
func (w *Workspace) Open(path string) error {
	p, pos, err := w.deps.Files.LoadProgram(path)
	if err != nil {
		return err
	}
	w.session = editing.NewSession(p, w.deps.Notation)
	w.path, w.positions = path, pos
	return nil
}

// NewProgram starts an empty program for a profile. It has no file path
// until it is saved.
func (w *Workspace) NewProgram(profileName string) error {
	if _, err := profile.Get(profileName); err != nil {
		return err
	}
	w.session = editing.NewSession(&ladder.Program{Profile: profileName}, w.deps.Notation)
	w.path, w.positions = "", nil
	return nil
}

// Loaded reports whether a program is open.
func (w *Workspace) Loaded() bool { return w.session != nil }

// Path is the file the program came from or was last saved to.
func (w *Workspace) Path() string { return w.path }

// Dirty reports unsaved changes.
func (w *Workspace) Dirty() bool { return w.session != nil && w.session.Dirty() }

// Positions returns source positions from the last Open (nil after edits are
// not tracked; they refer to the file as it was opened).
func (w *Workspace) Positions() Positions { return w.positions }

// Program returns the current program. Callers must not modify it.
func (w *Workspace) Program() (*ladder.Program, error) {
	if w.session == nil {
		return nil, ErrNoProgram
	}
	return w.session.Program(), nil
}

// Save writes the program. An empty path saves to the current path.
func (w *Workspace) Save(path string) (string, error) {
	p, err := w.Program()
	if err != nil {
		return "", err
	}
	if path == "" {
		path = w.path
	}
	if path == "" {
		return "", i18n.Errorf("保存先のパスが必要です")
	}
	if err := w.deps.Files.SaveProgram(path, p); err != nil {
		return "", err
	}
	w.path = path
	w.session.MarkSaved()
	return path, nil
}

// Text writes the program in the notation, with IDs when withIDs is set.
func (w *Workspace) Text(withIDs bool) (string, error) {
	p, err := w.Program()
	if err != nil {
		return "", err
	}
	return w.deps.Notation.FormatProgram(p, withIDs)
}

// Check validates the program.
func (w *Workspace) Check() ([]check.Diagnostic, error) {
	p, err := w.Program()
	if err != nil {
		return nil, err
	}
	ds := check.Program(p)
	if ds == nil {
		ds = []check.Diagnostic{}
	}
	return ds, nil
}

// Edit applies edit commands atomically.
func (w *Workspace) Edit(ops []editing.Op) (*editing.Result, error) {
	if w.session == nil {
		return nil, ErrNoProgram
	}
	w.positions = nil // positions no longer match the edited program
	return w.session.Apply(ops)
}

// Undo reverts the last edit.
func (w *Workspace) Undo() error {
	if w.session == nil {
		return ErrNoProgram
	}
	return w.session.Undo()
}

// Redo re-applies the last undone edit.
func (w *Workspace) Redo() error {
	if w.session == nil {
		return ErrNoProgram
	}
	return w.session.Redo()
}

// Formats lists the export formats.
func (w *Workspace) Formats() []string {
	var names []string
	for n := range w.exporters {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Export converts the program to a format.
func (w *Workspace) Export(format string) (string, error) {
	p, err := w.Program()
	if err != nil {
		return "", err
	}
	e, ok := w.exporters[format]
	if !ok {
		return "", i18n.Errorf("書き出し形式 %q はありません（使えるもの: %s）", format, strings.Join(w.Formats(), ", "))
	}
	return e.Export(p)
}

// ScenarioOptions control RunScenario.
type ScenarioOptions struct {
	// Trace records watched operands after every scan. With no watch list
	// in the scenario, every operand the program uses is watched.
	Trace bool
	// Watch adds operands to record.
	Watch []string
	// MaxTrace caps the trace records per test (0: no cap). The MCP server
	// sets it so results fit in an agent's context.
	MaxTrace int
}

// RunScenario parses scenario text and runs it.
func (w *Workspace) RunScenario(src string, opt ScenarioOptions) ([]scenario.Result, error) {
	p, err := w.Program()
	if err != nil {
		return nil, err
	}
	sc, err := w.deps.Scenarios.ParseScenario(src)
	if err != nil {
		return nil, i18n.Errorf("シナリオ: %w", err)
	}
	sc.Watch = append(sc.Watch, opt.Watch...)
	if opt.Trace && len(sc.Watch) == 0 {
		sc.Watch = sim.Operands(p)
	}
	res, err := scenario.Run(p, sc)
	if err != nil {
		return nil, err
	}
	for i := range res {
		if !opt.Trace && len(opt.Watch) == 0 {
			res[i].Watch, res[i].Trace = nil, nil
		}
		res[i].Limit(opt.MaxTrace)
	}
	return res, nil
}

// RunScenarioFile reads a scenario file and runs it.
func (w *Workspace) RunScenarioFile(path string, opt ScenarioOptions) ([]scenario.Result, error) {
	src, err := w.deps.Files.ReadText(path)
	if err != nil {
		return nil, err
	}
	return w.RunScenario(src, opt)
}

// ValueFormatter returns a function that writes simulator values the way the
// program reads them (on/off for bits, 1.50s for timer times).
func (w *Workspace) ValueFormatter() func(op string, v int64) string {
	plain := func(_ string, v int64) string { return fmt.Sprint(v) }
	p, err := w.Program()
	if err != nil {
		return plain
	}
	s, err := sim.New(p)
	if err != nil {
		return plain
	}
	return s.Format
}
