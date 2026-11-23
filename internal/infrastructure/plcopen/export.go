// Package plcopen reads and writes PLCopen XML (TC6 v2.01) Ladder Diagram.
//
// Export turns each rung tree into a small graph of contacts, coils and
// blocks with its own left and right power rail, laid out on a grid derived
// from the tree. Import goes the other way: it rebuilds the power-flow graph
// and reduces it to a series-parallel tree; graphs that cannot be reduced
// (bridge circuits) are refused with E_NON_SERIES_PARALLEL.
//
// PLCopen is IEC 61131-3, so programs of the melsec profile are written with
// IEC spellings (K10 -> 10, H1F -> 16#1F, timer K50 -> T#5000ms) and the
// profile is recorded in addData, so Import can restore the MELSEC spellings.
package plcopen

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Piyoone3939/rungkit/internal/domain/check"
	"github.com/Piyoone3939/rungkit/internal/domain/i18n"
	"github.com/Piyoone3939/rungkit/internal/domain/ladder"
	"github.com/Piyoone3939/rungkit/internal/domain/profile"
)

const (
	nsTC6   = "http://www.plcopen.org/xml/tc6_0201"
	nsXHTML = "http://www.w3.org/1999/xhtml"
	// profileData names the addData entry that records the rungkit profile.
	profileData = "https://github.com/Piyoone3939/rungkit/profile"

	colW, rowH = 120, 80 // grid cell
	elemW      = 40      // contact / coil size
	elemH      = 20
	rungGap    = 60
)

// Exporter is the workspace adapter for PLCopen XML.
type Exporter struct {
	// ProgramName is the POU name (default "Main").
	ProgramName string
	// Now gives the creation time written in the file header (default
	// time.Now). Tests set it for stable output.
	Now func() time.Time
}

// Name is the export format name.
func (Exporter) Name() string { return "plcopen" }

// Export writes p as PLCopen XML.
func (e Exporter) Export(p *ladder.Program) (string, error) {
	now := time.Now
	if e.Now != nil {
		now = e.Now
	}
	name := e.ProgramName
	if name == "" {
		name = "Main"
	}
	return Export(p, name, now())
}

// Export writes p as PLCopen XML with a POU named name.
func Export(p *ladder.Program, name string, created time.Time) (string, error) {
	if ds := check.Program(p); check.HasErrors(ds) {
		return "", i18n.Errorf("エラーがあるので書き出せません（rungkit check で確認できます）")
	}
	prof, err := profile.Get(p.Profile)
	if err != nil {
		return "", err
	}
	g := &gen{p: p, prof: prof, melsec: strings.HasPrefix(p.Profile, "melsec"), vars: map[string]string{}, outAt: map[conn][2]int{}}

	// Body first: it decides which helper instances need declaring.
	body := &xw{depth: 5}
	y := 20
	for _, r := range p.Rungs {
		var err error
		if y, err = g.rung(body, r, y); err != nil {
			return "", fmt.Errorf("rung %s: %w", r.ID, err)
		}
	}

	w := &xw{}
	w.b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	w.open("project", "xmlns", nsTC6, "xmlns:xhtml", nsXHTML)
	w.empty("fileHeader", "companyName", "", "productName", "rungkit", "productVersion", "0.1",
		"creationDateTime", created.Format("2006-01-02T15:04:05"))
	w.open("contentHeader", "name", name)
	w.open("coordinateInfo")
	for _, lang := range []string{"fbd", "ld", "sfc"} {
		w.open(lang)
		w.empty("scaling", "x", "1", "y", "1")
		w.close(lang)
	}
	w.close("coordinateInfo")
	w.close("contentHeader")
	w.open("types")
	w.empty("dataTypes")
	w.open("pous")
	w.open("pou", "name", name, "pouType", "program")
	w.open("interface")
	w.open("localVars")
	g.declarations(w)
	w.close("localVars")
	w.close("interface")
	w.open("body")
	w.open("LD")
	w.b.WriteString(body.b.String())
	w.close("LD")
	w.close("body")
	w.close("pou")
	w.close("pous")
	w.close("types")
	w.open("instances")
	// A default task so Beremiz / OpenPLC can build the project as is.
	w.open("configurations")
	w.open("configuration", "name", "config")
	w.open("resource", "name", "resource1")
	w.open("task", "name", "task0", "priority", "0", "interval", "T#10ms")
	w.empty("pouInstance", "name", "instance0", "typeName", name)
	w.close("task")
	w.close("resource")
	w.close("configuration")
	w.close("configurations")
	w.close("instances")
	w.open("addData")
	w.open("data", "name", profileData, "handleUnknown", "discard")
	w.empty("rungkit", "profile", p.Profile)
	w.close("data")
	w.close("addData")
	w.close("project")
	return w.b.String(), nil
}

