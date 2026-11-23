package profile

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"rungkit/ir"
)

// iec is a plain IEC 61131-3 dialect: named variables declared with sym,
// direct addresses like %IX0.0, integer literals and T#... time literals.
type iec struct{ instrs map[string]InstrSpec }

func init() { register(&iec{instrs: instrTable()}) }

func (*iec) Name() string                          { return "iec" }
func (*iec) TimerKinds() []string                  { return []string{"TON", "TOF", "TP"} }
func (i *iec) Instr(name string) (InstrSpec, bool) { s, ok := i.instrs[name]; return s, ok }
func (*iec) TimerPresetOK(o Operand) bool          { return o.Class == TimeConst }

var (
	reDirect = regexp.MustCompile(`^%([IQM])([XW])\d+(\.\d+)?$`)
	reTime   = regexp.MustCompile(`^(?i)T#(\d+(ms|s|m|h|d))+$`)
	reIdent  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

func (*iec) Resolve(op string, p *ir.Program) (Operand, error) {
	if m := reDirect.FindStringSubmatch(op); m != nil {
		class := Bit
		if m[2] == "W" {
			class = Word
		}
		return Operand{Class: class, Writable: m[1] != "I", Device: "%" + m[1] + m[2]}, nil
	}
	if reTime.MatchString(op) {
		return Operand{Class: TimeConst}, nil
	}
	if v, err := parseIECInt(op); err == nil {
		return Operand{Class: Const, Value: v}, nil
	}
	if !reIdent.MatchString(op) {
		return Operand{}, fmt.Errorf("%s: 変数名・%%アドレス・数値・T#時間 のどれでもありません", op)
	}
	s := p.Symbol(op)
	if s == nil {
		return Operand{}, fmt.Errorf("%s: 宣言されていません（sym %s : BOOL のように宣言します）", op, op)
	}
	writable := !strings.HasPrefix(s.Address, "%I")
	switch s.Type {
	case "BOOL":
		return Operand{Class: Bit, Writable: writable, Device: s.Type}, nil
	case "INT", "DINT", "WORD", "UINT", "UDINT":
		return Operand{Class: Word, Writable: writable, Device: s.Type}, nil
	case "TON", "TOF", "TP":
		return Operand{Class: TimerInst, Writable: true, Device: s.Type}, nil
	case "CTU":
		return Operand{Class: CounterInst, Writable: true, Device: s.Type}, nil
	case "":
		return Operand{}, fmt.Errorf("%s: 型がありません（sym %s : BOOL のように書きます）", op, op)
	}
	return Operand{}, fmt.Errorf("%s: 型 %s にはまだ対応していません", op, s.Type)
}

func parseIECInt(s string) (int64, error) {
	if base, digits, ok := strings.Cut(s, "#"); ok {
		b, err := strconv.Atoi(base)
		if err != nil || (b != 2 && b != 8 && b != 16) {
			return 0, fmt.Errorf("bad base")
		}
		return strconv.ParseInt(strings.ReplaceAll(digits, "_", ""), b, 64)
	}
	return strconv.ParseInt(strings.ReplaceAll(s, "_", ""), 10, 64)
}
