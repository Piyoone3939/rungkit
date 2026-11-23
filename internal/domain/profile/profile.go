// Package profile holds vendor-specific rules: how operands are written,
// which instructions exist and how presets are expressed.
package profile

import (
	"sort"

	"github.com/Piyoone3939/rungkit/internal/domain/i18n"
	"github.com/Piyoone3939/rungkit/internal/domain/ladder"
)

// Class is what an operand refers to.
type Class int

const (
	Bit Class = iota
	Word
	Const
	TimerInst
	CounterInst
	TimeConst
)

func (c Class) String() string {
	return i18n.T([...]string{"ビット", "ワード", "定数", "タイマー", "カウンタ", "時間定数"}[c])
}

// Operand is a resolved operand.
type Operand struct {
	Class    Class
	Writable bool
	// Device is the device letter(s) for MELSEC ("X", "D", ...), or the
	// declared type for iec symbols.
	Device string
	// BitOfWord is true for D0.3 style operands.
	BitOfWord bool
	// Index is the device number (MELSEC), and Hex tells how it is written.
	Index int64
	Hex   bool
	// Value is the numeric value of a Const, or milliseconds of a TimeConst.
	Value int64
}

// ArgKind is what an instruction argument accepts.
type ArgKind int

const (
	// Src accepts a word or a constant.
	Src ArgKind = iota
	// Dst accepts a writable word.
	Dst
)

// InstrSpec describes an application instruction.
type InstrSpec struct {
	Name string
	Args []ArgKind
	// Pulse is true for the P variants that run once on the rising edge.
	Pulse bool
	// Mnemonic is the vendor spelling (ADD -> "+" on MELSEC).
	Mnemonic string
	// Op is the arithmetic meaning, shared by exporters: MOV ADD SUB MUL DIV INC DEC.
	Op string
}

// Profile is a target dialect.
type Profile interface {
	Name() string
	Resolve(op string, p *ladder.Program) (Operand, error)
	Instr(name string) (InstrSpec, bool)
	// TimerKinds lists the timer kinds the target supports.
	TimerKinds() []string
	// PresetOK tells whether o can be a timer preset.
	TimerPresetOK(o Operand) bool
}

var registry = map[string]Profile{}

func register(p Profile) { registry[p.Name()] = p }

// Get returns the profile with the given name.
func Get(name string) (Profile, error) {
	if p, ok := registry[name]; ok {
		return p, nil
	}
	return nil, i18n.Errorf("プロファイル %q はありません（使えるもの: %v）", name, Names())
}

// Names lists the registered profiles.
func Names() []string {
	var ns []string
	for n := range registry {
		ns = append(ns, n)
	}
	sort.Strings(ns)
	return ns
}

// arithmetic instructions shared by the profiles, keyed by neutral name.
var baseInstrs = []struct {
	name, mnemonic string
	args           []ArgKind
}{
	{"MOV", "MOV", []ArgKind{Src, Dst}},
	{"ADD", "+", []ArgKind{Src, Src, Dst}},
	{"SUB", "-", []ArgKind{Src, Src, Dst}},
	{"MUL", "*", []ArgKind{Src, Src, Dst}},
	{"DIV", "/", []ArgKind{Src, Src, Dst}},
	{"INC", "INC", []ArgKind{Dst}},
	{"DEC", "DEC", []ArgKind{Dst}},
}

func instrTable() map[string]InstrSpec {
	t := map[string]InstrSpec{}
	for _, b := range baseInstrs {
		t[b.name] = InstrSpec{Name: b.name, Args: b.args, Mnemonic: b.mnemonic, Op: b.name}
		t[b.name+"P"] = InstrSpec{Name: b.name + "P", Args: b.args, Pulse: true, Mnemonic: b.mnemonic + "P", Op: b.name}
	}
	return t
}