type gen struct {
	p      *ladder.Program
	prof   profile.Profile
	melsec bool
	nextID int
	vars   map[string]string // name -> type, for the melsec profile and helpers
	cells  map[ladder.Element][2]int
	top    int // y of the rung being written
	// outAt holds the absolute position of every output point written so
	// far; wires repeat it, because Beremiz needs each wire's path.
	outAt map[conn][2]int
}

func (g *gen) id() int {
	g.nextID++
	return g.nextID
}

// conn is a connection to an output point: an element and, for blocks, the
// output parameter.
type conn struct {
	ref   int
	param string
}

func itoa(i int) string { return strconv.Itoa(i) }

// ---- declarations ----

func (g *gen) declare(name, typ string) {
	if _, ok := g.vars[name]; !ok {
		g.vars[name] = typ
	}
}

func (g *gen) declarations(w *xw) {
	type decl struct{ name, typ, addr, doc string }
	var ds []decl
	if g.melsec {
		// Commented devices first, in the order of the program's symbols, so
		// an import gives the same sym lines back; then the other devices.
		done := map[string]bool{}
		for _, s := range g.p.Symbols {
			if _, used := g.vars[s.Name]; !used {
				if _, err := g.value(s.Name); err != nil {
					continue
				}
			}
			ds = append(ds, decl{name: s.Name, typ: g.vars[s.Name], doc: s.Comment})
			done[s.Name] = true
		}
		var rest []decl
		for n, t := range g.vars {
			if !done[n] {
				rest = append(rest, decl{name: n, typ: t})
			}
		}
		sort.Slice(rest, func(i, j int) bool { return varOrder(rest[i].name) < varOrder(rest[j].name) })
		ds = append(ds, rest...)
	} else {
		for _, s := range g.p.Symbols {
			ds = append(ds, decl{s.Name, s.Type, s.Address, s.Comment})
		}
		var helpers []string
		for n := range g.vars {
			helpers = append(helpers, n)
		}
		sort.Strings(helpers)
		for _, n := range helpers {
			ds = append(ds, decl{name: n, typ: g.vars[n]})
		}
	}
	for _, d := range ds {
		attrs := []string{"name", d.name}
		if d.addr != "" {
			attrs = append(attrs, "address", d.addr)
		}
		w.open("variable", attrs...)
		w.open("type")
		switch d.typ {
		case "TON", "TOF", "TP", "CTU", "R_TRIG", "F_TRIG":
			w.empty("derived", "name", d.typ)
		default:
			w.empty(d.typ)
		}
		w.close("type")
		if d.doc != "" {
			w.open("documentation")
			w.text("xhtml:p", d.doc)
			w.close("documentation")
		}
		w.close("variable")
	}
}

func varOrder(n string) string {
	if strings.HasPrefix(n, "_") {
		return "1" + n
	}
	return "0" + n
}

