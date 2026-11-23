// Package sim runs a ladder program scan by scan on virtual time.
//
// Semantics follow the ladder, not a re-evaluation of logic:
//   - rungs run top to bottom; every instruction executes every scan (no
//     short-circuit), so edge contacts and timers always see their input,
//   - outputs are written immediately, so later rungs see the new value,
//   - an output branch keeps the power flow it started with (MELSEC MPS),
//     while contacts inside the branch read the current memory,
//   - time advances by a fixed scan time, so results never depend on the
//     machine running the simulation.
package sim

import (
	"fmt"
	"strconv"
	"strings"

	"rungkit/internal/domain/check"
	"rungkit/internal/domain/ladder"
	"rungkit/internal/domain/profile"
)

// DefaultScanMs is the scan time used unless SetScanTime is called.
const DefaultScanMs = 10

// RuntimeError is an error raised while executing an instruction.
type RuntimeError struct {
	Scan int // the scan that failed (1-based)
	Rung string
	Elem string
	Msg  string
}

func (e *RuntimeError) Error() string {
	return fmt.Sprintf("スキャン %d, %s/%s: %s", e.Scan, e.Rung, e.Elem, e.Msg)
}

// Record is the state of the watched operands after one scan.
type Record struct {
	Scan   int     `json:"scan"`
	TimeMs int64   `json:"time_ms"`
	Values []int64 `json:"values"`
}

type timerState struct {
	et      int64
	q       bool
	prevIn  bool
	running bool
}

type counterState struct {
	cv   int64
	q    bool
	prev bool
}

// Sim is a running program.
type Sim struct {
	p      *ladder.Program
	prof   profile.Profile
	melsec bool
	scanMs int64

	scans    int
	now      int64
	bits     map[string]bool
	words    map[string]int64
	timers   map[string]*timerState
	counters map[string]*counterState
	prev     map[string]bool // per element: last input of edges and pulses
	forced   map[string]int64
	cache    map[string]profile.Operand

	watch []string
	trace []Record

	rung, elem string // where execution is, for runtime errors
}

// New prepares p for simulation. Programs with check errors are refused.
func New(p *ladder.Program) (*Sim, error) {
	if ds := check.Program(p); check.HasErrors(ds) {
		d := ds[0]
		for _, x := range ds {
			if x.Severity == check.Error {
				d = x
				break
			}
		}
		return nil, fmt.Errorf("エラーがあるので動かせません: %s %s", d.Code, d.Message)
	}
	prof, _ := profile.Get(p.Profile)
	s := &Sim{p: p, prof: prof, melsec: strings.HasPrefix(p.Profile, "melsec"), scanMs: DefaultScanMs, cache: map[string]profile.Operand{}}
	s.Reset()
	return s, nil
}

// Reset clears memory, timers, forces, the trace and the clock.
func (s *Sim) Reset() {
	s.scans, s.now = 0, 0
	s.bits = map[string]bool{}
	s.words = map[string]int64{}
	s.timers = map[string]*timerState{}
	s.counters = map[string]*counterState{}
	s.prev = map[string]bool{}
	s.forced = map[string]int64{}
	s.trace = nil
}

// SetScanTime sets the virtual time one scan takes.
func (s *Sim) SetScanTime(ms int64) error {
	if ms <= 0 {
		return fmt.Errorf("スキャン時間は1ms以上です")
	}
	s.scanMs = ms
	return nil
}

// ScanTime returns the scan time in ms.
func (s *Sim) ScanTime() int64 { return s.scanMs }

// Scans returns the number of completed scans.
func (s *Sim) Scans() int { return s.scans }

// Now returns the virtual time in ms.
func (s *Sim) Now() int64 { return s.now }

// Watch adds operands whose values are recorded after every scan.
func (s *Sim) Watch(ops ...string) error {
	for _, op := range ops {
		if _, err := s.Get(op); err != nil {
			return err
		}
		dup := false
		for _, w := range s.watch {
			dup = dup || w == op
		}
		if !dup {
			s.watch = append(s.watch, op)
		}
	}
	return nil
}

// Watched returns the watched operands in column order.
func (s *Sim) Watched() []string { return s.watch }

