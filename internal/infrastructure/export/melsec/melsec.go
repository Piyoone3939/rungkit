// Package melsec writes a program as a MELSEC instruction list
// (LD / AND / OR / ANB / ORB / MPS / MRD / MPP ...).
//
// Two layouts are written from the same instruction sequence:
//   - melsec: one instruction per line, operands separated by tabs, "; "
//     comment lines for rung comments (for reading and diffs),
//   - gxw3: the listed-instruction CSV that GX Works3 imports with
//     [Edit] -> [Import File] (GX Works3 Operating Manual, "Importing from a
//     CSV file"), written as GX Works3 itself writes it: UTF-16 LE with a
//     BOM, tab separated, every item quoted, CR LF.
package melsec

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/Piyoone3939/rungkit/internal/domain/check"
	"github.com/Piyoone3939/rungkit/internal/domain/i18n"
	"github.com/Piyoone3939/rungkit/internal/domain/ladder"
	"github.com/Piyoone3939/rungkit/internal/domain/profile"
)

// AlwaysOn is the always-ON special relay used for unconditional outputs.
const AlwaysOn = "SM400"

// row is one line of the list: a statement (rung comment) or an instruction.
type row struct {
	stmt string
	ins  string
	ops  []string
}

// toRows converts p into rows ending with END. It refuses programs that have
// check errors or that target another vendor.
func toRows(p *ladder.Program) ([]row, error) {
	if !strings.HasPrefix(p.Profile, "melsec") {
		return nil, i18n.Errorf("三菱リスト形式は melsec のプロファイルでだけ書き出せます（今は %s）", p.Profile)
	}
	if ds := check.Program(p); check.HasErrors(ds) {
		return nil, i18n.Errorf("エラーがあるので書き出せません（rungkit check で確認できます）")
	}
	prof, _ := profile.Get(p.Profile)
	w := &writer{prof: prof}
	for _, r := range p.Rungs {
		if r.Comment != "" {
			w.rows = append(w.rows, row{stmt: r.Comment})
		}
		if err := w.outputs(r.Body, true); err != nil {
			return nil, fmt.Errorf("rung %s: %w", r.ID, err)
		}
	}
	w.emit("END")
	return w.rows, nil
}

// Export converts p to the plain list.
func Export(p *ladder.Program) (string, error) {
	rows, err := toRows(p)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, r := range rows {
		if r.ins == "" {
			b.WriteString("; " + r.stmt + "\n")
			continue
		}
		b.WriteString(strings.Join(append([]string{r.ins}, r.ops...), "\t") + "\n")
	}
	return b.String(), nil
}

// moduleTypes is the "Module Type Information" written for each profile.
// GX Works3 writes the series and model of the project here; whether import
// checks it is not known, so the models are only representative.
var moduleTypes = map[string]string{
	"melsec-iqr": "RCPU R08",
	"melsec-fx5": "FX5CPU FX5U",
}

// CSV converts p to the GX Works3 listed-instruction CSV as text (UTF-8).
// Use EncodeFile for the bytes GX Works3 reads.
func CSV(p *ladder.Program, project string) (string, error) {
	rows, err := toRows(p)
	if err != nil {
		return "", err
	}
	if project == "" {
		project = "Main"
	}
	var b strings.Builder
	line := func(items ...string) {
		for i, it := range items {
			if i > 0 {
				b.WriteByte('\t')
			}
			b.WriteString(`"` + strings.ReplaceAll(it, `"`, `""`) + `"`)
		}
		b.WriteString("\r\n")
	}
	line(project)
	line("Module Type Information:", moduleTypes[p.Profile])
	line("Step No.", "Line Statement", "Instruction", "I/O (Device)", "Blank", "PI Statement", "Note")
	n := 0
	next := func() string { s := strconv.Itoa(n); n++; return s }
	for _, r := range rows {
		if r.ins == "" {
			// A line statement is a single line; GX Works3 cuts at a line feed.
			stmt := strings.Join(strings.Fields(r.stmt), " ")
			line(next(), stmt, "", "", "", "", "")
			continue
		}
		// One argument per row: the instruction with its first argument,
		// then a row per further argument (format 1 of the manual).
		first := ""
		if len(r.ops) > 0 {
			first = r.ops[0]
		}
		line(next(), "", r.ins, first, "", "", "")
		for _, op := range r.ops[min(1, len(r.ops)):] {
			line(next(), "", "", op, "", "", "")
		}
	}
	return b.String(), nil
}

// EncodeFile turns CSV text into UTF-16 LE with a byte order mark, the
// encoding GX Works3 uses for this file.
func EncodeFile(text string) []byte {
	u := utf16.Encode([]rune(text))
	out := make([]byte, 2, 2+2*len(u))
	out[0], out[1] = 0xFF, 0xFE
	for _, c := range u {
		out = append(out, byte(c), byte(c>>8))
	}
	return out
}

type writer struct {
	prof profile.Profile
	rows []row
}

func (w *writer) emit(ins string, ops ...string) {
	w.rows = append(w.rows, row{ins: ins, ops: ops})
}