// value writes an operand as an IEC expression and declares MELSEC devices.
func (g *gen) value(op string) (string, error) {
	o, err := g.prof.Resolve(op, g.p)
	if err != nil {
		return "", err
	}
	if !g.melsec {
		return op, nil
	}
	switch o.Class {
	case profile.Const:
		if op[0] == 'H' {
			return "16#" + op[1:], nil
		}
		return strconv.FormatInt(o.Value, 10), nil
	case profile.TimerInst:
		g.declare(op, "TON")
	case profile.CounterInst:
		g.declare(op, "CTU")
	case profile.Word:
		g.declare(op, "INT")
	case profile.Bit:
		if o.BitOfWord {
			word, _, _ := strings.Cut(op, ".")
			g.declare(word, "INT")
		} else {
			g.declare(op, "BOOL")
		}
	}
	return op, nil
}

func (g *gen) timerPreset(op string) (string, error) {
	if !g.melsec {
		return op, nil
	}
	o, err := g.prof.Resolve(op, g.p)
	if err != nil {
		return "", err
	}
	if o.Class != profile.Const {
		return "", i18n.Errorf("PLCopen XML への書き出しでは、タイマーの設定値は定数だけに対応しています（%s）", op)
	}
	return fmt.Sprintf("T#%dms", o.Value*100), nil
}

// ---- layout ----

// place gives every element a grid cell; a series grows to the right, a
// parallel grows downward. It returns the width and height in cells.
func (g *gen) place(n ladder.Node, col, row int) (int, int) {
	switch v := n.(type) {
	case *ladder.Series:
		w, h := 0, 0
		for _, it := range v.Items {
			iw, ih := g.place(it, col+w, row)
			w += iw
			h = max(h, ih)
		}
		return w, h
	case *ladder.Parallel:
		w, h := 0, 0
		for _, it := range v.Items {
			iw, ih := g.place(it, col, row+h)
			h += ih
			w = max(w, iw)
		}
		return w, h
	case ladder.Element:
		g.cells[v] = [2]int{col, row}
		return 1, 1
	}
	return 0, 0
}

// ---- rungs ----

func (g *gen) rung(w *xw, r *ladder.Rung, top int) (int, error) {
	g.cells = map[ladder.Element][2]int{}
	cols, rows := g.place(r.Body, 1, 0)
	height := rows * rowH
	if r.Comment != "" {
		w.open("comment", "localId", itoa(g.id()), "height", "20", "width", itoa(max(200, cols*colW)))
		w.empty("position", "x", "10", "y", itoa(top))
		w.open("content")
		w.text("xhtml:p", r.Comment)
		w.close("content")
		w.close("comment")
		top += 30
	}
	g.top = top
	left := g.id()
	w.open("leftPowerRail", "localId", itoa(left), "height", itoa(height), "width", "4")
	w.empty("position", "x", "20", "y", itoa(top))
	w.open("connectionPointOut", "formalParameter", "")
	w.empty("relPosition", "x", "4", "y", itoa(elemH/2))
	w.close("connectionPointOut")
	w.close("leftPowerRail")
	g.outAt[conn{ref: left}] = [2]int{24, top + elemH/2}

	outs, err := g.emit(w, r.Body, []conn{{ref: left}})
	if err != nil {
		return 0, err
	}
	right := g.id()
	w.open("rightPowerRail", "localId", itoa(right), "height", itoa(height), "width", "4")
	rx := colW*(cols+1) + 20
	w.empty("position", "x", itoa(rx), "y", itoa(top))
	for _, c := range outs {
		g.pointIn(w, []conn{c}, rx, top, 0, elemH/2)
	}
	w.close("rightPowerRail")
	return top + height + rungGap, nil
}

// connection writes a wire from c to the input point at (x, y). The path
// runs from the input back to the source's output point.
func (g *gen) connection(w *xw, c conn, x, y int) {
	attrs := []string{"refLocalId", itoa(c.ref)}
	if c.param != "" {
		attrs = append(attrs, "formalParameter", c.param)
	}
	src, ok := g.outAt[c]
	if !ok {
		w.empty("connection", attrs...)
		return
	}
	w.open("connection", attrs...)
	w.empty("position", "x", itoa(x), "y", itoa(y))
	w.empty("position", "x", itoa(src[0]), "y", itoa(src[1]))
	w.close("connection")
}