// Trace returns the records collected so far.
func (s *Sim) Trace() []Record { return s.trace }

// Run executes n scans.
func (s *Sim) Run(n int) error {
	for i := 0; i < n; i++ {
		if err := s.Scan(); err != nil {
			return err
		}
	}
	return nil
}

// Scan executes one scan.
func (s *Sim) Scan() error {
	if s.melsec {
		s.bits["SM400"] = true         // always ON
		s.bits["SM401"] = false        // always OFF
		s.bits["SM402"] = s.scans == 0 // ON for the first scan only
		s.bits["SM403"] = s.scans != 0
	}
	for _, r := range s.p.Rungs {
		s.rung = r.ID
		if err := s.execOut(r.Body, true); err != nil {
			return err
		}
	}
	s.scans++
	s.now += s.scanMs
	if len(s.watch) > 0 {
		rec := Record{Scan: s.scans, TimeMs: s.now, Values: make([]int64, len(s.watch))}
		for i, op := range s.watch {
			rec.Values[i], _ = s.Get(op)
		}
		s.trace = append(s.trace, rec)
	}
	return nil
}

func (s *Sim) fail(format string, args ...any) error {
	return &RuntimeError{Scan: s.scans + 1, Rung: s.rung, Elem: s.elem, Msg: fmt.Sprintf(format, args...)}
}

// ---- execution ----

func (s *Sim) execOut(n ladder.Node, power bool) error {
	switch v := n.(type) {
	case *ladder.Series:
		last := len(v.Items) - 1
		for _, it := range v.Items[:last] {
			ok, err := s.evalCond(it)
			if err != nil {
				return err
			}
			power = power && ok
		}
		return s.execOut(v.Items[last], power)
	case *ladder.Parallel:
		for _, it := range v.Items {
			if err := s.execOut(it, power); err != nil {
				return err
			}
		}
		return nil
	case ladder.Element:
		s.elem = v.ElemID()
		return s.execElem(v, power)
	}
	return fmt.Errorf("unexpected node %T", n)
}

// evalCond evaluates every element (no short-circuit).
func (s *Sim) evalCond(n ladder.Node) (bool, error) {
	switch v := n.(type) {
	case *ladder.Series:
		r := true
		for _, it := range v.Items {
			ok, err := s.evalCond(it)
			if err != nil {
				return false, err
			}
			r = ok && r
		}
		return r, nil
	case *ladder.Parallel:
		r := false
		for _, it := range v.Items {
			ok, err := s.evalCond(it)
			if err != nil {
				return false, err
			}
			r = ok || r
		}
		return r, nil
	case *ladder.Contact:
		s.elem = v.ID
		cur, err := s.readBool(v.Operand)
		if err != nil {
			return false, err
		}
		switch v.Mode {
		case ladder.NC:
			return !cur, nil
		case ladder.Rise, ladder.Fall:
			prev := s.prev[v.ID]
			s.prev[v.ID] = cur
			if v.Mode == ladder.Rise {
				return cur && !prev, nil
			}
			return !cur && prev, nil
		}
		return cur, nil
	case *ladder.Compare:
		s.elem = v.ID
		a, err := s.read(v.LHS)
		if err != nil {
			return false, err
		}
		b, err := s.read(v.RHS)
		if err != nil {
			return false, err
		}
		switch v.Op {
		case "=":
			return a == b, nil
		case "<>":
			return a != b, nil
		case "<":
			return a < b, nil
		case "<=":
			return a <= b, nil
		case ">":
			return a > b, nil
		case ">=":
			return a >= b, nil
		}
		return false, s.fail("比較演算子 %q は使えません", v.Op)
	}
	return false, fmt.Errorf("unexpected condition %T", n)
}

// edge returns true when power rises, remembering it under key.
func (s *Sim) edge(key string, power bool) bool {
	prev := s.prev[key]
	s.prev[key] = power
	return power && !prev
}

