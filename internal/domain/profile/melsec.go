package profile

import (
	"strconv"
	"strings"

	"github.com/Piyoone3939/rungkit/internal/domain/i18n"
	"github.com/Piyoone3939/rungkit/internal/domain/ladder"
)

// melsec is a MELSEC dialect. iQ-R writes X/Y/B/W in hexadecimal; FX5
// writes X/Y in octal (X0-X7, X10, ...).
type melsec struct {
	name    string
	instrs  map[string]InstrSpec
	octalXY bool
}

func init() {
	register(&melsec{name: "melsec-iqr", instrs: instrTable()})
	register(&melsec{name: "melsec-fx5", instrs: instrTable(), octalXY: true})
}

func (m *melsec) Name() string                        { return m.name }
func (m *melsec) TimerKinds() []string                { return []string{"TON"} }
func (m *melsec) Instr(name string) (InstrSpec, bool) { s, ok := m.instrs[name]; return s, ok }
func (m *melsec) TimerPresetOK(o Operand) bool        { return o.Class == Const || o.Class == Word }

type devInfo struct {
	class    Class
	writable bool
	hex      bool
}

// Longest prefixes first so "SM" wins over "M".
var melsecDevs = []struct {
	prefix string
	info   devInfo
}{
	{"SM", devInfo{Bit, false, false}},
	{"SD", devInfo{Word, false, false}},
	{"X", devInfo{Bit, false, true}},
	{"Y", devInfo{Bit, true, true}},
	{"M", devInfo{Bit, true, false}},
	{"L", devInfo{Bit, true, false}},
	{"B", devInfo{Bit, true, true}},
	{"F", devInfo{Bit, true, false}},
	{"T", devInfo{TimerInst, true, false}},
	{"C", devInfo{CounterInst, true, false}},
	{"D", devInfo{Word, true, false}},
	{"W", devInfo{Word, true, true}},
	{"R", devInfo{Word, true, false}},
}

func (m *melsec) Resolve(op string, _ *ladder.Program) (Operand, error) {
	if op == "" {
		return Operand{}, i18n.Errorf("オペランドが空です")
	}
	if strings.ToUpper(op) != op {
		return Operand{}, i18n.Errorf("%s: 三菱のデバイスは大文字で書きます（%s）", op, strings.ToUpper(op))
	}
	switch op[0] {
	case 'K':
		v, err := strconv.ParseInt(op[1:], 10, 32)
		if err != nil {
			return Operand{}, i18n.Errorf("%s: K のあとは10進数です", op)
		}
		return Operand{Class: Const, Value: v}, nil
	case 'H':
		v, err := strconv.ParseInt(op[1:], 16, 64)
		if err != nil || len(op) < 2 {
			return Operand{}, i18n.Errorf("%s: H のあとは16進数です", op)
		}
		return Operand{Class: Const, Value: v}, nil
	}
	for _, d := range melsecDevs {
		if !strings.HasPrefix(op, d.prefix) {
			continue
		}
		rest := op[len(d.prefix):]
		num, bit, hasBit := strings.Cut(rest, ".")
		base := 10
		if d.info.hex {
			base = 16
		}
		if m.octalXY && (d.prefix == "X" || d.prefix == "Y") {
			base = 8
		}
		if num == "" {
			return Operand{}, i18n.Errorf("%s: デバイス番号がありません", op)
		}
		idx, err := strconv.ParseUint(num, base, 32)
		if err != nil {
			switch base {
			case 16:
				return Operand{}, i18n.Errorf("%s: %s の番号は16進数です", op, d.prefix)
			case 8:
				return Operand{}, i18n.Errorf("%s: %s の番号は8進数です（0〜7）", op, d.prefix)
			}
			return Operand{}, i18n.Errorf("%s: %s の番号は10進数です", op, d.prefix)
		}
		if hasBit {
			if d.info.class != Word {
				return Operand{}, i18n.Errorf("%s: ビット指定（.n）はワードデバイスにだけ使えます", op)
			}
			if v, err := strconv.ParseUint(bit, 16, 8); err != nil || v > 15 || len(bit) != 1 {
				return Operand{}, i18n.Errorf("%s: ビット番号は 0〜F の1桁です", op)
			}
			return Operand{Class: Bit, Writable: d.info.writable, Device: d.prefix, BitOfWord: true, Index: int64(idx), Hex: d.info.hex}, nil
		}
		return Operand{Class: d.info.class, Writable: d.info.writable, Device: d.prefix, Index: int64(idx), Hex: d.info.hex}, nil
	}
	return Operand{}, i18n.Errorf("%s: 三菱のデバイスとして読めません", op)
}