// pointIn writes an input point at (rx, ry) relative to an element at
// (ex, ey), wired to ins.
func (g *gen) pointIn(w *xw, ins []conn, ex, ey, rx, ry int) {
	w.open("connectionPointIn")
	w.empty("relPosition", "x", itoa(rx), "y", itoa(ry))
	for _, c := range ins {
		g.connection(w, c, ex+rx, ey+ry)
	}
	w.close("connectionPointIn")
}

// pointOut writes the output point c at (rx, ry) relative to an element at
// (ex, ey) and remembers where it is.
func (g *gen) pointOut(w *xw, c conn, ex, ey, rx, ry int) {
	w.open("connectionPointOut")
	w.empty("relPosition", "x", itoa(rx), "y", itoa(ry))
	w.close("connectionPointOut")
	g.outAt[c] = [2]int{ex + rx, ey + ry}
}

// pos returns the absolute position of an element's cell.
func (g *gen) pos(e ladder.Element) (int, int) {
	c := g.cells[e]
	return 20 + c[0]*colW - colW/2, g.top + c[1]*rowH
}

// emit writes n fed by ins and returns the output points that carry its
// power flow onward.
func (g *gen) emit(w *xw, n ladder.Node, ins []conn) ([]conn, error) {
	switch v := n.(type) {
	case *ladder.Series:
		cur := ins
		for _, it := range v.Items {
			var err error
			if cur, err = g.emit(w, it, cur); err != nil {
				return nil, err
			}
		}
		return cur, nil
	case *ladder.Parallel:
		var outs []conn
		for _, it := range v.Items {
			o, err := g.emit(w, it, ins)
			if err != nil {
				return nil, err
			}
			outs = append(outs, o...)
		}
		return outs, nil
	case *ladder.Contact:
		return g.contactOrCoil(w, "contact", v, v.Operand, ins, contactAttrs(v))
	case *ladder.Coil:
		if v.Mode == ladder.Reset {
			o, _ := g.prof.Resolve(v.Operand, g.p)
			switch o.Class {
			case profile.Word:
				return g.instruction(w, v, "MOVE", []string{g.zero()}, v.Operand, false, ins)
			case profile.TimerInst, profile.CounterInst:
				return nil, i18n.Errorf("タイマー・カウンタの RST（%s）は PLCopen XML に書き出せません", v.Operand)
			}
		}
		return g.contactOrCoil(w, "coil", v, v.Operand, ins, coilAttrs(v))
	case *ladder.Compare:
		return g.compare(w, v, ins)
	case *ladder.Timer:
		pt, err := g.timerPreset(v.Preset)
		if err != nil {
			return nil, err
		}
		inst, err := g.value(v.Instance)
		if err != nil {
			return nil, err
		}
		if g.melsec {
			g.vars[inst] = v.Kind
		}
		return g.fb(w, v, v.Kind, inst, "IN", "PT", pt, ins)
	case *ladder.Counter:
		pv, err := g.value(v.Preset)
		if err != nil {
			return nil, err
		}
		inst, err := g.value(v.Instance)
		if err != nil {
			return nil, err
		}
		return g.fb(w, v, "CTU", inst, "CU", "PV", pv, ins)
	case *ladder.Instr:
		return g.instr(w, v, ins)
	}
	return nil, fmt.Errorf("unexpected node %T", n)
}

func (g *gen) zero() string {
	if g.melsec {
		return "K0"
	}
	return "0"
}

func contactAttrs(c *ladder.Contact) []string {
	neg, edge := "false", "none"
	switch c.Mode {
	case ladder.NC:
		neg = "true"
	case ladder.Rise:
		edge = "rising"
	case ladder.Fall:
		edge = "falling"
	}
	return []string{"negated", neg, "edge", edge}
}