func (s *Sim) execElem(e ladder.Element, power bool) error {
	switch v := e.(type) {
	case *ladder.Coil:
		switch v.Mode {
		case ladder.Out:
			return s.write(v.Operand, b2i(power))
		case ladder.Set:
			if power {
				return s.write(v.Operand, 1)
			}
		case ladder.Reset:
			if power {
				return s.reset(v.Operand)
			}
		case ladder.Pulse:
			return s.write(v.Operand, b2i(s.edge(v.ID, power)))
		case ladder.PulseFall:
			prev := s.prev[v.ID]
			s.prev[v.ID] = power
			return s.write(v.Operand, b2i(!power && prev))
		}
		return nil
	case *ladder.Timer:
		return s.execTimer(v, power)
	case *ladder.Counter:
		return s.execCounter(v, power)
	case *ladder.Instr:
		return s.execInstr(v, power)
	}
	return fmt.Errorf("unexpected element %T", e)
}

func (s *Sim) timer(name string) *timerState {
	t, ok := s.timers[name]
	if !ok {
		t = &timerState{}
		s.timers[name] = t
	}
	return t
}

func (s *Sim) counter(name string) *counterState {
	c, ok := s.counters[name]
	if !ok {
		c = &counterState{}
		s.counters[name] = c
	}
	return c
}

func (s *Sim) execTimer(v *ladder.Timer, in bool) error {
	pt, err := s.presetMs(v.Preset)
	if err != nil {
		return err
	}
	t := s.timer(v.Instance)
	dt := s.scanMs
	switch v.Kind {
	case "TON":
		if in {
			if t.prevIn {
				t.et += dt
			} else {
				t.et = 0 // measuring starts at the scan the input turns on
			}
			if t.et > pt {
				t.et = pt
			}
			t.q = t.et >= pt
		} else {
			t.et, t.q = 0, false
		}
	case "TOF":
		if in {
			t.q, t.et, t.running = true, 0, false
		} else {
			if t.prevIn {
				t.running, t.et = true, 0
			} else if t.running {
				t.et += dt
			}
			if t.running && t.et >= pt {
				t.et, t.q, t.running = pt, false, false
			}
		}
	case "TP":
		if in && !t.prevIn && !t.running {
			t.running, t.q, t.et = true, true, 0
		} else if t.running {
			t.et += dt
		}
		if t.running && t.et >= pt {
			t.et, t.q, t.running = pt, false, false
		}
		if !in && !t.running {
			t.et = 0
		}
	default:
		return s.fail("タイマー %s には対応していません", v.Kind)
	}
	t.prevIn = in
	return nil
}

// presetMs converts a timer preset to milliseconds. MELSEC T timers count in
// 100 ms units, so K50 is 5000 ms.
func (s *Sim) presetMs(op string) (int64, error) {
	o, err := s.resolve(op)
	if err != nil {
		return 0, err
	}
	if o.Class == profile.TimeConst {
		return o.Value, nil
	}
	v, err := s.read(op)
	if err != nil {
		return 0, err
	}
	if s.melsec {
		v *= 100
	}
	if v < 0 {
		return 0, s.fail("タイマーの設定値 %s が負の値 (%d) です", op, v)
	}
	return v, nil
}

func (s *Sim) execCounter(v *ladder.Counter, power bool) error {
	pv, err := s.read(v.Preset)
	if err != nil {
		return err
	}
	c := s.counter(v.Instance)
	if power && !c.prev {
		limit := int64(32767)
		if s.melsec {
			limit = pv // a MELSEC counter stops at its set value
		}
		if c.cv < limit {
			c.cv++
		}
	}
	c.prev = power
	c.q = c.cv >= pv
	return nil
}