// outputs writes an Output node. atRail means no condition has been loaded
// yet (the node hangs directly off the left rail).
func (w *writer) outputs(n ladder.Node, atRail bool) error {
	switch v := n.(type) {
	case *ladder.Series:
		prefix, tail := v.Items[:len(v.Items)-1], v.Items[len(v.Items)-1]
		for i, it := range prefix {
			if i == 0 && atRail {
				w.load(it)
			} else {
				w.and(it)
			}
		}
		return w.outputs(tail, false)
	case *ladder.Parallel:
		if atRail {
			// Separate circuits sharing the rail: each loads its own condition.
			for _, it := range v.Items {
				if err := w.outputs(it, true); err != nil {
					return err
				}
			}
			return nil
		}
		return w.branch(v.Items)
	case ladder.Element:
		if atRail {
			w.emit("LD", AlwaysOn)
		}
		return w.output(v)
	}
	return fmt.Errorf("unexpected node %T", n)
}

// branch writes an output branch. The operation result only needs saving
// (MPS) when a branch other than the last one adds conditions of its own.
func (w *writer) branch(items []ladder.Node) error {
	modifies := func(n ladder.Node) bool { _, ok := n.(*ladder.Series); return ok }
	laterModifier := func(from int) bool { // any modifier in items[from : len-1]
		for j := from; j < len(items)-1; j++ {
			if modifies(items[j]) {
				return true
			}
		}
		return false
	}
	pushed, dirty := false, false
	for i, it := range items {
		if dirty {
			if laterModifier(i) {
				w.emit("MRD")
			} else {
				w.emit("MPP")
				pushed = false
			}
			dirty = false
		}
		if modifies(it) && i < len(items)-1 && !pushed {
			w.emit("MPS")
			pushed = true
		}
		if err := w.outputs(it, false); err != nil {
			return err
		}
		if modifies(it) {
			dirty = true
		}
	}
	return nil
}

// load starts a new logic block with n.
func (w *writer) load(n ladder.Node) {
	switch v := n.(type) {
	case *ladder.Series:
		w.load(v.Items[0])
		for _, it := range v.Items[1:] {
			w.and(it)
		}
	case *ladder.Parallel:
		w.load(v.Items[0])
		for _, it := range v.Items[1:] {
			w.or(it)
		}
	default:
		w.contact("LD", n)
	}
}

// and connects n in series with the current block.
func (w *writer) and(n ladder.Node) {
	switch v := n.(type) {
	case *ladder.Parallel:
		w.load(v)
		w.emit("ANB")
	case *ladder.Series:
		for _, it := range v.Items {
			w.and(it)
		}
	default:
		w.contact("AND", n)
	}
}

// or connects n in parallel with the current block.
func (w *writer) or(n ladder.Node) {
	switch v := n.(type) {
	case *ladder.Series:
		w.load(v)
		w.emit("ORB")
	case *ladder.Parallel:
		for _, it := range v.Items {
			w.or(it)
		}
	default:
		w.contact("OR", n)
	}
}

// contact writes LD/AND/OR forms of a single condition element.
func (w *writer) contact(base string, n ladder.Node) {
	switch v := n.(type) {
	case *ladder.Contact:
		suffix := map[ladder.ContactMode]string{ladder.NO: "", ladder.NC: "I", ladder.Rise: "P", ladder.Fall: "F"}[v.Mode]
		ins := base + suffix
		if base == "AND" && v.Mode == ladder.NC {
			ins = "ANI" // AND + I is spelled ANI
		}
		w.emit(ins, v.Operand)
	case *ladder.Compare:
		w.emit(base+v.Op, v.LHS, v.RHS)
	}
}

func (w *writer) output(e ladder.Element) error {
	switch v := e.(type) {
	case *ladder.Coil:
		w.emit(string(v.Mode), v.Operand)
	case *ladder.Timer:
		w.emit("OUT", v.Instance, v.Preset)
	case *ladder.Counter:
		w.emit("OUT", v.Instance, v.Preset)
	case *ladder.Instr:
		spec, ok := w.prof.Instr(v.Name)
		if !ok {
			return i18n.Errorf("命令 %s は使えません", v.Name)
		}
		w.emit(spec.Mnemonic, v.Args...)
	default:
		return i18n.Errorf("出力ではない要素 %T", e)
	}
	return nil
}

// Exporter is the workspace adapter for this format.
type Exporter struct{}

// Name is the export format name.
func (Exporter) Name() string { return "melsec" }

// Export converts p.
func (Exporter) Export(p *ladder.Program) (string, error) { return Export(p) }

// CSVExporter is the workspace adapter for the GX Works3 CSV.
type CSVExporter struct {
	ProjectName string
}

// Name is the export format name.
func (CSVExporter) Name() string { return "gxw3" }

// Export converts p to the CSV as text.
func (e CSVExporter) Export(p *ladder.Program) (string, error) { return CSV(p, e.ProjectName) }

// EncodeFile gives the bytes of the file GX Works3 imports.
func (CSVExporter) EncodeFile(text string) []byte { return EncodeFile(text) }