func coilAttrs(c *ladder.Coil) []string {
	storage, edge := "none", "none"
	switch c.Mode {
	case ladder.Set:
		storage = "set"
	case ladder.Reset:
		storage = "reset"
	case ladder.Pulse:
		edge = "rising"
	case ladder.PulseFall:
		edge = "falling"
	}
	return []string{"negated", "false", "storage", storage, "edge", edge}
}

func (g *gen) contactOrCoil(w *xw, tag string, e ladder.Element, op string, ins []conn, attrs []string) ([]conn, error) {
	v, err := g.value(op)
	if err != nil {
		return nil, err
	}
	if tag == "contact" {
		// A contact on a timer or counter reads its output Q in IEC.
		if o, err := g.prof.Resolve(op, g.p); err == nil && (o.Class == profile.TimerInst || o.Class == profile.CounterInst) {
			v += ".Q"
		}
	}
	id := g.id()
	x, y := g.pos(e)
	w.open(tag, append([]string{"localId", itoa(id)}, append(attrs, "height", itoa(elemH), "width", itoa(elemW))...)...)
	w.empty("position", "x", itoa(x), "y", itoa(y))
	g.pointIn(w, ins, x, y, 0, elemH/2)
	g.pointOut(w, conn{ref: id}, x, y, elemW, elemH/2)
	w.text("variable", v)
	w.close(tag)
	return []conn{{ref: id}}, nil
}

// inVariable writes a data source and returns its id.
func (g *gen) inVariable(w *xw, expr string, x, y int) int {
	id := g.id()
	w.open("inVariable", "localId", itoa(id), "height", itoa(elemH), "width", "60", "negated", "false")
	w.empty("position", "x", itoa(x), "y", itoa(y))
	g.pointOut(w, conn{ref: id}, x, y, 60, elemH/2)
	w.text("expression", expr)
	w.close("inVariable")
	return id
}

func (g *gen) outVariable(w *xw, expr string, from conn, x, y int) {
	id := g.id()
	w.open("outVariable", "localId", itoa(id), "height", itoa(elemH), "width", "60", "negated", "false")
	w.empty("position", "x", itoa(x), "y", itoa(y))
	g.pointIn(w, []conn{from}, x, y, 0, elemH/2)
	w.text("expression", expr)
	w.close("outVariable")
}

type blockIn struct {
	param string
	ins   []conn
}

// block writes a block. Inputs connect to power or data; outputs are listed
// by name.
func (g *gen) block(w *xw, id int, typeName, instance string, x, y int, inputs []blockIn, outputs []string) {
	attrs := []string{"localId", itoa(id), "typeName", typeName}
	if instance != "" {
		attrs = append(attrs, "instanceName", instance)
	}
	h := 20 * (max(len(inputs), len(outputs)) + 1)
	w.open("block", append(attrs, "height", itoa(h), "width", "60")...)
	w.empty("position", "x", itoa(x), "y", itoa(y))
	w.open("inputVariables")
	for k, in := range inputs {
		w.open("variable", "formalParameter", in.param)
		g.pointIn(w, in.ins, x, y, 0, 20*(k+1))
		w.close("variable")
	}
	w.close("inputVariables")
	w.empty("inOutVariables")
	w.open("outputVariables")
	for k, out := range outputs {
		w.open("variable", "formalParameter", out)
		g.pointOut(w, conn{ref: id, param: out}, x, y, 60, 20*(k+1))
		w.close("variable")
	}
	w.close("outputVariables")
	w.close("block")
}

// fb writes a timer or counter: power into powerIn, a preset into dataIn,
// power onward from Q.
func (g *gen) fb(w *xw, e ladder.Element, typ, inst, powerIn, dataIn, preset string, ins []conn) ([]conn, error) {
	x, y := g.pos(e)
	src := g.inVariable(w, preset, x-70, y+rowH/2)
	id := g.id()
	outs := []string{"Q", "ET"}
	if typ == "CTU" {
		outs = []string{"Q", "CV"}
	}
	g.block(w, id, typ, inst, x, y, []blockIn{{powerIn, ins}, {dataIn, []conn{{ref: src}}}}, outs)
	return []conn{{ref: id, param: "Q"}}, nil
}