func (s *Sim) execInstr(v *ladder.Instr, power bool) error {
	spec, ok := s.prof.Instr(v.Name)
	if !ok {
		return s.fail("命令 %s は使えません", v.Name)
	}
	fire := power
	if spec.Pulse {
		fire = s.edge(v.ID, power)
	}
	if !fire {
		return nil
	}
	val := func(i int) (int64, error) { return s.read(v.Args[i]) }
	switch spec.Op {
	case "MOV":
		a, err := val(0)
		if err != nil {
			return err
		}
		return s.write(v.Args[1], a)
	case "ADD", "SUB", "MUL", "DIV":
		a, err := val(0)
		if err != nil {
			return err
		}
		b, err := val(1)
		if err != nil {
			return err
		}
		d := v.Args[2]
		switch spec.Op {
		case "ADD":
			return s.write(d, a+b)
		case "SUB":
			return s.write(d, a-b)
		case "MUL":
			if s.melsec { // 32-bit result in D(n) (low) and D(n+1) (high)
				r := int64(int32(a * b))
				if err := s.write(d, r&0xFFFF); err != nil {
					return err
				}
				return s.writeNext(d, r>>16)
			}
			return s.write(d, a*b)
		case "DIV":
			if b == 0 {
				return s.fail("0 で割りました（%s）", v.Args[1])
			}
			if s.melsec { // quotient in D(n), remainder in D(n+1)
				if err := s.write(d, a/b); err != nil {
					return err
				}
				return s.writeNext(d, a%b)
			}
			return s.write(d, a/b)
		}
	case "INC", "DEC":
		a, err := val(0)
		if err != nil {
			return err
		}
		if spec.Op == "INC" {
			return s.write(v.Args[0], a+1)
		}
		return s.write(v.Args[0], a-1)
	}
	return s.fail("命令 %s はまだシミュレーションできません", v.Name)
}

// writeNext writes to the device after op (D0 -> D1, W1F -> W20).
func (s *Sim) writeNext(op string, v int64) error {
	o, err := s.resolve(op)
	if err != nil {
		return err
	}
	base := 10
	if o.Hex {
		base = 16
	}
	return s.write(o.Device+strings.ToUpper(strconv.FormatInt(o.Index+1, base)), v)
}

func (s *Sim) reset(op string) error {
	o, err := s.resolve(op)
	if err != nil {
		return err
	}
	switch o.Class {
	case profile.TimerInst:
		t := s.timer(op)
		t.et, t.q, t.running = 0, false, false
		return nil
	case profile.CounterInst:
		c := s.counter(op)
		c.cv, c.q = 0, false
		return nil
	}
	return s.write(op, 0)
}

// ---- memory ----

func (s *Sim) resolve(op string) (profile.Operand, error) {
	if o, ok := s.cache[op]; ok {
		return o, nil
	}
	o, err := s.prof.Resolve(op, s.p)
	if err != nil {
		return o, err
	}
	s.cache[op] = o
	return o, nil
}

func (s *Sim) readBool(op string) (bool, error) {
	v, err := s.read(op)
	return v != 0, err
}

func (s *Sim) read(op string) (int64, error) {
	if v, ok := s.forced[op]; ok {
		return v, nil
	}
	o, err := s.resolve(op)
	if err != nil {
		return 0, err
	}
	switch o.Class {
	case profile.Const:
		return o.Value, nil
	case profile.TimeConst:
		return o.Value, nil
	case profile.Word:
		return s.words[op], nil
	case profile.TimerInst:
		return b2i(s.timer(op).q), nil
	case profile.CounterInst:
		return b2i(s.counter(op).q), nil
	}
	if o.BitOfWord {
		word, bit := splitBit(op)
		return (s.words[word] >> bit) & 1, nil
	}
	return b2i(s.bits[op]), nil
}

func (s *Sim) write(op string, v int64) error {
	if _, ok := s.forced[op]; ok {
		return nil // a forced operand keeps its forced value
	}
	o, err := s.resolve(op)
	if err != nil {
		return err
	}
	switch o.Class {
	case profile.Word:
		s.words[op] = s.wrap(o, v)
		return nil
	case profile.Bit:
		if o.BitOfWord {
			word, bit := splitBit(op)
			w := s.words[word]
			if v != 0 {
				w |= 1 << bit
			} else {
				w &^= 1 << bit
			}
			s.words[word] = int64(int16(w))
			return nil
		}
		s.bits[op] = v != 0
		return nil
	}
	return s.fail("%s（%s）には値を書けません", op, o.Class)
}

// wrap keeps a value inside the width of its operand, like the PLC does.
func (s *Sim) wrap(o profile.Operand, v int64) int64 {
	if s.melsec {
		return int64(int16(v))
	}
	switch o.Device {
	case "DINT":
		return int64(int32(v))
	case "UDINT":
		return int64(uint32(v))
	case "WORD", "UINT":
		return int64(uint16(v))
	}
	return int64(int16(v)) // INT and %xW
}

func splitBit(op string) (string, uint) {
	word, b, _ := strings.Cut(op, ".")
	n, _ := strconv.ParseUint(b, 16, 8)
	return word, uint(n)
}

func b2i(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// ---- outside access ----

// Get reads an operand. "T0.ET" gives a timer's elapsed ms and "C0.CV" a
// counter's current value; a bare timer or counter gives its output.
func (s *Sim) Get(op string) (int64, error) {
	if base, ok := strings.CutSuffix(op, ".ET"); ok {
		if o, err := s.resolve(base); err == nil && o.Class == profile.TimerInst {
			return s.timer(base).et, nil
		}
	}
	if base, ok := strings.CutSuffix(op, ".CV"); ok {
		if o, err := s.resolve(base); err == nil && o.Class == profile.CounterInst {
			return s.counter(base).cv, nil
		}
	}
	if _, err := s.resolve(op); err != nil {
		return 0, err
	}
	return s.read(op)
}

// Set writes an operand from outside, e.g. an input. Inputs may be set even
// though the program cannot write them.
func (s *Sim) Set(op string, v int64) error {
	o, err := s.resolve(op)
	if err != nil {
		return err
	}
	if o.Class != profile.Bit && o.Class != profile.Word {
		return fmt.Errorf("%s（%s）は外から設定できません", op, o.Class)
	}
	f, wasForced := s.forced[op]
	delete(s.forced, op)
	err = s.write(op, v)
	if wasForced {
		s.forced[op] = f
	}
	return err
}

// Force pins op to v: reads return v and program writes are ignored.
func (s *Sim) Force(op string, v int64) error {
	o, err := s.resolve(op)
	if err != nil {
		return err
	}
	if o.Class != profile.Bit && o.Class != profile.Word {
		return fmt.Errorf("%s（%s）は強制できません", op, o.Class)
	}
	s.forced[op] = v
	return nil
}

// Unforce releases a forced operand.
func (s *Sim) Unforce(op string) { delete(s.forced, op) }

// IsBit tells whether op reads as on/off (bits, timer and counter outputs).
func (s *Sim) IsBit(op string) bool {
	o, err := s.resolve(op)
	if err != nil {
		return false
	}
	return o.Class == profile.Bit || o.Class == profile.TimerInst || o.Class == profile.CounterInst
}

// Format writes a value the way a person reads it: on/off for bits.
func (s *Sim) Format(op string, v int64) string {
	if s.IsBit(op) {
		if v != 0 {
			return "on"
		}
		return "off"
	}
	if strings.HasSuffix(op, ".ET") {
		return FormatDuration(v)
	}
	return strconv.FormatInt(v, 10)
}

// Operands lists the non-constant operands of p in order of appearance.
func Operands(p *ladder.Program) []string {
	var out []string
	seen := map[string]bool{}
	prof, err := profile.Get(p.Profile)
	if err != nil {
		return nil
	}
	add := func(ops ...string) {
		for _, op := range ops {
			o, err := prof.Resolve(op, p)
			if err != nil || o.Class == profile.Const || o.Class == profile.TimeConst || seen[op] {
				continue
			}
			seen[op] = true
			out = append(out, op)
		}
	}
	for _, r := range p.Rungs {
		for _, e := range ladder.Elements(r.Body) {
			switch v := e.(type) {
			case *ladder.Contact:
				add(v.Operand)
			case *ladder.Compare:
				add(v.LHS, v.RHS)
			case *ladder.Coil:
				add(v.Operand)
			case *ladder.Timer:
				add(v.Instance)
			case *ladder.Counter:
				add(v.Instance)
			case *ladder.Instr:
				add(v.Args...)
			}
		}
	}
	return out
}

// FormatDuration writes milliseconds the way people read them: 5s, 1.50s, 200ms.
func FormatDuration(ms int64) string {
	if ms%1000 == 0 {
		return fmt.Sprintf("%ds", ms/1000)
	}
	if ms > 1000 {
		return fmt.Sprintf("%.2fs", float64(ms)/1000)
	}
	return fmt.Sprintf("%dms", ms)
}