var compareBlocks = map[string]string{"=": "EQ", "<>": "NE", "<": "LT", "<=": "LE", ">": "GT", ">=": "GE"}

// compare writes a comparison block: power into EN, the result OUT carries
// the power onward.
func (g *gen) compare(w *xw, c *ladder.Compare, ins []conn) ([]conn, error) {
	a, err := g.value(c.LHS)
	if err != nil {
		return nil, err
	}
	b, err := g.value(c.RHS)
	if err != nil {
		return nil, err
	}
	x, y := g.pos(c)
	in1 := g.inVariable(w, a, x-70, y+rowH/4)
	in2 := g.inVariable(w, b, x-70, y+rowH/2)
	id := g.id()
	g.block(w, id, compareBlocks[c.Op], "", x, y,
		[]blockIn{{"EN", ins}, {"IN1", []conn{{ref: in1}}}, {"IN2", []conn{{ref: in2}}}}, []string{"ENO", "OUT"})
	return []conn{{ref: id, param: "OUT"}}, nil
}

var instrBlocks = map[string]string{"MOV": "MOVE", "ADD": "ADD", "SUB": "SUB", "MUL": "MUL", "DIV": "DIV"}

func (g *gen) instr(w *xw, v *ladder.Instr, ins []conn) ([]conn, error) {
	spec, ok := g.prof.Instr(v.Name)
	if !ok {
		return nil, i18n.Errorf("命令 %s は使えません", v.Name)
	}
	switch spec.Op {
	case "MOV":
		return g.instruction(w, v, "MOVE", v.Args[:1], v.Args[1], spec.Pulse, ins)
	case "ADD", "SUB", "MUL", "DIV":
		return g.instruction(w, v, spec.Op, v.Args[:2], v.Args[2], spec.Pulse, ins)
	case "INC":
		return g.instruction(w, v, "ADD", []string{v.Args[0], g.one()}, v.Args[0], spec.Pulse, ins)
	case "DEC":
		return g.instruction(w, v, "SUB", []string{v.Args[0], g.one()}, v.Args[0], spec.Pulse, ins)
	}
	return nil, i18n.Errorf("命令 %s は PLCopen XML に書き出せません", v.Name)
}

func (g *gen) one() string {
	if g.melsec {
		return "K1"
	}
	return "1"
}

// instruction writes an EN/ENO block whose OUT goes to dst. A pulse
// instruction is fed through an R_TRIG instance.
func (g *gen) instruction(w *xw, e ladder.Element, typ string, srcs []string, dst string, pulse bool, ins []conn) ([]conn, error) {
	x, y := g.pos(e)
	if pulse {
		inst := "_p_" + e.ElemID()
		g.declare(inst, "R_TRIG")
		rt := g.id()
		g.block(w, rt, "R_TRIG", inst, x-40, y+rowH/2, []blockIn{{"CLK", ins}}, []string{"Q"})
		ins = []conn{{ref: rt, param: "Q"}}
	}
	inputs := []blockIn{{"EN", ins}}
	params := []string{"IN"}
	if len(srcs) == 2 {
		params = []string{"IN1", "IN2"}
	}
	for i, s := range srcs {
		expr, err := g.value(s)
		if err != nil {
			return nil, err
		}
		src := g.inVariable(w, expr, x-70, y+(i+1)*rowH/4)
		inputs = append(inputs, blockIn{params[i], []conn{{ref: src}}})
	}
	dstExpr, err := g.value(dst)
	if err != nil {
		return nil, err
	}
	id := g.id()
	g.block(w, id, typ, "", x, y, inputs, []string{"ENO", "OUT"})
	g.outVariable(w, dstExpr, conn{ref: id, param: "OUT"}, x+80, y+rowH/2)
	return []conn{{ref: id, param: "ENO"}}, nil
}
